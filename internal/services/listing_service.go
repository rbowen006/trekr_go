// Package services holds the domain/service layer. Domain rules live here
// rather than in GORM callbacks (mirroring the Rails app's service objects and
// the plan's ListingService/BookingService/ChatService split), so handlers stay
// thin and the rules are unit-testable.
package services

import (
	"errors"
	"log"
	"os"

	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/region"
	"github.com/rbowen/trekr_go/internal/storage"
	"gorm.io/gorm"
)

// UploadedImage is one multipart image handed to the service for attachment.
type UploadedImage struct {
	Filename    string
	ContentType string
	Data        []byte
}

// ListingParams carries the permitted listing attributes. Every field is a
// pointer so the service can tell "absent" from "set to zero" — required for
// partial updates and for presence validation on create.
type ListingParams struct {
	Title       *string
	Description *string
	RvType      *string
	Town        *string
	State       *string
	Postcode    *string
	PricePerDay *string
	MaxGuests   *int
	PetFriendly *bool
	Latitude    *float64
	Longitude   *float64
}

// ListingEmbedQueue enqueues a listing-embed job. It is the seam for
// RvListing's `after_commit :refresh_embedding` (ADR-0011); the asynq client
// (internal/jobs) satisfies it, and tests substitute a fake. A nil queue skips
// enqueuing (e.g. tests that don't exercise embeddings).
type ListingEmbedQueue interface {
	EnqueueListingEmbed(listingID int64) error
}

// ListingService owns listing create/update/destroy and image attachment,
// including canonical region resolution (ADR-0013) and Active Storage writes.
type ListingService struct {
	DB          *gorm.DB
	StorageRoot string
	Regions     region.Manifest
	Queue       ListingEmbedQueue
}

// refreshEmbedding enqueues a re-embed for the listing, mirroring RvListing's
// after_commit callback. Best-effort: a queue error is logged, not surfaced, so
// the write still succeeds (Rails swallows after_commit callback errors).
func (s *ListingService) refreshEmbedding(listingID int64) {
	if s.Queue == nil {
		return
	}
	if err := s.Queue.EnqueueListingEmbed(listingID); err != nil {
		log.Printf("failed to enqueue listing embed for %d: %v", listingID, err)
	}
}

// Create builds, validates, and persists a listing owned by ownerID, attaching
// the uploaded images. It mirrors RvListingsController#create + the model's
// presence validations and at_least_one_image (on: :create). It returns the
// persisted listing, a slice of Rails-style validation messages (non-empty =>
// 422), or an internal error.
func (s *ListingService) Create(ownerID int64, p ListingParams, images []UploadedImage) (*models.RvListing, []string, error) {
	// max_guests defaults to 1 in the DB (NOT NULL); mirror that so an omitted
	// value persists 1 rather than Go's zero. rv_type/pet_friendly zero values
	// already match their DB defaults (0/caravan, false).
	listing := &models.RvListing{OwnerID: ownerID, MaxGuests: 1}
	errs := applyCreate(listing, p)
	if len(images) == 0 {
		errs = append(errs, "Images must have at least one photo")
	}
	if len(errs) > 0 {
		return nil, errs, nil
	}
	s.assignRegion(listing)

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(listing).Error; err != nil {
			return err
		}
		return s.attachImages(tx, listing.ID, images)
	})
	if err != nil {
		return nil, nil, err
	}
	s.refreshEmbedding(listing.ID)
	return listing, nil, nil
}

// Update applies a partial change to an existing listing, re-resolving its
// region, and re-runs presence validation (Rails validates on update too). It
// never touches attached images. Returns validation messages (=> 422) or an
// internal error.
func (s *ListingService) Update(listing *models.RvListing, p ListingParams) ([]string, error) {
	if errs := applyUpdate(listing, p); len(errs) > 0 {
		return errs, nil
	}
	s.assignRegion(listing)
	if err := s.DB.Save(listing).Error; err != nil {
		return nil, err
	}
	s.refreshEmbedding(listing.ID)
	return nil, nil
}

// Destroy deletes a listing and its dependents, mirroring the Rails model's
// dependent: :destroy associations (bookings, chats, listing_embedding) and the
// has_many_attached :images purge. Done in one transaction, in foreign-key-safe
// order, so a listing with existing Rails-created bookings/chats can be removed.
func (s *ListingService) Destroy(listing *models.RvListing) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.purgeImages(tx, listing.ID); err != nil {
			return err
		}
		cascades := []string{
			"DELETE FROM messages WHERE chat_id IN (SELECT id FROM chats WHERE rv_listing_id = ?)",
			"DELETE FROM chats WHERE rv_listing_id = ?",
			"DELETE FROM bookings WHERE rv_listing_id = ?",
			"DELETE FROM listing_embeddings WHERE rv_listing_id = ?",
		}
		for _, q := range cascades {
			if err := tx.Exec(q, listing.ID).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&models.RvListing{}, listing.ID).Error
	})
}

// AttachImages attaches additional images to an existing listing (the images
// create endpoint). Mirrors @listing.images.attach(params[:images]).
func (s *ListingService) AttachImages(listingID int64, images []UploadedImage) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		return s.attachImages(tx, listingID, images)
	})
}

// DeleteImage purges one image attachment (blob row + disk file) from a
// listing. Reports found=false when the attachment does not belong to the
// listing, so the handler can return 404. Mirrors attachment.purge_later.
func (s *ListingService) DeleteImage(listingID, attachmentID int64) (found bool, err error) {
	var att models.ActiveStorageAttachment
	q := s.DB.Where("id = ? AND record_type = ? AND record_id = ? AND name = ?",
		attachmentID, "RvListing", listingID, "images").First(&att)
	if errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if q.Error != nil {
		return false, q.Error
	}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		return s.purgeAttachment(tx, att)
	})
	return true, err
}

// assignRegion sets the canonical region slug (or nil outside coverage),
// mirroring the model's before_validation :assign_region (ADR-0013).
func (s *ListingService) assignRegion(l *models.RvListing) {
	if slug := s.Regions.Resolve(l.Town, l.State, l.Postcode); slug != "" {
		l.Region = &slug
	} else {
		l.Region = nil
	}
}

// attachImages writes each image to disk and inserts its blob + attachment rows
// (ActiveStorage::Blob.create_and_upload! + attach). Must run inside a tx.
func (s *ListingService) attachImages(tx *gorm.DB, listingID int64, images []UploadedImage) error {
	for _, img := range images {
		key, err := storage.GenerateKey()
		if err != nil {
			return err
		}
		if err := storage.WriteBlob(s.StorageRoot, key, img.Data); err != nil {
			return err
		}
		checksum := storage.Checksum(img.Data)
		metadata := `{"identified":true}`
		blob := &models.ActiveStorageBlob{
			Key:         key,
			Filename:    img.Filename,
			ContentType: img.ContentType,
			Metadata:    &metadata,
			ServiceName: "local",
			ByteSize:    int64(len(img.Data)),
			Checksum:    &checksum,
		}
		if err := tx.Create(blob).Error; err != nil {
			return err
		}
		att := &models.ActiveStorageAttachment{
			Name:       "images",
			RecordType: "RvListing",
			RecordID:   listingID,
			BlobID:     blob.ID,
		}
		if err := tx.Create(att).Error; err != nil {
			return err
		}
	}
	return nil
}

// purgeImages removes all image attachments (+ blobs + files) for a listing.
func (s *ListingService) purgeImages(tx *gorm.DB, listingID int64) error {
	var atts []models.ActiveStorageAttachment
	if err := tx.Where("record_type = ? AND record_id = ? AND name = ?",
		"RvListing", listingID, "images").Find(&atts).Error; err != nil {
		return err
	}
	for _, att := range atts {
		if err := s.purgeAttachment(tx, att); err != nil {
			return err
		}
	}
	return nil
}

// purgeAttachment deletes one attachment row, its blob row, and the disk file.
func (s *ListingService) purgeAttachment(tx *gorm.DB, att models.ActiveStorageAttachment) error {
	if err := tx.Delete(&models.ActiveStorageAttachment{}, att.ID).Error; err != nil {
		return err
	}
	var blob models.ActiveStorageBlob
	if err := tx.First(&blob, att.BlobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := tx.Delete(&models.ActiveStorageBlob{}, blob.ID).Error; err != nil {
		return err
	}
	// Best-effort disk removal; a missing file must not fail the purge.
	if path, err := storage.DiskPath(s.StorageRoot, blob.Key); err == nil {
		_ = os.Remove(path)
	}
	return nil
}

// applyCreate populates a fresh listing from params, collecting Rails-style
// presence/enum errors in the model's validation declaration order.
func applyCreate(l *models.RvListing, p ListingParams) []string {
	var errs []string
	l.Title = deref(p.Title)
	if blank(p.Title) {
		errs = append(errs, "Title can't be blank")
	}
	l.Description = deref(p.Description)
	if blank(p.Description) {
		errs = append(errs, "Description can't be blank")
	}
	// rv_type is NOT NULL with a DB default (0/caravan), so Rails never reports
	// it blank — an absent value defaults to caravan. Only an explicit, invalid
	// enum name is rejected.
	if p.RvType != nil && *p.RvType != "" {
		if v, ok := models.RvTypeValue(*p.RvType); ok {
			l.RvType = v
		} else {
			errs = append(errs, "Rv type is not included in the list")
		}
	}
	l.Town = deref(p.Town)
	if blank(p.Town) {
		errs = append(errs, "Town can't be blank")
	}
	l.State = deref(p.State)
	if blank(p.State) {
		errs = append(errs, "State can't be blank")
	}
	l.Postcode = deref(p.Postcode)
	if blank(p.Postcode) {
		errs = append(errs, "Postcode can't be blank")
	}
	l.PricePerDay = p.PricePerDay
	if blank(p.PricePerDay) {
		errs = append(errs, "Price per day can't be blank")
	}
	// max_guests is NOT NULL with a DB default (1); an absent value keeps the
	// default set by the caller, so Rails never reports it blank.
	if p.MaxGuests != nil {
		l.MaxGuests = *p.MaxGuests
	}
	if p.PetFriendly != nil {
		l.PetFriendly = *p.PetFriendly
	}
	l.Latitude = p.Latitude
	l.Longitude = p.Longitude
	return errs
}

// applyUpdate applies only the provided fields to an existing listing, then
// re-validates presence (Rails presence validations run on update too).
func applyUpdate(l *models.RvListing, p ListingParams) []string {
	if p.Title != nil {
		l.Title = *p.Title
	}
	if p.Description != nil {
		l.Description = *p.Description
	}
	if p.Town != nil {
		l.Town = *p.Town
	}
	if p.State != nil {
		l.State = *p.State
	}
	if p.Postcode != nil {
		l.Postcode = *p.Postcode
	}
	if p.PricePerDay != nil {
		l.PricePerDay = p.PricePerDay
	}
	if p.MaxGuests != nil {
		l.MaxGuests = *p.MaxGuests
	}
	if p.PetFriendly != nil {
		l.PetFriendly = *p.PetFriendly
	}
	if p.Latitude != nil {
		l.Latitude = p.Latitude
	}
	if p.Longitude != nil {
		l.Longitude = p.Longitude
	}

	var errs []string
	if p.RvType != nil {
		if v, ok := models.RvTypeValue(*p.RvType); ok {
			l.RvType = v
		} else {
			errs = append(errs, "Rv type is not included in the list")
		}
	}
	if l.Title == "" {
		errs = append(errs, "Title can't be blank")
	}
	if l.Description == "" {
		errs = append(errs, "Description can't be blank")
	}
	if l.Town == "" {
		errs = append(errs, "Town can't be blank")
	}
	if l.State == "" {
		errs = append(errs, "State can't be blank")
	}
	if l.Postcode == "" {
		errs = append(errs, "Postcode can't be blank")
	}
	if l.PricePerDay == nil || *l.PricePerDay == "" {
		errs = append(errs, "Price per day can't be blank")
	}
	return errs
}

func blank(s *string) bool { return s == nil || *s == "" }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

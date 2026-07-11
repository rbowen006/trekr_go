package httpapi

import (
	"net/url"
	"strings"

	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/storage"
	"gorm.io/gorm"
)

// listingJSON matches RvListing#as_json field-for-field, including key order.
type listingJSON struct {
	ID          int64       `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	RvType      string      `json:"rv_type"`
	Town        string      `json:"town"`
	State       string      `json:"state"`
	Postcode    string      `json:"postcode"`
	PricePerDay *string     `json:"price_per_day"`
	OwnerID     int64       `json:"owner_id"`
	MaxGuests   int         `json:"max_guests"`
	PetFriendly bool        `json:"pet_friendly"`
	Latitude    *float64    `json:"latitude"`
	Longitude   *float64    `json:"longitude"`
	Owner       ownerJSON   `json:"owner"`
	Images      []imageJSON `json:"images"`
}

type ownerJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type imageJSON struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

// serializeListings builds the JSON for a set of listings, batching the owner
// and image lookups to avoid N+1 queries.
func (app *App) serializeListings(db *gorm.DB, listings []models.RvListing) ([]listingJSON, error) {
	ids := make([]int64, len(listings))
	for i, l := range listings {
		ids[i] = l.ID
	}
	imagesByListing, err := app.loadImages(db, ids)
	if err != nil {
		return nil, err
	}

	out := make([]listingJSON, len(listings))
	for i, l := range listings {
		images := imagesByListing[l.ID]
		if images == nil {
			images = []imageJSON{}
		}
		owner := ownerJSON{ID: l.OwnerID}
		if l.Owner != nil {
			owner.Name = l.Owner.Name
		}
		out[i] = listingJSON{
			ID:          l.ID,
			Title:       l.Title,
			Description: l.Description,
			RvType:      models.RvTypeName(l.RvType),
			Town:        l.Town,
			State:       l.State,
			Postcode:    l.Postcode,
			PricePerDay: formatDecimal(l.PricePerDay),
			OwnerID:     l.OwnerID,
			MaxGuests:   l.MaxGuests,
			PetFriendly: l.PetFriendly,
			Latitude:    l.Latitude,
			Longitude:   l.Longitude,
			Owner:       owner,
			Images:      images,
		}
	}
	return out, nil
}

// imageRow is the joined attachment+blob data for building an image URL.
type imageRow struct {
	RecordID     int64
	AttachmentID int64
	BlobID       int64
	Filename     string
}

func (app *App) loadImages(db *gorm.DB, listingIDs []int64) (map[int64][]imageJSON, error) {
	result := map[int64][]imageJSON{}
	if len(listingIDs) == 0 {
		return result, nil
	}

	var rows []imageRow
	err := db.Table("active_storage_attachments AS a").
		Select("a.record_id AS record_id, a.id AS attachment_id, b.id AS blob_id, b.filename AS filename").
		Joins("JOIN active_storage_blobs b ON b.id = a.blob_id").
		Where("a.record_type = ? AND a.name = ? AND a.record_id IN ?", "RvListing", "images", listingIDs).
		Order("a.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		signedID, err := storage.GenerateSignedID(app.Config.SecretKeyBase, row.BlobID)
		if err != nil {
			return nil, err
		}
		u := "/rails/active_storage/blobs/redirect/" + signedID + "/" + url.PathEscape(row.Filename)
		result[row.RecordID] = append(result[row.RecordID], imageJSON{ID: row.AttachmentID, URL: u})
	}
	return result, nil
}

// formatDecimal renders a Postgres numeric the way Rails serializes a
// BigDecimal: float notation with trailing zeros trimmed but at least one
// decimal place (e.g. "149" -> "149.0", "250.00" -> "250.0", "149.90" -> "149.9").
func formatDecimal(raw *string) *string {
	if raw == nil {
		return nil
	}
	s := *raw
	if !strings.Contains(s, ".") {
		s += ".0"
		return &s
	}
	s = strings.TrimRight(s, "0")
	if strings.HasSuffix(s, ".") {
		s += "0"
	}
	return &s
}

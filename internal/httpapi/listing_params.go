package httpapi

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/rbowen/trekr_go/internal/services"
)

// maxMultipartMemory caps in-memory multipart parsing; larger uploads spill to
// temp files. 32 MiB matches a generous listing-photo batch.
const maxMultipartMemory = 32 << 20

// listingFileKeys are the multipart field names a create/update may use for
// image files. Rails request specs nest them under listing[images][]; the
// frontend FormData may also send images[] — accept both.
var listingFileKeys = []string{"listing[images][]", "listing[images]", "images[]", "images"}

// imageFileKeys are the field names the images endpoint accepts (params[:images]).
var imageFileKeys = []string{"images[]", "images"}

// parseListingRequest reads listing params (and any uploaded images) from a
// create/update request, accepting either JSON ({"listing": {...}}) or
// multipart/form-data (listing[field], listing[images][]).
func parseListingRequest(r *http.Request) (services.ListingParams, []services.UploadedImage, error) {
	if isMultipart(r) {
		if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
			return services.ListingParams{}, nil, err
		}
		params := multipartListingParams(r.MultipartForm)
		images, err := collectFiles(r.MultipartForm, listingFileKeys)
		return params, images, err
	}
	params, err := jsonListingParams(r)
	return params, nil, err
}

// parseImageUpload reads just the uploaded images from the images endpoint.
func parseImageUpload(r *http.Request) ([]services.UploadedImage, error) {
	if !isMultipart(r) {
		return nil, nil
	}
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return nil, err
	}
	return collectFiles(r.MultipartForm, imageFileKeys)
}

func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
}

// jsonBodyListing mirrors the permitted attributes in a JSON body. Pointers
// distinguish absent from zero; price_per_day accepts a number or string.
type jsonBodyListing struct {
	Listing struct {
		Title       *string      `json:"title"`
		Description *string      `json:"description"`
		RvType      *string      `json:"rv_type"`
		Town        *string      `json:"town"`
		State       *string      `json:"state"`
		Postcode    *string      `json:"postcode"`
		PricePerDay *json.Number `json:"price_per_day"`
		MaxGuests   *int         `json:"max_guests"`
		PetFriendly *bool        `json:"pet_friendly"`
		Latitude    *float64     `json:"latitude"`
		Longitude   *float64     `json:"longitude"`
	} `json:"listing"`
}

func jsonListingParams(r *http.Request) (services.ListingParams, error) {
	var body jsonBodyListing
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return services.ListingParams{}, err
	}
	l := body.Listing
	p := services.ListingParams{
		Title:       l.Title,
		Description: l.Description,
		RvType:      l.RvType,
		Town:        l.Town,
		State:       l.State,
		Postcode:    l.Postcode,
		MaxGuests:   l.MaxGuests,
		PetFriendly: l.PetFriendly,
		Latitude:    l.Latitude,
		Longitude:   l.Longitude,
	}
	if l.PricePerDay != nil {
		s := l.PricePerDay.String()
		p.PricePerDay = &s
	}
	return p, nil
}

func multipartListingParams(form *multipart.Form) services.ListingParams {
	var p services.ListingParams
	if v, ok := formVal(form, "listing[title]"); ok {
		p.Title = &v
	}
	if v, ok := formVal(form, "listing[description]"); ok {
		p.Description = &v
	}
	if v, ok := formVal(form, "listing[rv_type]"); ok {
		p.RvType = &v
	}
	if v, ok := formVal(form, "listing[town]"); ok {
		p.Town = &v
	}
	if v, ok := formVal(form, "listing[state]"); ok {
		p.State = &v
	}
	if v, ok := formVal(form, "listing[postcode]"); ok {
		p.Postcode = &v
	}
	if v, ok := formVal(form, "listing[price_per_day]"); ok {
		p.PricePerDay = &v
	}
	if v, ok := formVal(form, "listing[max_guests]"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			p.MaxGuests = &n
		}
	}
	if v, ok := formVal(form, "listing[pet_friendly]"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			p.PetFriendly = &b
		}
	}
	if v, ok := formVal(form, "listing[latitude]"); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			p.Latitude = &f
		}
	}
	if v, ok := formVal(form, "listing[longitude]"); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			p.Longitude = &f
		}
	}
	return p
}

// formVal returns the first value for key and whether the key was present.
func formVal(form *multipart.Form, key string) (string, bool) {
	vals, ok := form.Value[key]
	if !ok || len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}

// collectFiles reads uploaded files from the first candidate key that is
// present, returning them as UploadedImages with bytes read into memory.
func collectFiles(form *multipart.Form, keys []string) ([]services.UploadedImage, error) {
	var headers []*multipart.FileHeader
	for _, key := range keys {
		if fhs, ok := form.File[key]; ok {
			headers = fhs
			break
		}
	}
	images := make([]services.UploadedImage, 0, len(headers))
	for _, fh := range headers {
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		ct := fh.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		images = append(images, services.UploadedImage{
			Filename:    fh.Filename,
			ContentType: ct,
			Data:        data,
		})
	}
	return images, nil
}

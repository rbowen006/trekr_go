package models

import "time"

// RvListing maps to Rails' rv_listings table.
type RvListing struct {
	ID          int64     `gorm:"primaryKey"`
	Title       string    `gorm:"column:title"`
	Description string    `gorm:"column:description"`
	PricePerDay *string   `gorm:"column:price_per_day"` // numeric read as text
	OwnerID     int64     `gorm:"column:owner_id"`
	MaxGuests   int       `gorm:"column:max_guests"`
	PetFriendly bool      `gorm:"column:pet_friendly"`
	Latitude    *float64  `gorm:"column:latitude"`
	Longitude   *float64  `gorm:"column:longitude"`
	RvType      int       `gorm:"column:rv_type"`
	Town        string    `gorm:"column:town"`
	State       string    `gorm:"column:state"`
	Postcode    string    `gorm:"column:postcode"`
	Region      *string   `gorm:"column:region"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`

	Owner *User `gorm:"foreignKey:OwnerID"`
}

func (RvListing) TableName() string { return "rv_listings" }

// RvTypeName maps the rv_type enum integer to its Rails string value.
func RvTypeName(rvType int) string {
	switch rvType {
	case 0:
		return "caravan"
	case 1:
		return "motorhome"
	case 2:
		return "camper_trailer"
	default:
		return ""
	}
}

// RvTypeValue maps a Rails rv_type enum name to its integer, reporting whether
// the name is a valid enum member (the inverse of RvTypeName).
func RvTypeValue(name string) (int, bool) {
	switch name {
	case "caravan":
		return 0, true
	case "motorhome":
		return 1, true
	case "camper_trailer":
		return 2, true
	default:
		return 0, false
	}
}

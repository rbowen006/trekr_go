package models

import "time"

// Booking maps to Rails' bookings table. start_date/end_date are date columns
// (no time component); the serializer renders them as "YYYY-MM-DD".
type Booking struct {
	ID          int64     `gorm:"primaryKey"`
	StartDate   time.Time `gorm:"column:start_date"`
	EndDate     time.Time `gorm:"column:end_date"`
	Status      string    `gorm:"column:status"`
	HirerID     int64     `gorm:"column:hirer_id"`
	RvListingID int64     `gorm:"column:rv_listing_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`

	Hirer     *User      `gorm:"foreignKey:HirerID"`
	RvListing *RvListing `gorm:"foreignKey:RvListingID"`
}

func (Booking) TableName() string { return "bookings" }

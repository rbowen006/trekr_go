package models

import "time"

// Chat maps to Rails' chats table: a conversation between a hirer and an owner,
// optionally about a listing or a booking. The nullable columns are pointers so
// the serializer can render them as JSON null (Rails as_json includes them).
type Chat struct {
	ID                 int64      `gorm:"primaryKey"`
	HirerID            int64      `gorm:"column:hirer_id"`
	OwnerID            int64      `gorm:"column:owner_id"`
	RvListingID        *int64     `gorm:"column:rv_listing_id"`
	BookingID          *int64     `gorm:"column:booking_id"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	LastMessageAt      *time.Time `gorm:"column:last_message_at"`
	LastMessageContent *string    `gorm:"column:last_message_content"`
	HirerLastReadAt    *time.Time `gorm:"column:hirer_last_read_at"`
	OwnerLastReadAt    *time.Time `gorm:"column:owner_last_read_at"`

	Hirer     *User      `gorm:"foreignKey:HirerID"`
	Owner     *User      `gorm:"foreignKey:OwnerID"`
	RvListing *RvListing `gorm:"foreignKey:RvListingID"`
	Messages  []Message  `gorm:"foreignKey:ChatID"`
}

func (Chat) TableName() string { return "chats" }

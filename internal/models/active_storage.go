package models

import "time"

// ActiveStorageBlob maps to Rails' active_storage_blobs table.
type ActiveStorageBlob struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"column:key" json:"key"`
	Filename    string    `gorm:"column:filename" json:"filename"`
	ContentType string    `gorm:"column:content_type" json:"content_type"`
	Metadata    *string   `gorm:"column:metadata" json:"-"`
	ServiceName string    `gorm:"column:service_name" json:"-"`
	ByteSize    int64     `gorm:"column:byte_size" json:"-"`
	Checksum    *string   `gorm:"column:checksum" json:"-"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"-"`
}

func (ActiveStorageBlob) TableName() string { return "active_storage_blobs" }

// ActiveStorageAttachment maps to Rails' active_storage_attachments table
// (polymorphic join between a record like RvListing and a blob).
type ActiveStorageAttachment struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"column:name" json:"-"`
	RecordType string    `gorm:"column:record_type" json:"-"`
	RecordID   int64     `gorm:"column:record_id" json:"-"`
	BlobID     int64     `gorm:"column:blob_id" json:"-"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"-"`
}

func (ActiveStorageAttachment) TableName() string { return "active_storage_attachments" }

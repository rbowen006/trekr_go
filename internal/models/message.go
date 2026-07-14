package models

import "time"

// Message maps to Rails' messages table: one message posted by a user within a
// chat. read_at is nullable (unread until marked), so it is a pointer.
type Message struct {
	ID        int64      `gorm:"primaryKey"`
	Content   string     `gorm:"column:content"`
	UserID    int64      `gorm:"column:user_id"`
	ChatID    int64      `gorm:"column:chat_id"`
	ReadAt    *time.Time `gorm:"column:read_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}

func (Message) TableName() string { return "messages" }

package models

import "time"

// User maps to the Rails users table (schema owned by rv_marketplace).
type User struct {
	ID                   int64      `gorm:"primaryKey" json:"id"`
	Email                string     `gorm:"column:email;not null" json:"email"`
	EncryptedPassword    string     `gorm:"column:encrypted_password;not null" json:"-"`
	ResetPasswordToken   *string    `gorm:"column:reset_password_token" json:"-"`
	ResetPasswordSentAt  *time.Time `gorm:"column:reset_password_sent_at" json:"-"`
	RememberCreatedAt    *time.Time `gorm:"column:remember_created_at" json:"-"`
	Name                 string     `gorm:"column:name;not null" json:"name"`
	CreatedAt            time.Time  `gorm:"column:created_at" json:"-"`
	UpdatedAt            time.Time  `gorm:"column:updated_at" json:"-"`
}

func (User) TableName() string {
	return "users"
}

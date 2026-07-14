package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open connects to Postgres using the given DSN.
func Open(databaseURL string) (*gorm.DB, error) {
	conn, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		// Rails stores UTC in the `timestamp without time zone` columns. Force
		// autoCreateTime/autoUpdateTime to write UTC so Go-created rows (e.g.
		// messages.created_at, chats.last_message_at) match Rails' values and
		// serialize identically, instead of the host's local wall-clock.
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return conn, nil
}

package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open connects to Postgres using the given DSN.
func Open(databaseURL string) (*gorm.DB, error) {
	conn, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return conn, nil
}

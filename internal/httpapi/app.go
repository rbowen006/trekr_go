package httpapi

import (
	"github.com/rbowen/trekr_go/internal/config"
	"gorm.io/gorm"
)

// App holds shared dependencies for HTTP handlers.
type App struct {
	Config config.Config
	DB     *gorm.DB
}

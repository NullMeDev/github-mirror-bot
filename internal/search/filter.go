package search

import (
	"time"

	"github.com/NullMeDev/github-mirror-bot/internal/config"
)

func ShouldKeep(cfg *config.Config, pushedAt time.Time, stars int) bool {
	age := time.Since(pushedAt)
	maxAge := cfg.MaxInactive()
	if age <= maxAge {
		return true
	}
	return stars >= cfg.Filter.MinStarsForStale
}

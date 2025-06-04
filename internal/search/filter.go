package search

import (
	"time"

	"github.com/NullMeDev/github-mirror-bot/internal/config"
)

// ShouldKeep returns true if the repo was pushed within the max inactive window,
// or if it has at least the minimum star count for stale repos.
func ShouldKeep(cfg *config.Config, pushedAt time.Time, stars int) bool {
	age := time.Since(pushedAt)
	maxAge := cfg.MaxInactive()
	if age <= maxAge {
		return true
	}
	return stars >= cfg.Filter.MinStarsForStale
}

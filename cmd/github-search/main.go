package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/NullMeDev/github-mirror-bot/internal/config"
	"github.com/NullMeDev/github-mirror-bot/internal/search"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
	}

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get GitHub token from environment
	token := cfg.GetGitHubToken()
	if token == "" {
		log.Fatalf("GitHub token not found in environment variable: %s", cfg.GitHub.TokenEnv)
	}

	// Validate Discord webhook URL if notifications are enabled
	if cfg.Discord.EnableNotifications {
		webhookURL := cfg.GetDiscordWebhookURL()
		if webhookURL == "" {
			log.Printf("Warning: Discord notifications enabled but webhook URL not found in environment variable: %s", cfg.Discord.WebhookURLEnv)
		}
	}

	// Setup logging
	if cfg.Logging.File != "" {
		logFile, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
		} else {
			log.SetOutput(logFile)
			defer logFile.Close()
		}
	}

	// Initialize queue
	redisPassword := cfg.GetRedisPassword()
	q := search.NewQueue(cfg.Redis.Address, redisPassword, cfg.Redis.DB)
	defer q.Close()

	// Initialize searcher
	searcher := search.NewSearcher(cfg, token, q)
	defer searcher.Close()

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup cron scheduler
	c := cron.New()
	
	_, err = c.AddFunc(cfg.Search.Schedule, func() {
		log.Println("Starting scheduled search...")
		searchCtx, searchCancel := context.WithTimeout(ctx, 30*time.Minute)
		defer searchCancel()
		
		if err := searcher.Run(searchCtx); err != nil {
			log.Printf("Search failed: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to schedule search: %v", err)
	}

	c.Start()
	defer c.Stop()

	log.Printf("GitHub Mirror Bot started with schedule: %s", cfg.Search.Schedule)

	// Run initial search
	log.Println("Running initial search...")
	initialCtx, initialCancel := context.WithTimeout(ctx, 30*time.Minute)
	if err := searcher.Run(initialCtx); err != nil {
		log.Printf("Initial search failed: %v", err)
	}
	initialCancel()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping...")
	cancel()
	
	// Give some time for cleanup
	time.Sleep(2 * time.Second)
	log.Println("Shutdown complete")
}

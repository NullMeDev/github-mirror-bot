package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourusername/github-mirror-bot/internal/config"
    "github.com/yourusername/github-mirror-bot/internal/search"
)

func main() {
    cfg, err := config.LoadConfig("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    bot, err := search.NewBot(cfg)
    if err != nil {
        log.Fatalf("Failed to initialize bot: %v", err)
    }

    // Run bot asynchronously
    go bot.Run()

    // Listen for termination signals to gracefully shutdown
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    sig := <-sigs
    log.Printf("Received signal %s, shutting down bot...", sig)
    bot.Stop()
}

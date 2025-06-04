package main

import (
	"context"
	"log"
	"os"

	"github.com/NullMeDev/github-mirror-bot/internal/config"
	"github.com/NullMeDev/github-mirror-bot/internal/search"
)

func main() {
	cfg, err := config.Load("/home/gitbackup/github-mirror-bot/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	token := os.Getenv(cfg.GitHub.TokenEnv)
	if token == "" {
		log.Fatal("GITHUB_TOKEN env variable not set")
	}

	q := search.NewQueue("127.0.0.1:6379")
	s := search.NewSearcher(cfg, token, q)
	if err := s.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
	log.Println("Search cycle complete")
}

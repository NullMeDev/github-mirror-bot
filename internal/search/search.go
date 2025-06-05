package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/NullMeDev/github-mirror-bot/internal/config"
	"github.com/NullMeDev/github-mirror-bot/internal/util"
)

type Repo struct {
	FullName string    `json:"full_name"`
	SSHURL   string    `json:"ssh_url"`
	HTMLURL  string    `json:"html_url"`
	Stars    int       `json:"stargazers_count"`
	PushedAt time.Time `json:"pushed_at"`
}

type SearchResponse struct {
	Items []Repo `json:"items"`
	Total int    `json:"total_count"`
}

type Searcher struct {
	cfg    *config.Config
	token  string
	bucket *util.TokenBucket
	queue  *Queue
	client *http.Client
}

func NewSearcher(cfg *config.Config, token string, q *Queue) *Searcher {
	b := util.NewBucket(25, time.Minute) // 25 calls per min < 30 limit
	b.Start()
	
	return &Searcher{
		cfg:    cfg,
		token:  token,
		bucket: b,
		queue:  q,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Searcher) Close() {
	s.bucket.Stop()
}

func (s *Searcher) query(ctx context.Context, qs string, page int) ([]Repo, error) {
	if err := s.bucket.Take(ctx); err != nil {
		return nil, fmt.Errorf("rate limit context cancelled: %w", err)
	}

	endpoint := fmt.Sprintf(
		"https://api.github.com/search/repositories?q=%s&sort=updated&per_page=100&page=%d",
		url.QueryEscape(qs), page,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("User-Agent", "github-mirror-bot/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var data SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return data.Items, nil
}

func (s *Searcher) BuildQueries() []string {
	var queries []string
	for _, kw := range s.cfg.Search.Keywords {
		for _, lang := range s.cfg.Search.Languages {
			queries = append(queries, fmt.Sprintf("%s language:%s", kw, lang))
		}
	}
	return queries
}

func (s *Searcher) Run(ctx context.Context) error {
	log.Println("Starting search cycle...")
	
	queries := s.BuildQueries()
	log.Printf("Built %d search queries", len(queries))

	for i, qs := range queries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("Processing query %d/%d: %s", i+1, len(queries), qs)
		
		if err := s.processQuery(ctx, qs); err != nil {
			log.Printf("Error processing query '%s': %v", qs, err)
			continue
		}
	}

	log.Println("Search cycle completed")
	return nil
}

func (s *Searcher) processQuery(ctx context.Context, qs string) error {
	page := 1
	maxPages := (s.cfg.Search.MaxReposPerKeyword + 99) / 100 // Round up division

	for page <= maxPages && page <= 10 { // GitHub limits to 1000 results (10 pages)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		repos, err := s.query(ctx, qs, page)
		if err != nil {
			return fmt.Errorf("failed to query page %d: %w", page, err)
		}

		if len(repos) == 0 {
			break
		}

		log.Printf("Processing %d repos from page %d", len(repos), page)

		for _, r := range repos {
			if err := s.processRepo(ctx, r); err != nil {
				log.Printf("Error processing repo %s: %v", r.FullName, err)
				continue
			}
		}

		page++
	}

	return nil
}

func (s *Searcher) processRepo(ctx context.Context, r Repo) error {
	if !ShouldKeep(s.cfg, r.PushedAt, r.Stars) {
		return nil
	}

	seen, err := s.queue.Seen(ctx, r.FullName)
	if err != nil {
		return fmt.Errorf("failed to check if repo seen: %w", err)
	}
	if seen {
		return nil
	}

	target := r.SSHURL
	if s.cfg.Search.ForkInsteadOfClone {
		target = r.HTMLURL
	}

	// Enqueue for mirror/fork
	if err := s.queue.Enqueue(ctx, target); err != nil {
		return fmt.Errorf("failed to enqueue repo: %w", err)
	}

	if err := s.queue.Mark(ctx, r.FullName); err != nil {
		return fmt.Errorf("failed to mark repo as seen: %w", err)
	}

	// Send Discord notification
	if s.cfg.Discord.WebhookURL != "" {
		msg := fmt.Sprintf(
			"New repo queued: **%s**\nStars: %d  Pushed: %s\nURL: %s",
			r.FullName, r.Stars, r.PushedAt.Format("2006-01-02"), target,
		)
		
		if err := util.SendWebhook(ctx, s.cfg.Discord.WebhookURL, msg); err != nil {
			log.Printf("Failed to send Discord notification: %v", err)
		}
	}

	log.Printf("Queued repo: %s (stars: %d)", r.FullName, r.Stars)
	return nil
}

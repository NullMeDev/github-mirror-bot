package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/NullMeDev/github-mirror-bot/internal/config"
	"github.com/NullMeDev/github-mirror-bot/internal/util"
)

type Repo struct {
	FullName string    `json:"full_name"`
	SSHURL   string    `json:"ssh_url"`
	ForkURL  string    `json:"html_url"`
	Stars    int       `json:"stargazers_count"`
	PushedAt time.Time `json:"pushed_at"`
}

type Searcher struct {
	cfg    *config.Config
	token  string
	bucket *util.TokenBucket
	queue  *Queue
}

func NewSearcher(cfg *config.Config, token string, q *Queue) *Searcher {
	b := util.NewBucket(25, time.Minute) // 25 calls per min < 30 limit
	b.Run()
	return &Searcher{cfg: cfg, token: token, bucket: b, queue: q}
}

func (s *Searcher) query(qs string, page int) ([]Repo, error) {
	s.bucket.Take()
	endpoint := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=updated&per_page=100&page=%d",
		url.QueryEscape(qs), page)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Items []Repo `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
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
	for _, qs := range s.BuildQueries() {
		page := 1
		for page <= 10 {
			repos, err := s.query(qs, page)
			if err != nil {
				return err
			}
			if len(repos) == 0 {
				break
			}
			for _, r := range repos {
				if !ShouldKeep(s.cfg, r.PushedAt, r.Stars) {
					continue
				}
				if s.queue.Seen(r.FullName) {
					continue
				}
				target := r.SSHURL
				if s.cfg.Search.ForkInsteadOfClone {
					target = fmt.Sprintf("https://github.com/%s", r.FullName)
				}
				_ = s.queue.Enqueue(ctx, target)
				s.queue.Mark(r.FullName)
			}
			if page*s.cfg.Search.MaxReposPerKeyword >= 100 {
				break
			}
			page++
		}
	}
	return nil
}

package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GitHub struct {
		TokenEnv string `yaml:"token_env"`
	} `yaml:"github"`
	Search struct {
		Keywords             []string `yaml:"keywords"`
		Languages            []string `yaml:"languages"`
		MaxReposPerKeyword   int      `yaml:"max_repos_per_keyword"`
		ForkInsteadOfClone   bool     `yaml:"fork_instead_of_clone"`
		Schedule             string   `yaml:"schedule"`
	} `yaml:"search"`
	Filter struct {
		MaxInactiveMonths int `yaml:"max_inactive_months"`
		MinStarsForStale  int `yaml:"min_stars_for_stale"`
	} `yaml:"filter"`
	Storage struct {
		LocalDir            string `yaml:"local_dir"`
		Remote              string `yaml:"remote"`
		OffloadAfterMinutes int    `yaml:"offload_after_minutes"`
	} `yaml:"storage"`
	Discord struct {
		WebhookURL string `yaml:"webhook_url"`
	} `yaml:"discord"`
	Redis struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	Logging struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"logging"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	
	// Set defaults
	if c.Redis.Address == "" {
		c.Redis.Address = "127.0.0.1:6379"
	}
	if c.Search.Schedule == "" {
		c.Search.Schedule = "0 */1 * * *"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	
	return &c, nil
}

func (c *Config) MaxInactive() time.Duration {
	return time.Duration(c.Filter.MaxInactiveMonths) * 30 * 24 * time.Hour
}

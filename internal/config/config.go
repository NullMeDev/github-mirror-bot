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
	return &c, nil
}

func (c *Config) MaxInactive() time.Duration {
	return time.Duration(c.Filter.MaxInactiveMonths) * 30 * 24 * time.Hour
}

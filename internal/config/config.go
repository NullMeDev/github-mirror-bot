package config

import (
    "errors"
    "os"
    "strconv"
    "strings"

    "gopkg.in/yaml.v2"
    "io/ioutil"
)

type GithubConfig struct {
    Token           string   `yaml:"token"`
    User            string   `yaml:"user"`
    MinStars        int      `yaml:"min_stars"`
    MaxForkAgeDays  int      `yaml:"max_fork_age_days"`
    Languages       []string `yaml:"languages"`
    Topics          []string `yaml:"topics"`
    SearchInterval  int      `yaml:"search_interval"`
}

type DiscordConfig struct {
    WebhookURL string `yaml:"webhook_url"`
}

type BackupConfig struct {
    RclonePath  string `yaml:"rclone_path"`
    Enabled     bool   `yaml:"enabled"`
    SyncInterval int   `yaml:"sync_interval"`
}

type LoggingConfig struct {
    Level string `yaml:"level"`
    File  string `yaml:"file"`
}

type Config struct {
    Github  GithubConfig  `yaml:"github"`
    Discord DiscordConfig `yaml:"discord"`
    Backup  BackupConfig  `yaml:"backup"`
    Logging LoggingConfig `yaml:"logging"`
}

func LoadConfig(path string) (*Config, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    err = yaml.Unmarshal(data, &cfg)
    if err != nil {
        return nil, err
    }

    // Environment overrides for sensitive data
    if token := os.Getenv("GITHUB_TOKEN"); token != "" {
        cfg.Github.Token = token
    }
    if user := os.Getenv("GITHUB_USER"); user != "" {
        cfg.Github.User = user
    }
    if webhook := os.Getenv("DISCORD_WEBHOOK_URL"); webhook != "" {
        cfg.Discord.WebhookURL = webhook
    }

    // Validate required fields
    if cfg.Github.Token == "" {
        return nil, errors.New("github.token is required (set via env GITHUB_TOKEN)")
    }
    if cfg.Github.User == "" {
        return nil, errors.New("github.user is required (set via env GITHUB_USER)")
    }
    if cfg.Discord.WebhookURL == "" {
        return nil, errors.New("discord.webhook_url is required (set via env DISCORD_WEBHOOK_URL)")
    }
    if cfg.Backup.Enabled && cfg.Backup.RclonePath == "" {
        return nil, errors.New("backup.rclone_path is required if backup is enabled")
    }

    // Defaults for intervals if missing or zero
    if cfg.Github.SearchInterval <= 0 {
        cfg.Github.SearchInterval = 3600
    }
    if cfg.Backup.SyncInterval <= 0 {
        cfg.Backup.SyncInterval = 7200
    }
    if cfg.Github.MinStars < 0 {
        cfg.Github.MinStars = 50
    }
    if cfg.Github.MaxForkAgeDays < 0 {
        cfg.Github.MaxForkAgeDays = 180
    }

    // Normalize languages and topics to lowercase
    for i, lang := range cfg.Github.Languages {
        cfg.Github.Languages[i] = strings.ToLower(lang)
    }
    for i, topic := range cfg.Github.Topics {
        cfg.Github.Topics[i] = strings.ToLower(topic)
    }

    return &cfg, nil
}

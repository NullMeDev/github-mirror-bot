package search

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "math/rand"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/yourusername/github-mirror-bot/internal/config"
    "github.com/yourusername/github-mirror-bot/internal/util"
)

type Bot struct {
    cfg         *config.Config
    ctx         context.Context
    cancel      context.CancelFunc
    discord     *util.DiscordWebhook
    httpClient  *http.Client
    forkedRepos map[string]time.Time // map[repoFullName]forkDate
    forkLock    sync.Mutex
    backupLock  sync.Mutex
    repoDir     string
}

type githubRepo struct {
    FullName    string    `json:"full_name"`
    Description string    `json:"description"`
    Stars       int       `json:"stargazers_count"`
    Forks       int       `json:"forks_count"`
    Language    string    `json:"language"`
    PushedAt    time.Time `json:"pushed_at"`
    URL         string    `json:"html_url"`
    Owner       struct {
        Login string `json:"login"`
    } `json:"owner"`
}

type githubSearchResponse struct {
    TotalCount int           `json:"total_count"`
    Items      []githubRepo  `json:"items"`
}

func NewBot(cfg *config.Config) (*Bot, error) {
    ctx, cancel := context.WithCancel(context.Background())

    discord := util.NewDiscordWebhookFromURL(cfg.Discord.WebhookURL)

    repoDir := filepath.Join(cfg.Backup.RclonePath, "repos")
    if err := os.MkdirAll(repoDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create repo directory: %w", err)
    }

    bot := &Bot{
        cfg:         cfg,
        ctx:         ctx,
        cancel:      cancel,
        discord:     discord,
        httpClient:  &http.Client{Timeout: 15 * time.Second},
        forkedRepos: make(map[string]time.Time),
        repoDir:     repoDir,
    }

    // Load existing forked repos from cache file if exists
    bot.loadForkedReposCache()

    return bot, nil
}

func (b *Bot) Run() {
    log.Println("Bot started")
    searchTicker := time.NewTicker(time.Duration(b.cfg.Github.SearchInterval) * time.Second)
    backupTicker := time.NewTicker(time.Duration(b.cfg.Backup.SyncInterval) * time.Second)

    defer func() {
        searchTicker.Stop()
        backupTicker.Stop()
        b.saveForkedReposCache()
        log.Println("Bot stopped")
    }()

    for {
        select {
        case <-b.ctx.Done():
            log.Println("Received stop signal, exiting Run loop")
            return
        case <-searchTicker.C:
            b.runScrapeAndForkCycle()
        case <-backupTicker.C:
            b.runBackupSyncCycle()
        }
    }
}

func (b *Bot) Stop() {
    b.cancel()
}

func (b *Bot) runScrapeAndForkCycle() {
    log.Println("Starting scrape and fork cycle")

    repos, err := b.searchRepos()
    if err != nil {
        log.Printf("Error searching repos: %v", err)
        b.discord.SendMessage(fmt.Sprintf("Error searching repos: %v", err))
        return
    }

    if len(repos) == 0 {
        log.Println("No repos found this cycle")
        return
    }

    var backedUpCount, forkedCount, failedCount int

    for _, repo := range repos {
        select {
        case <-b.ctx.Done():
            return
        default:
        }

        if b.isRepoForked(repo.FullName) {
            log.Printf("Already forked: %s", repo.FullName)
            continue
        }

        // Fork repo
        err := b.forkRepo(repo)
        if err != nil {
            log.Printf("Failed to fork repo %s: %v", repo.FullName, err)
            failedCount++
            continue
        }
        forkedCount++

        // Clone repo locally
        err = b.cloneRepo(repo)
        if err != nil {
            log.Printf("Failed to clone repo %s: %v", repo.FullName, err)
            failedCount++
            continue
        }
        backedUpCount++
    }

    // Send summary message
    summary := fmt.Sprintf("Scrape and fork cycle complete.\nForked: %d\nBacked up: %d\nFailed: %d", forkedCount, backedUpCount, failedCount)
    b.discord.SendMessage(summary)
}

func (b *Bot) runBackupSyncCycle() {
    b.backupLock.Lock()
    defer b.backupLock.Unlock()

    if !b.cfg.Backup.Enabled {
        log.Println("Backup is disabled, skipping sync cycle")
        return
    }

    log.Println("Starting backup sync cycle")

    // Run rclone sync command
    cmd := exec.Command("rclone", "sync", b.repoDir, b.cfg.Backup.RclonePath, "--progress")
    output, err := cmd.CombinedOutput()
    if err != nil {
        log.Printf("rclone sync failed: %v, output: %s", err, string(output))
        b.discord.SendMessage(fmt.Sprintf("Backup sync failed: %v", err))
        return
    }
    log.Println("Backup sync complete")
    b.discord.SendMessage("Backup sync completed successfully.")
}

func (b *Bot) searchRepos() ([]githubRepo, error) {
    var allRepos []githubRepo
    page := 1
    maxPages := 3 // Limit pages to avoid rate limits

    for page <= maxPages {
        select {
        case <-b.ctx.Done():
            return nil, errors.New("context cancelled")
        default:
        }

        query := b.buildSearchQuery()
        url := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=30&page=%d", query, page)

        req, err := http.NewRequestWithContext(b.ctx, "GET", url, nil)
        if err != nil {
            return nil, err
        }
        req.Header.Set("Authorization", "token "+b.cfg.Github.Token)
        req.Header.Set("Accept", "application/vnd.github.v3+json")

        resp, err := b.httpClient.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()

        if resp.StatusCode == 403 {
            resetTime, _ := util.ParseGithubRateLimitReset(resp.Header)
            sleepDur := time.Until(resetTime) + time.Second*5
            log.Printf("Rate limited. Sleeping for %v", sleepDur)
            time.Sleep(sleepDur)
            continue
        }

        if resp.StatusCode != 200 {
            return nil, fmt.Errorf("github API error: %s", resp.Status)
        }

        var result githubSearchResponse
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return nil, err
        }

        if len(result.Items) == 0 {
            break
        }

        filtered := b.filterRepos(result.Items)
        allRepos = append(allRepos, filtered...)

        if len(result.Items) < 30 {
            break
        }

        page++
        // Randomized delay between pages to avoid detection
        time.Sleep(time.Duration(rand.Intn(10)+5) * time.Second)
    }

    return allRepos, nil
}

func (b *Bot) buildSearchQuery() string {
    stars := fmt.Sprintf("stars:>=%d", b.cfg.Github.MinStars)
    pushed := fmt.Sprintf("pushed:>=%s", time.Now().AddDate(0, 0, -b.cfg.Github.MaxForkAgeDays).Format("2006-01-02"))

    langParts := []string{}
    for _, lang := range b.cfg.Github.Languages {
        langParts = append(langParts, fmt.Sprintf("language:%s", lang))
    }
    langQuery := strings.Join(langParts, " ")

    topicParts := []string{}
    for _, topic := range b.cfg.Github.Topics {
        topicParts = append(topicParts, fmt.Sprintf("topic:%s", topic))
    }
    topicQuery := strings.Join(topicParts, " ")

    query := fmt.Sprintf("%s %s %s %s", stars, pushed, langQuery, topicQuery)
    query = strings.TrimSpace(query)
    query = strings.ReplaceAll(query, " ", "+")
    return query
}

func (b *Bot) filterRepos(repos []githubRepo) []githubRepo {
    filtered := []githubRepo{}
    cutoff := time.Now().AddDate(0, 0, -b.cfg.Github.MaxForkAgeDays)
    for _, r := range repos {
        if r.PushedAt.Before(cutoff) {
            continue
        }
        filtered = append(filtered, r)
    }
    return filtered
}

func (b *Bot) isRepoForked(fullName string) bool {
    b.forkLock.Lock()
    defer b.forkLock.Unlock()
    _, exists := b.forkedRepos[fullName]
    return exists
}

func (b *Bot) forkRepo(repo githubRepo) error {
    b.forkLock.Lock()
    defer b.forkLock.Unlock()

    // Check again inside lock
    if _, exists := b.forkedRepos[repo.FullName]; exists {
        return nil
    }

    forkURL := fmt.Sprintf("https://api.github.com/repos/%s/forks", repo.FullName)
    reqBody := strings.NewReader("{}")
    req, err := http.NewRequest("POST", forkURL, reqBody)
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "token "+b.cfg.Github.Token)
    req.Header.Set("Accept", "application/vnd.github.v3+json")

    resp, err := b.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 202 {
        return fmt.Errorf("failed to fork repo, status: %s", resp.Status)
    }

    // Wait for fork to be available
    forkFullName := fmt.Sprintf("%s/%s", b.cfg.Github.User, strings.Split(repo.FullName, "/")[1])
    for i := 0; i < 10; i++ {
        exists, err := b.checkRepoExists(forkFullName)
        if err != nil {
            return err
        }
        if exists {
            b.forkedRepos[repo.FullName] = time.Now()
            b.saveForkedReposCache()
            return nil
        }
        time.Sleep(5 * time.Second)
    }

    return errors.New("timeout waiting for fork availability")
}

func (b *Bot) checkRepoExists(fullName string) (bool, error) {
    url := fmt.Sprintf("https://api.github.com/repos/%s", fullName)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return false, err
    }
    req.Header.Set("Authorization", "token "+b.cfg.Github.Token)
    req.Header.Set("Accept", "application/vnd.github.v3+json")

    resp, err := b.httpClient.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == 200 {
        return true, nil
    }
    if resp.StatusCode == 404 {
        return false, nil
    }
    return false, fmt.Errorf("unexpected status code checking repo: %d", resp.StatusCode)
}

func (b *Bot) cloneRepo(repo githubRepo) error {
    targetPath := filepath.Join(b.repoDir, strings.ReplaceAll(repo.FullName, "/", "_"))

    // If directory exists, assume already cloned
    if _, err := os.Stat(targetPath); err == nil {
        log.Printf("Repo already cloned at %s", targetPath)
        return nil
    }

    cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", b.cfg.Github.User, strings.Split(repo.FullName, "/")[1])
    cmd := exec.Command("git", "clone", "--depth=1", cloneURL, targetPath)

    out, err := cmd.CombinedOutput()
    if err != nil {
        log.Printf("git clone error: %s", string(out))
        return err
    }

    log.Printf("Successfully cloned %s", repo.FullName)
    return nil
}

// Persist forkedRepos cache to disk for restart resilience
func (b *Bot) saveForkedReposCache() {
    b.forkLock.Lock()
    defer b.forkLock.Unlock()

    cacheFile := filepath.Join(b.repoDir, "forked_repos.json")
    data, err := json.MarshalIndent(b.forkedRepos, "", "  ")
    if err != nil {
        log.Printf("Error marshaling fork cache: %v", err)
        return
    }
    err = os.WriteFile(cacheFile, data, 0644)
    if err != nil {
        log.Printf("Error writing fork cache: %v", err)
    }
}

func (b *Bot) loadForkedReposCache() {
    cacheFile := filepath.Join(b.repoDir, "forked_repos.json")
    data, err := os.ReadFile(cacheFile)
    if err != nil {
        log.Printf("No fork cache found, starting fresh")
        return
    }

    err = json.Unmarshal(data, &b.forkedRepos)
    if err != nil {
        log.Printf("Failed to load fork cache: %v", err)
    }
}

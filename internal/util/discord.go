package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type DiscordPayload struct {
	Content string        `json:"content,omitempty"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

type RepoInfo struct {
	Name        string
	Description string
	Stars       int
	Language    string
	URL         string
	BackedUp    bool
	Error       string
}

type BackupSummary struct {
	TotalFound    int
	TotalBackedUp int
	TotalFailed   int
	Repos         []RepoInfo
	Duration      time.Duration
}

// SendWebhook sends a simple text message to Discord
func SendWebhook(ctx context.Context, webhookURL, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	payload := DiscordPayload{Content: message}
	return sendPayload(ctx, webhookURL, payload)
}

// SendRepoNotification sends a formatted notification for a single repo
func SendRepoNotification(ctx context.Context, webhookURL string, repo RepoInfo) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	color := 0x00ff00 // Green for success
	if !repo.BackedUp {
		color = 0xff0000 // Red for failure
	}

	status := "✅ Successfully queued"
	if !repo.BackedUp {
		status = "❌ Failed to backup"
		if repo.Error != "" {
			status += fmt.Sprintf(": %s", repo.Error)
		}
	}

	description := repo.Description
	if description == "" {
		description = "No description available"
	}
	if len(description) > 100 {
		description = description[:97] + "..."
	}

	embed := DiscordEmbed{
		Title:       fmt.Sprintf("🔍 New Repository Found: %s", repo.Name),
		Description: description,
		Color:       color,
		Fields: []DiscordEmbedField{
			{Name: "⭐ Stars", Value: fmt.Sprintf("%d", repo.Stars), Inline: true},
			{Name: "💻 Language", Value: repo.Language, Inline: true},
			{Name: "📊 Status", Value: status, Inline: false},
			{Name: "🔗 URL", Value: fmt.Sprintf("[View Repository](%s)", repo.URL), Inline: false},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "GitHub Mirror Bot"},
	}

	payload := DiscordPayload{Embeds: []DiscordEmbed{embed}}
	return sendPayload(ctx, webhookURL, payload)
}

// SendBackupSummary sends a comprehensive summary of the backup operation
func SendBackupSummary(ctx context.Context, webhookURL string, summary BackupSummary, maxLength int) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	// Determine overall status color
	color := 0x00ff00 // Green
	if summary.TotalFailed > 0 {
		if summary.TotalBackedUp == 0 {
			color = 0xff0000 // Red - all failed
		} else {
			color = 0xffaa00 // Orange - partial success
		}
	} else if summary.TotalFound == 0 {
		color = 0x808080 // Gray - no repos found
	}

	// Create main embed
	mainEmbed := DiscordEmbed{
		Title:     "📊 Backup Operation Summary",
		Color:     color,
		Timestamp: time.Now().Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "GitHub Mirror Bot"},
		Fields: []DiscordEmbedField{
			{Name: "🔍 Total Found", Value: fmt.Sprintf("%d", summary.TotalFound), Inline: true},
			{Name: "✅ Successfully Backed Up", Value: fmt.Sprintf("%d", summary.TotalBackedUp), Inline: true},
			{Name: "❌ Failed", Value: fmt.Sprintf("%d", summary.TotalFailed), Inline: true},
			{Name: "⏱️ Duration", Value: summary.Duration.Round(time.Second).String(), Inline: true},
		},
	}

	if summary.TotalFound == 0 {
		mainEmbed.Description = "No new repositories found matching the criteria."
		payload := DiscordPayload{Embeds: []DiscordEmbed{mainEmbed}}
		return sendPayload(ctx, webhookURL, payload)
	}

	// Create detailed repo list
	var repoDetails strings.Builder
	for i, repo := range summary.Repos {
		status := "✅"
		if !repo.BackedUp {
			status = "❌"
		}

		line := fmt.Sprintf("%s **%s** (%d⭐) - %s\n", 
			status, repo.Name, repo.Stars, truncateString(repo.Description, 50))
		
		// Check if adding this line would exceed the limit
		if repoDetails.Len()+len(line) > maxLength-200 { // Leave room for embed structure
			remaining := len(summary.Repos) - i
			repoDetails.WriteString(fmt.Sprintf("... and %d more repositories", remaining))
			break
		}
		
		repoDetails.WriteString(line)
	}

	if repoDetails.Len() > 0 {
		mainEmbed.Fields = append(mainEmbed.Fields, DiscordEmbedField{
			Name:  "📋 Repository Details",
			Value: repoDetails.String(),
			Inline: false,
		})
	}

	payload := DiscordPayload{Embeds: []DiscordEmbed{mainEmbed}}
	return sendPayload(ctx, webhookURL, payload)
}

// SendBackupScriptSummary sends summary from the backup script
func SendBackupScriptSummary(ctx context.Context, webhookURL string, totalRepos int, successCount int, failCount int, uploadSuccess bool, duration time.Duration) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	color := 0x00ff00 // Green
	if failCount > 0 || !uploadSuccess {
		if successCount == 0 {
			color = 0xff0000 // Red
		} else {
			color = 0xffaa00 // Orange
		}
	}

	uploadStatus := "✅ Successfully uploaded to Google Drive"
	if !uploadSuccess {
		uploadStatus = "❌ Failed to upload to Google Drive"
	}

	embed := DiscordEmbed{
		Title:     "💾 Repository Backup Complete",
		Color:     color,
		Timestamp: time.Now().Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "GitHub Backup Script"},
		Fields: []DiscordEmbedField{
			{Name: "📊 Total Repositories", Value: fmt.Sprintf("%d", totalRepos), Inline: true},
			{Name: "✅ Successfully Backed Up", Value: fmt.Sprintf("%d", successCount), Inline: true},
			{Name: "❌ Failed", Value: fmt.Sprintf("%d", failCount), Inline: true},
			{Name: "⏱️ Duration", Value: duration.Round(time.Second).String(), Inline: true},
			{Name: "☁️ Cloud Storage", Value: uploadStatus, Inline: false},
		},
	}

	if totalRepos == 0 {
		embed.Description = "No repositories found to backup."
	}

	payload := DiscordPayload{Embeds: []DiscordEmbed{embed}}
	return sendPayload(ctx, webhookURL, payload)
}

func sendPayload(ctx context.Context, webhookURL string, payload DiscordPayload) error {
	bs, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(bs))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

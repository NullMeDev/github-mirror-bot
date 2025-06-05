package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DiscordPayload struct {
	Content string `json:"content"`
}

func SendWebhook(ctx context.Context, webhookURL, message string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	payload := DiscordPayload{Content: message}
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

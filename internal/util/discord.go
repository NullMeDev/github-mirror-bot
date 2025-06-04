package util

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// DiscordPayload is the JSON body format Discord expects.
type DiscordPayload struct {
	Content string `json:"content"`
}

// SendWebhook posts a simple text message to the Discord webhook.
func SendWebhook(webhookURL, message string) error {
	payload := DiscordPayload{Content: message}
	bs, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(bs))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	_, err = client.Do(req)
	return err
}

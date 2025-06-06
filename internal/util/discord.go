package util

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "time"
)

type DiscordWebhook struct {
    URL string
}

func NewDiscordWebhookFromURL(url string) *DiscordWebhook {
    return &DiscordWebhook{URL: url}
}

func (w *DiscordWebhook) SendMessage(message string) error {
    payload := map[string]string{"content": message}
    body, err := json.Marshal(payload)
    if err != nil {
        return err
    }

    var resp *http.Response
    var lastErr error

    for attempts := 0; attempts < 5; attempts++ {
        resp, err = http.Post(w.URL, "application/json", bytes.NewBuffer(body))
        if err != nil {
            lastErr = err
            log.Printf("Discord webhook send attempt %d failed: %v", attempts+1, err)
            time.Sleep(time.Duration(attempts+1) * time.Second)
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode == 429 {
            data, _ := ioutil.ReadAll(resp.Body)
            log.Printf("Discord rate limited: %s", string(data))
            retryAfter := resp.Header.Get("Retry-After")
            waitDur, _ := time.ParseDuration(retryAfter + "ms")
            time.Sleep(waitDur + time.Second)
            continue
        }

        if resp.StatusCode >= 200 && resp.StatusCode < 300 {
            return nil
        } else {
            data, _ := ioutil.ReadAll(resp.Body)
            lastErr = fmt.Errorf("discord webhook error %d: %s", resp.StatusCode, string(data))
            log.Printf("Discord webhook returned error: %v", lastErr)
            time.Sleep(time.Duration(attempts+1) * time.Second)
        }
    }

    return fmt.Errorf("failed to send discord message after retries: %v", lastErr)
}

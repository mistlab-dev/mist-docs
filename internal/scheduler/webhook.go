package scheduler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// postWebhook delivers one JSON payload to a webhook URL.
//
// This is intentionally a separate small helper rather than a reuse of the
// handler package's fireWebhooks: that one is a fan-out keyed on document
// events, while the scheduler needs a request/response outcome per endpoint so
// it can record delivery status in md_reminder_log.
func postWebhook(url, secret, teamID string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", "deadline.reminder")
	if secret != "" {
		req.Header.Set("X-Webhook-Secret", secret)
	}
	if teamID != "" {
		req.Header.Set("X-Team-Id", teamID)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return &httpError{code: resp.StatusCode}
	}
	return nil
}

type httpError struct{ code int }

func (e *httpError) Error() string {
	return "webhook returned HTTP " + itoa(e.code)
}

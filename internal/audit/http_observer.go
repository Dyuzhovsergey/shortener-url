package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPObserver отправляет события аудита на удалённый сервер методом POST.
type HTTPObserver struct {
	client *http.Client
	url    string
}

// NewHTTPObserver создаёт HTTPObserver.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		client: &http.Client{Timeout: 2 * time.Second},
		url:    url,
	}
}

// Observe реализует интерфейс Observer.
func (o *HTTPObserver) Observe(ctx context.Context, event Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}

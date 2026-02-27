package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublisher_Publish_NoObservers(t *testing.T) {
	p := NewPublisher()
	if err := p.Publish(context.Background(), Event{TS: 1, Action: "shorten", UserID: "u", URL: "https://example.com"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

type errObserver struct{ err error }

func (o errObserver) Observe(_ context.Context, _ Event) error { return o.err }

func TestPublisher_Publish_MultipleObservers(t *testing.T) {
	p := NewPublisher()
	p.Add(errObserver{err: nil})
	p.Add(errObserver{err: os.ErrInvalid})

	err := p.Publish(context.Background(), Event{TS: 1, Action: "shorten", UserID: "u", URL: "https://example.com"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFileObserver_AppendLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	obs, err := NewFileObserver(path)
	if err != nil {
		t.Fatalf("NewFileObserver: %v", err)
	}
	defer obs.Close()

	ev1 := Event{TS: 1, Action: "shorten", UserID: "u1", URL: "https://example.com/1"}
	ev2 := Event{TS: 2, Action: "follow", UserID: "u2", URL: "https://example.com/2"}

	if err := obs.Observe(context.Background(), ev1); err != nil {
		t.Fatalf("Observe(1): %v", err)
	}
	if err := obs.Observe(context.Background(), ev2); err != nil {
		t.Fatalf("Observe(2): %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var got1, got2 Event
	if err := json.Unmarshal([]byte(lines[0]), &got1); err != nil {
		t.Fatalf("unmarshal line1: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &got2); err != nil {
		t.Fatalf("unmarshal line2: %v", err)
	}

	if got1 != ev1 {
		t.Fatalf("line1 mismatch: %#v != %#v", got1, ev1)
	}
	if got2 != ev2 {
		t.Fatalf("line2 mismatch: %#v != %#v", got2, ev2)
	}
}

func TestHTTPObserver_Post(t *testing.T) {
	var called int32
	ch := make(chan Event, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		defer r.Body.Close()

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var ev Event
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ch <- ev
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	obs := NewHTTPObserver(ts.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ev := Event{TS: 123, Action: "shorten", UserID: "u1", URL: "https://example.com"}
	if err := obs.Observe(ctx, ev); err != nil {
		t.Fatalf("Observe: %v", err)
	}

	select {
	case got := <-ch:
		if got != ev {
			t.Fatalf("event mismatch: %#v != %#v", got, ev)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting event")
	}

	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
}

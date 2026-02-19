package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/handler"
	"github.com/Dyuzhovsergey/shortener-url/internal/model"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

// testBaseURL используется сервисом для формирования коротких ссылок.
// В примерах мы не печатаем сам shortID (он случайный), только проверяем коды и заголовки.
const testBaseURL = "http://short.local"

// okDB — заглушка для /ping.
type okDB struct{}

func (okDB) PingContext(ctx context.Context) error { return nil }

// newTestServer поднимает HTTP сервер на in-memory репозитории.
func newTestServer() *httptest.Server {
	cfg := &config.ShortenerConfig{
		CharSet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		LengthID: 8,
		BaseURL:  testBaseURL,
	}

	repo := repository.NewMemoryRepository()
	svc := service.NewShorterService(repo, cfg)

	log := zap.NewNop()

	aud := audit.NewPublisher()

	srv := handler.NewHTTPServer(testBaseURL, svc, log, okDB{}, aud)

	return httptest.NewServer(srv.Router())
}

// newClient возвращает http.Client.
// Если withJar=true, клиент будет сохранять cookie (нужно для /api/user/urls и удаления).
func newClient(withJar bool) *http.Client {
	c := &http.Client{Timeout: 2 * time.Second}
	if !withJar {
		return c
	}
	jar, _ := cookiejar.New(nil)
	c.Jar = jar
	return c
}

// shortenPlain делает POST / и возвращает shortID.
func shortenPlain(client *http.Client, base string, original string) (string, int, error) {
	req, err := http.NewRequest(http.MethodPost, base+"/", strings.NewReader(original))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	shortURL := strings.TrimSpace(string(body))

	u, err := url.Parse(shortURL)
	if err != nil {
		return "", resp.StatusCode, err
	}
	id := path.Base(u.Path)
	return id, resp.StatusCode, nil
}

func ExampleHTTPServer_shortenPlain() {
	ts := newTestServer()
	defer ts.Close()

	client := newClient(false)

	id, status, err := shortenPlain(client, ts.URL, "https://example.com/long/path")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// shortID случайный
	shortLen := len(testBaseURL + "/" + id)

	fmt.Println("status:", status)
	fmt.Println("short_url_len:", shortLen)

	// Output:
	// status: 201
	// short_url_len: 27
}

func ExampleHTTPServer_shortenJSON() {
	ts := newTestServer()
	defer ts.Close()

	client := newClient(false)

	reqBody, _ := json.Marshal(model.ShortenRequest{URL: "https://example.com/api/shorten"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/shorten", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer resp.Body.Close()

	var out model.ShortenResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)

	fmt.Println("status:", resp.StatusCode)
	fmt.Println("has_prefix:", strings.HasPrefix(out.Result, testBaseURL+"/"))
	fmt.Println("short_url_len:", len(out.Result))

	// Output:
	// status: 201
	// has_prefix: true
	// short_url_len: 27
}

func ExampleHTTPServer_followRedirect() {
	ts := newTestServer()
	defer ts.Close()

	client := newClient(false)

	id, status, err := shortenPlain(client, ts.URL, "https://example.com/redirect/me")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if status != http.StatusCreated {
		fmt.Println("unexpected status:", status)
		return
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/"+id, nil)
	req.Header.Set("Accept-Encoding", "identity")

	// Отключаем автоматический follow.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("status:", resp.StatusCode)
	fmt.Println("location:", resp.Header.Get("Location"))

	// Output:
	// status: 307
	// location: https://example.com/redirect/me
}

func ExampleHTTPServer_batch() {
	ts := newTestServer()
	defer ts.Close()

	client := newClient(false)

	in := []model.BatchShortenRequestItem{
		{CorrelationID: "1", OriginalURL: "https://example.com/b1"},
		{CorrelationID: "2", OriginalURL: "https://example.com/b2"},
	}
	reqBody, _ := json.Marshal(in)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/shorten/batch", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer resp.Body.Close()

	var out []model.BatchShortenResponseItem
	_ = json.NewDecoder(resp.Body).Decode(&out)

	fmt.Println("status:", resp.StatusCode)
	fmt.Println("count:", len(out))
	fmt.Println("corr_ids:", out[0].CorrelationID, out[1].CorrelationID)

	// Output:
	// status: 201
	// count: 2
	// corr_ids: 1 2
}

func ExampleHTTPServer_userURLsAndDelete() {
	ts := newTestServer()
	defer ts.Close()

	client := newClient(true)

	id1, _, err := shortenPlain(client, ts.URL, "https://example.com/u1")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	id2, _, err := shortenPlain(client, ts.URL, "https://example.com/u2")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// GET /api/user/urls
	reqGet, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/user/urls", nil)
	reqGet.Header.Set("Accept-Encoding", "identity")

	respGet, err := client.Do(reqGet)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer respGet.Body.Close()

	count := 0
	if respGet.StatusCode == http.StatusOK {
		var out []model.UserURLResponse
		_ = json.NewDecoder(respGet.Body).Decode(&out)
		count = len(out)
	}

	// DELETE /api/user/urls
	delBody, _ := json.Marshal([]string{id1, id2})
	reqDel, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", bytes.NewReader(delBody))
	reqDel.Header.Set("Content-Type", "application/json")
	reqDel.Header.Set("Accept-Encoding", "identity")

	respDel, err := client.Do(reqDel)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer respDel.Body.Close()

	fmt.Println("get_status:", respGet.StatusCode)
	fmt.Println("urls_count:", count)
	fmt.Println("delete_status:", respDel.StatusCode)

	// Output:
	// get_status: 200
	// urls_count: 2
	// delete_status: 202
}

func ExampleHTTPServer_ping() {
	ts := newTestServer()
	defer ts.Close()

	client := newClient(false)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/ping", nil)
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("status:", resp.StatusCode)

	// Output:
	// status: 200
}

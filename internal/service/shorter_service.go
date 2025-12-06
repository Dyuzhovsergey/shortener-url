// Package service for business logic project
package service

import (
	"context"
	"errors"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
)

const maxAttempts = 10

// ShorterService отвечает за бизнес-логику: валидацию, генерацию ID и сохранение ссылок.
type ShorterService struct {
	repo repository.Repository
	cfg  *config.ShortenerConfig
	rnd  *rand.Rand
	mu   sync.Mutex
}

// NewShorterService - Конструктор с внедрением зависимости (DI)
func NewShorterService(repo repository.Repository, cfg *config.ShortenerConfig) *ShorterService {
	return &ShorterService{
		repo: repo,
		cfg:  cfg,
		rnd:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// BatchItem — один элемент батч-запроса для сервиса.
type BatchItem struct {
	CorrelationID string
	OriginalURL   string
}

// BatchResult — результат обработки одного элемента батча.
type BatchResult struct {
	CorrelationID string
	ShortURL      string
}

// CreateShortURL — создаёт короткую ссылку и сохраняет её.
func (svc *ShorterService) CreateShortURL(ctx context.Context, originalURL string, baseURL string) (string, error) {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return "", errors.New("invalid URL format")
	}

	parsedURL, err := url.ParseRequestURI(originalURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("invalid URL format")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", errors.New("unsupported URL scheme")
	}

	var shortID string
	for i := 0; i < maxAttempts; i++ {
		shortID = svc.generateID()
		if _, exists := svc.repo.Get(ctx, shortID); !exists {
			break
		}
	}

	if _, exists := svc.repo.Get(ctx, shortID); exists {
		return "", errors.New("failed to generate unique shortID")
	}

	if err := svc.repo.Save(ctx, shortID, originalURL); err != nil {
		return "", err
	}

	shortURL := baseURL + "/" + shortID
	return shortURL, nil
}

// GetOriginalURL — возвращает оригинальный URL по shortID
func (svc *ShorterService) GetOriginalURL(ctx context.Context, shortID string) (string, bool) {
	return svc.repo.Get(ctx, strings.TrimSpace(shortID))
}

// generateID — генерирует случайный shortID.
func (svc *ShorterService) generateID() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	id := make([]byte, svc.cfg.LengthID)
	for i := range id {
		id[i] = svc.cfg.CharSet[svc.rnd.Intn(len(svc.cfg.CharSet))]
	}
	return string(id)
}

// CreateShortURLBatch — обрабатывает батч URL'ов.
// На каждый originalURL создаёт shortID, сохраняет через repo и возвращает список результатов.
func (svc *ShorterService) CreateShortURLBatch(ctx context.Context, baseURL string, items []BatchItem) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, errors.New("empty batch")
	}

	results := make([]BatchResult, 0, len(items))

	for _, it := range items {
		// используем уже существующую логику CreateShortURL:
		shortURL, err := svc.CreateShortURL(ctx, it.OriginalURL, baseURL)
		if err != nil {
			return nil, err
		}

		results = append(results, BatchResult{
			CorrelationID: it.CorrelationID,
			ShortURL:      shortURL,
		})
	}

	return results, nil
}

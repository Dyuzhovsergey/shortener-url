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
	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
)

const maxAttempts = 10

var (
	ErrAlreadyExists   = errors.New("URL already exists")
	ErrDeleteQueueFull = errors.New("delete queue is full")
)

type deleteTask struct {
	userID   string
	shortIDs []string
}

// ShorterService отвечает за бизнес-логику: валидацию, генерацию ID и сохранение ссылок.
type ShorterService struct {
	repo repository.Repository
	cfg  *config.ShortenerConfig
	rnd  *rand.Rand
	mu   sync.Mutex

	deleteCh chan deleteTask
}

// NewShorterService - Конструктор с внедрением зависимости (DI)
func NewShorterService(repo repository.Repository, cfg *config.ShortenerConfig) *ShorterService {
	svc := &ShorterService{
		repo:     repo,
		cfg:      cfg,
		rnd:      rand.New(rand.NewSource(time.Now().UnixNano())),
		deleteCh: make(chan deleteTask, 1024),
	}

	go svc.deleteWorker() // асинхронный воркер
	return svc
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

	baseURL = strings.TrimRight(baseURL, "/")

	var shortID string

	for i := 0; i < maxAttempts; i++ {
		shortID = svc.generateID()

		_, ok, err := svc.repo.Get(ctx, shortID)
		if err != nil {
			return "", err
		}
		if !ok {
			break
		}
	}

	_, ok, err := svc.repo.Get(ctx, shortID)
	if err != nil {
		return "", err
	}
	if ok {
		return "", errors.New("failed to generate unique shortID")
	}

	userID, _ := middleware.UserIDFromContext(ctx)

	if err := svc.repo.Save(ctx, shortID, originalURL, userID); err != nil {
		var dup *repository.ErrOriginalAlreadyExists
		if errors.As(err, &dup) {
			existingShortURL := baseURL + "/" + dup.ShortID
			return existingShortURL, ErrAlreadyExists
		}
		return "", err
	}

	shortURL := baseURL + "/" + shortID

	return shortURL, nil
}

// GetOriginalURL — возвращает оригинальный URL по shortID
func (svc *ShorterService) GetOriginalURL(ctx context.Context, shortID string) (string, bool, error) {
	shortID = strings.TrimSpace(shortID)
	return svc.repo.Get(ctx, shortID)
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

// GetUserURLs для получения ссылок пользователя
func (svc *ShorterService) GetUserURLs(ctx context.Context) ([]repository.UserURL, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok || userID == "" {
		return nil, nil
	}
	return svc.repo.GetUserURLs(ctx, userID)
}

// CreateShortURLBatch — обрабатывает батч URL'ов.
func (svc *ShorterService) CreateShortURLBatch(ctx context.Context, baseURL string, items []BatchItem) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, errors.New("empty batch")
	}

	results := make([]BatchResult, 0, len(items))

	for _, it := range items {
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

func (svc *ShorterService) DeleteUserURLsAsync(ctx context.Context, shortIDs []string) error {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok || userID == "" {
		return errors.New("unauthorized")
	}
	if len(shortIDs) == 0 {
		return errors.New("empty list")
	}

	cp := make([]string, len(shortIDs))
	copy(cp, shortIDs)

	select {
	case svc.deleteCh <- deleteTask{userID: userID, shortIDs: cp}:
		return nil
	default:
		return ErrDeleteQueueFull // err 503/500
	}
}

func (svc *ShorterService) deleteWorker() {
	for task := range svc.deleteCh {
		_ = svc.repo.DeleteUserURLs(context.Background(), task.userID, task.shortIDs)
	}
}

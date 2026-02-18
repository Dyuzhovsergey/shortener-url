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
	// ErrAlreadyExists возвращается при попытке сократить URL, который уже есть в хранилище.
	ErrAlreadyExists = errors.New("URL already exists")
	// ErrDeleteQueueFull возвращается, если очередь на асинхронное удаление переполнена.
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

	idPool sync.Pool
}

// NewShorterService создаёт сервис сокращения URL и запускает фонового воркера для удаления.
func NewShorterService(repo repository.Repository, cfg *config.ShortenerConfig) *ShorterService {
	svc := &ShorterService{
		repo:     repo,
		cfg:      cfg,
		rnd:      rand.New(rand.NewSource(time.Now().UnixNano())),
		deleteCh: make(chan deleteTask, 1024),
	}
	// Пул буферов под генерацию shortID.
	svc.idPool.New = func() any {
		b := make([]byte, svc.cfg.LengthID)
		return &b
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
	unique := false

	for i := 0; i < maxAttempts; i++ {

		shortID = svc.generateID()

		_, ok, err := svc.repo.Get(ctx, shortID)
		if err != nil {
			return "", err
		}
		if !ok {
			unique = true
			break
		}
	}

	if !unique {
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
	bufp := svc.idPool.Get().(*[]byte)
	buf := *bufp

	if cap(buf) < svc.cfg.LengthID {
		buf = make([]byte, svc.cfg.LengthID)
	}
	buf = buf[:svc.cfg.LengthID]

	svc.mu.Lock()
	for i := 0; i < svc.cfg.LengthID; i++ {
		buf[i] = svc.cfg.CharSet[svc.rnd.Intn(len(svc.cfg.CharSet))]
	}
	svc.mu.Unlock()
	id := string(buf)

	*bufp = buf
	svc.idPool.Put(bufp)

	return id
}

// GetUserURLs возвращает список ссылок текущего пользователя (из контекста запроса).
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

// DeleteUserURLsAsync ставит удаление ссылок пользователя в очередь и возвращает сразу (202 на HTTP-уровне)
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
	const (
		batchSize  = 100
		flushEvery = 200 * time.Millisecond
	)

	// userID -> set(shortID)
	pending := make(map[string]map[string]struct{})

	flushUser := func(userID string) {
		set := pending[userID]
		if len(set) == 0 {
			return
		}

		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}

		_ = svc.repo.DeleteUserURLs(context.Background(), userID, ids)
		delete(pending, userID)
	}

	flushAll := func() {
		for userID := range pending {
			flushUser(userID)
		}
	}

	t := time.NewTicker(flushEvery)
	defer t.Stop()

	for {
		select {
		case task, ok := <-svc.deleteCh:
			if !ok {
				flushAll()
				return
			}

			if task.userID == "" || len(task.shortIDs) == 0 {
				continue
			}

			set, ok := pending[task.userID]
			if !ok {
				set = make(map[string]struct{})
				pending[task.userID] = set
			}

			for _, id := range task.shortIDs {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				set[id] = struct{}{}
			}

			if len(set) >= batchSize {
				flushUser(task.userID)
			}

		case <-t.C:
			flushAll()
		}
	}
}

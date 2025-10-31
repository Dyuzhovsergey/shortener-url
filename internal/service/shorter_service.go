// Package service for business logic project
package service

import (
	"errors"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

// ShorterService отвечает за бизнес-логику: валидацию, генерацию ID и сохранение ссылок.
type ShorterService struct {
	repo repository.Repository
}

// NewShorterService - Конструктор с внедрением зависимости (DI)
func NewShorterService(repo repository.Repository) *ShorterService {
	return &ShorterService{repo: repo}
}

// CreateShortURL — создаёт короткую ссылку и сохраняет её.
func (svc *ShorterService) CreateShortURL(originalURL string, baseURL string) (string, error) {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return "", errors.New("invalid URL format")
	}
	parsedURL, err := url.ParseRequestURI(originalURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("invalid URL format")
	}

	shortID := svc.generateID()

	if err := svc.repo.Save(shortID, originalURL); err != nil {
		return "", err
	}

	shortURL := baseURL + "/" + shortID
	return shortURL, nil
}

// GetOriginalURL — возвращает оригинальный URL по shortID
func (svc *ShorterService) GetOriginalURL(shortID string) (string, bool) {
	return svc.repo.Get(strings.TrimSpace(shortID))
}

// generateID — генерирует случайный shortID.
func (svc *ShorterService) generateID() string {
	id := make([]byte, config.Param.LengthID)
	for i := range id {
		id[i] = config.Param.CharSet[rnd.Intn(len(config.Param.CharSet))]
	}
	return string(id)
}

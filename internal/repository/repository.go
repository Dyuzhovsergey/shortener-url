package repository

import (
	"context"
	"errors"
)

// ErrDeleted возвращается методом Repository.Get, если ссылка найдена, но помечена как удалённая.
// На HTTP-уровне обычно соответствует статусу 410 Gone.
var ErrDeleted = errors.New("url is deleted")

// UserURL представляет ссылку пользователя в формате {shortID, originalURL}.
type UserURL struct {
	ShortID     string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Stats содержит агрегированную статистику сервиса.
type Stats struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// Repository описывает поведение хранилища сокращённых ссылок.
type Repository interface {
	// Save сохраняет соответствие shortID -> originalURL и привязывает его к userID (если задан).
	Save(ctx context.Context, shortID, originalURL, userID string) error

	// Get возвращает originalURL по shortID.
	// ok=false означает, что shortID не найден.
	// err=ErrDeleted означает, что shortID найден, но помечен как удалённый.
	Get(ctx context.Context, shortID string) (string, bool, error)

	// GetUserURLs возвращает все активные ссылки пользователя.
	GetUserURLs(ctx context.Context, userID string) ([]UserURL, error)

	// DeleteUserURLs помечает ссылки пользователя как удалённые.
	DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error

	// Stats возвращает агрегированную статистику сервиса.
	Stats(ctx context.Context) (Stats, error)
}

// ErrOriginalAlreadyExists означает, что originalURL уже сохранён в системе.
// ShortID содержит уже существующий shortID для этого originalURL.
type ErrOriginalAlreadyExists struct {
	ShortID string
}

func (e *ErrOriginalAlreadyExists) Error() string {
	return "original URL already exists"
}

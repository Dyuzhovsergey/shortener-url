// Package repository for interface storage short URL
package repository

import (
	"context"
	"errors"
)

var ErrDeleted = errors.New("url is deleted")

type UserURL struct {
	ShortID     string
	OriginalURL string
}

// Repository — интерфейс для хранилища сокращённых ссылок.
type Repository interface {
	Save(ctx context.Context, shortID, originalURL, userID string) error
	// Get:
	// originalURL — строка URL
	// ok=false => shortID не найден
	// err=ErrDeleted => shortID найден, но помечен как удалённый
	Get(ctx context.Context, shortID string) (string, bool, error)
	GetUserURLs(ctx context.Context, userID string) ([]UserURL, error)

	// DeleteUserURLs — помечает ссылки как удалённые (асинхронно вызывается из service)
	DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error
}

type ErrOriginalAlreadyExists struct {
	ShortID string
}

func (e *ErrOriginalAlreadyExists) Error() string {
	return "original URL already exists"
}

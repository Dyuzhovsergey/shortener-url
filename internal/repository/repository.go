// Package repository for interface storage short URL
package repository

import "context"

// Repository — интерфейс для хранилища сокращённых ссылок.
type Repository interface {
	Save(ctx context.Context, shortID, originalURL string) error
	Get(ctx context.Context, shortID string) (string, bool)
}

type ErrOriginalAlreadyExists struct {
	ShortID string
}

func (e *ErrOriginalAlreadyExists) Error() string {
	return "original URL already exists"
}

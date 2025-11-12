// Package repository for interface storage short URL
package repository

// Repository — интерфейс для хранилища сокращённых ссылок.
type Repository interface {
	Save(shortID, originalURL string) error
	Get(shortID string) (string, bool)
}

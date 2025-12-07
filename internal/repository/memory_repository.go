package repository

import (
	"context"
	"sync"
)

// MemoryRepository — реализация Repository в оперативной памяти.
type MemoryRepository struct {
	data map[string]string
	mu   sync.RWMutex
}

// NewMemoryRepository — конструктор.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		data: make(map[string]string),
	}
}

// Save сохраняет оригинальный URL по shortID.
func (repo *MemoryRepository) Save(ctx context.Context, shortID, originalURL string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	for sid, url := range repo.data {
		if url == originalURL {
			// URL уже есть, возвращаем специальную ошибку с существующим shortID
			return &ErrOriginalAlreadyExists{ShortID: sid}
		}
	}

	repo.data[shortID] = originalURL
	return nil
}

// Get возвращает оригинальный URL по shortID.
func (repo *MemoryRepository) Get(ctx context.Context, shortID string) (string, bool) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	url, ok := repo.data[shortID]
	return url, ok
}

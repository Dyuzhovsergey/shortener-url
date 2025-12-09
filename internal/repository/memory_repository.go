package repository

import (
	"context"
	"sync"
)

// MemoryRepository — реализация Repository в оперативной памяти.
type MemoryRepository struct {
	data    map[string]string // shortID -> originalURL
	reverse map[string]string // originalURL -> shortID
	mu      sync.RWMutex
}

// NewMemoryRepository — конструктор.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		data:    make(map[string]string),
		reverse: make(map[string]string),
	}
}

// Save сохраняет оригинальный URL по shortID.
func (repo *MemoryRepository) Save(ctx context.Context, shortID, originalURL string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	// 1. Если по этому shortID уже есть другой URL — почистим reverse
	if oldURL, ok := repo.data[shortID]; ok && oldURL != originalURL {
		delete(repo.reverse, oldURL)
	}

	// 2. Проверяем, не существует ли уже такой originalURL
	if existingShortID, ok := repo.reverse[originalURL]; ok && existingShortID != shortID {
		return &ErrOriginalAlreadyExists{ShortID: existingShortID}
	}

	// 3. Обновляем обе мапы
	repo.data[shortID] = originalURL
	repo.reverse[originalURL] = shortID

	return nil
}

// Get возвращает оригинальный URL по shortID.
func (repo *MemoryRepository) Get(ctx context.Context, shortID string) (string, bool) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	url, ok := repo.data[shortID]
	return url, ok
}

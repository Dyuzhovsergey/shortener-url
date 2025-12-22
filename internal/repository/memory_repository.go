package repository

import (
	"context"
	"sync"
)

type memRec struct {
	OriginalURL string
	UserID      string
	Deleted     bool
}

// MemoryRepository — реализация Repository в оперативной памяти.
type MemoryRepository struct {
	data      map[string]memRec            // shortID -> record
	reverse   map[string]string            // originalURL -> shortID
	userIndex map[string]map[string]string // userID -> (shortID -> originalURL)
	mu        sync.RWMutex
}

// NewMemoryRepository — конструктор.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		data:      make(map[string]memRec),
		reverse:   make(map[string]string),
		userIndex: make(map[string]map[string]string),
	}
}

// Save сохраняет оригинальный URL по shortID.
func (repo *MemoryRepository) Save(ctx context.Context, shortID, originalURL, userID string) error {
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

	if userID != "" {
		m, ok := repo.userIndex[userID]
		if !ok {
			m = make(map[string]string)
			repo.userIndex[userID] = m
		}
		m[shortID] = originalURL
	}

	return nil
}

// Get возвращает оригинальный URL по shortID.
func (repo *MemoryRepository) Get(ctx context.Context, shortID string) (string, bool) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	url, ok := repo.data[shortID]
	return url, ok
}

func (repo *MemoryRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	if userID == "" {
		return nil, nil
	}

	repo.mu.RLock()
	defer repo.mu.RUnlock()

	m, ok := repo.userIndex[userID]
	if !ok || len(m) == 0 {
		return nil, nil
	}

	res := make([]UserURL, 0, len(m))
	for shortID, originalURL := range m {
		res = append(res, UserURL{
			ShortID:     shortID,
			OriginalURL: originalURL,
		})
	}
	return res, nil
}

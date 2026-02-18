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
	data      map[string]memRec              // shortID -> record
	reverse   map[string]string              // originalURL -> shortID
	userIndex map[string]map[string]struct{} // userID -> set(shortID)
	mu        sync.RWMutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		data:      make(map[string]memRec),
		reverse:   make(map[string]string),
		userIndex: make(map[string]map[string]struct{}),
	}
}

// Save сохраняет ссылку в памяти.
func (repo *MemoryRepository) Save(ctx context.Context, shortID, originalURL, userID string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	// если shortID уже был и указывал на другой URL — чистим reverse у старого URL
	if old, ok := repo.data[shortID]; ok && old.OriginalURL != originalURL {
		delete(repo.reverse, old.OriginalURL)
	}

	// если originalURL уже есть -> конфликт
	if existingShortID, ok := repo.reverse[originalURL]; ok && existingShortID != shortID {
		return &ErrOriginalAlreadyExists{ShortID: existingShortID}
	}

	// записываем/обновляем
	repo.data[shortID] = memRec{
		OriginalURL: originalURL,
		UserID:      userID,
		Deleted:     false,
	}
	repo.reverse[originalURL] = shortID

	if userID != "" {
		set, ok := repo.userIndex[userID]
		if !ok {
			set = make(map[string]struct{})
			repo.userIndex[userID] = set
		}
		set[shortID] = struct{}{}
	}

	return nil
}

func (repo *MemoryRepository) Get(ctx context.Context, shortID string) (string, bool, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	rec, ok := repo.data[shortID]
	if !ok {
		return "", false, nil
	}
	if rec.Deleted {
		return "", true, ErrDeleted
	}
	return rec.OriginalURL, true, nil
}

func (repo *MemoryRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	if userID == "" {
		return nil, nil
	}

	repo.mu.RLock()
	defer repo.mu.RUnlock()

	set, ok := repo.userIndex[userID]
	if !ok || len(set) == 0 {
		return nil, nil
	}

	res := make([]UserURL, 0, len(set))
	for shortID := range set {
		rec, ok := repo.data[shortID]
		if !ok || rec.Deleted {
			continue
		}
		res = append(res, UserURL{ShortID: shortID, OriginalURL: rec.OriginalURL})
	}
	return res, nil
}

func (repo *MemoryRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	if userID == "" || len(shortIDs) == 0 {
		return nil
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()

	for _, id := range shortIDs {
		rec, ok := repo.data[id]
		if !ok {
			continue
		}

		if rec.UserID != userID {
			continue
		}

		if rec.Deleted {
			continue
		}

		rec.Deleted = true
		repo.data[id] = rec
	}
	return nil
}

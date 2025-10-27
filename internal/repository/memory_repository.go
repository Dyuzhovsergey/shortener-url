package repository

import "sync"

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
func (m *MemoryRepository) Save(shortID, originalURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[shortID] = originalURL
	return nil
}

// Get возвращает оригинальный URL по shortID.
func (m *MemoryRepository) Get(shortID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	url, ok := m.data[shortID]
	return url, ok
}

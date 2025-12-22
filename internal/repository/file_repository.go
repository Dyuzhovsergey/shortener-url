package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync"
)

type urlRecord struct {
	RecordID    string `json:"record_id"`
	ShortID     string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id"`
	IsDeleted   bool   `json:"is_deleted"`
}

// FileRepository — реализация Repository с сохранением на диск.
type FileRepository struct {
	mu sync.RWMutex

	data      map[string]string            // shortID -> originalURL
	reverse   map[string]string            // originalURL -> shortID
	userIndex map[string]map[string]string // userID -> (shortID -> originalURL)
	deleted   map[string]bool              // shortID -> is_deleted

	records  []urlRecord
	filePath string
}

func NewFileRepository(path string) (*FileRepository, error) {
	fr := &FileRepository{
		data:      make(map[string]string),
		reverse:   make(map[string]string),
		userIndex: make(map[string]map[string]string),
		deleted:   make(map[string]bool),
		records:   make([]urlRecord, 0),
		filePath:  path,
	}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return fr, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return fr, nil
	}

	var records []urlRecord
	dec := json.NewDecoder(f)
	if err := dec.Decode(&records); err != nil {
		return nil, err
	}

	// восстанавливаем индексы из файла
	for _, rec := range records {
		fr.records = append(fr.records, rec)

		fr.data[rec.ShortID] = rec.OriginalURL
		fr.reverse[rec.OriginalURL] = rec.ShortID

		fr.deleted[rec.ShortID] = rec.IsDeleted

		if rec.UserID != "" {
			m, ok := fr.userIndex[rec.UserID]
			if !ok {
				m = make(map[string]string)
				fr.userIndex[rec.UserID] = m
			}
			m[rec.ShortID] = rec.OriginalURL
		}
	}

	return fr, nil
}

// Save сохраняет originalURL по shortID и userID.
func (fr *FileRepository) Save(ctx context.Context, shortID, originalURL, userID string) error {
	_ = ctx

	fr.mu.Lock()
	defer fr.mu.Unlock()

	// 1) Если originalURL уже существует — конфликт
	if existingShortID, ok := fr.reverse[originalURL]; ok && existingShortID != shortID {
		return &ErrOriginalAlreadyExists{ShortID: existingShortID}
	}

	// 2) Если shortID уже был и указывал на другой URL — чистим reverse у старого URL
	if oldURL, ok := fr.data[shortID]; ok && oldURL != originalURL {
		delete(fr.reverse, oldURL)
	}

	// 3) сохраняем индексы
	fr.data[shortID] = originalURL
	fr.reverse[originalURL] = shortID
	fr.deleted[shortID] = false

	if userID != "" {
		m, ok := fr.userIndex[userID]
		if !ok {
			m = make(map[string]string)
			fr.userIndex[userID] = m
		}
		m[shortID] = originalURL
	}

	// 4) добавляем запись в records
	recordID := strconv.Itoa(len(fr.records) + 1)
	fr.records = append(fr.records, urlRecord{
		RecordID:    recordID,
		ShortID:     shortID,
		OriginalURL: originalURL,
		UserID:      userID,
		IsDeleted:   false,
	})

	return fr.rewriteFileLocked()
}

// Get возвращает originalURL по shortID.
func (fr *FileRepository) Get(ctx context.Context, shortID string) (string, bool, error) {
	_ = ctx

	fr.mu.RLock()
	defer fr.mu.RUnlock()

	original, ok := fr.data[shortID]
	if !ok {
		return "", false, nil
	}
	if fr.deleted[shortID] {
		return "", true, ErrDeleted
	}
	return original, true, nil
}

func (fr *FileRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	_ = ctx

	if userID == "" {
		return nil, nil
	}

	fr.mu.RLock()
	defer fr.mu.RUnlock()

	m, ok := fr.userIndex[userID]
	if !ok || len(m) == 0 {
		return nil, nil
	}

	res := make([]UserURL, 0, len(m))
	for shortID, originalURL := range m {
		if fr.deleted[shortID] {
			continue
		}
		res = append(res, UserURL{
			ShortID:     shortID,
			OriginalURL: originalURL,
		})
	}

	return res, nil
}

// DeleteUserURLs помечает ссылки удалёнными
func (fr *FileRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	_ = ctx

	if userID == "" || len(shortIDs) == 0 {
		return nil
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()

	owned, ok := fr.userIndex[userID]
	if !ok || len(owned) == 0 {
		return nil
	}

	// 1) помечаем deleted в map
	for _, id := range shortIDs {
		if _, ok := owned[id]; !ok {
			continue
		}
		fr.deleted[id] = true
	}

	// 2) синхронизируем records
	for i := range fr.records {
		id := fr.records[i].ShortID
		if fr.deleted[id] {
			fr.records[i].IsDeleted = true
		}
	}

	return fr.rewriteFileLocked()
}

// rewriteFileLocked перезаписывает файл текущими fr.records.
func (fr *FileRepository) rewriteFileLocked() error {
	f, err := os.OpenFile(fr.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(fr.records)
}

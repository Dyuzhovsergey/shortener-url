package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	reverse   map[string]string            // originalURL -> shortID (для быстрого поиска дубликатов)
	userIndex map[string]map[string]string // userID -> (shortID -> originalURL)
	deleted   map[string]bool              // shortID -> is_deleted

	records  []urlRecord // то, что пишем в файл
	filePath string
}

// NewFileRepository создаёт файловый репозиторий и загружает данные из файла, если он есть.
func NewFileRepository(path string) (*FileRepository, error) {
	fr := &FileRepository{
		data:      make(map[string]string),
		reverse:   make(map[string]string),
		userIndex: make(map[string]map[string]string),
		deleted:   make(map[string]bool),
		records:   make([]urlRecord, 0),
		filePath:  path,
	}

	// открываем файл
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

	// читаем JSON-массив
	body, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var records []urlRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, err
	}

	// наполняем мапу и внутренний слайс
	// восстанавливаем индексы
	for _, rec := range records {
		fr.records = append(fr.records, rec)
		fr.data[rec.ShortID] = rec.OriginalURL
		fr.reverse[rec.OriginalURL] = rec.ShortID

		if rec.UserID != "" {
			m, ok := fr.userIndex[rec.UserID]
			if !ok {
				m = make(map[string]string)
				fr.userIndex[rec.UserID] = m
			}
			m[rec.ShortID] = rec.OriginalURL
		}
		fr.deleted[rec.ShortID] = rec.IsDeleted
	}

	return fr, nil
}

// Save сохраняет оригинальный URL по shortID и перезаписывает файл.
func (fr *FileRepository) Save(ctx context.Context, shortID, originalURL, userID string) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	// 1. Если по этому shortID уже был другой URL — подчистим reverse
	if oldURL, ok := fr.data[shortID]; ok && oldURL != originalURL {
		delete(fr.reverse, oldURL)
	}

	// 2. Проверяем, не существует ли уже такой originalURL
	if existingShortID, ok := fr.reverse[originalURL]; ok && existingShortID != shortID {
		return &ErrOriginalAlreadyExists{ShortID: existingShortID}
	}

	// 3) сохраняем в индексы
	fr.data[shortID] = originalURL
	fr.reverse[originalURL] = shortID
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
	fr.deleted[shortID] = false

	// 5) перезаписываем файл
	f, err := os.OpenFile(fr.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fr.records); err != nil {
		return err
	}

	return nil
}

// Get возвращает оригинальный URL по shortID.
func (fr *FileRepository) Get(ctx context.Context, shortID string) (string, bool, error) {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	url, ok := fr.data[shortID]
	if !ok {
		return "", false, nil
	}
	if fr.deleted[shortID] {
		return "", true, ErrDeleted
	}
	return url, true, nil
}


func (fr *FileRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	if userID == "" {
		return nil, nil
	}

	fr.mu.RLock()
	defer fr.mu.RUnlock()

	m, ok := fr.userIndex[userID]
	if !ok || len(m) == 0 {
		return nil, nil
	}
	if fr.deleted[shortID] {
	continue
}

func (fr *FileRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	if userID == "" || len(shortIDs) == 0 {
		return nil
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()

	owned, ok := fr.userIndex[userID]
	if !ok || len(owned) == 0 {
		return nil
	}

	// помечаем удалёнными в индексе
	for _, id := range shortIDs {
		if _, ok := owned[id]; !ok {
			continue
		}
		fr.deleted[id] = true
	}

	// синхронизируем records (чтобы пережить рестарт)
	for i := range fr.records {
		id := fr.records[i].ShortID
		if fr.deleted[id] {
			fr.records[i].IsDeleted = true
		}
	}

	// перезаписываем файл
	f, err := os.OpenFile(fr.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(fr.records)
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

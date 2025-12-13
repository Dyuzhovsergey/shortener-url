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
}

// FileRepository — реализация Repository с сохранением на диск.
type FileRepository struct {
	mu sync.RWMutex

	data      map[string]string            // shortID -> originalURL
	reverse   map[string]string            // originalURL -> shortID (для быстрого поиска дубликатов)
	userIndex map[string]map[string]string // userID -> (shortID -> originalURL)

	records  []urlRecord // то, что пишем в файл
	filePath string
}

// NewFileRepository создаёт файловый репозиторий и загружает данные из файла, если он есть.
func NewFileRepository(path string) (*FileRepository, error) {
	fr := &FileRepository{
		data:      make(map[string]string),
		reverse:   make(map[string]string),
		userIndex: make(map[string]map[string]string),
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
	})

	// 5) перезаписываем файл (у тебя так и было)
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
func (fr *FileRepository) Get(ctx context.Context, shortID string) (string, bool) {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	url, ok := fr.data[shortID]
	return url, ok
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

	res := make([]UserURL, 0, len(m))
	for shortID, originalURL := range m {
		res = append(res, UserURL{
			ShortID:     shortID,
			OriginalURL: originalURL,
		})
	}
	return res, nil
}

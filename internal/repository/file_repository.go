package repository

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"sync"
)

type urlRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// FileRepository — реализация Repository с сохранением на диск.
type FileRepository struct {
	mu       sync.RWMutex
	data     map[string]string // shortID -> originalURL
	records  []urlRecord       // для записи и чтения файла
	filePath string
}

// NewFileRepository создаёт файловый репозиторий и загружает данные из файла, если он есть.
func NewFileRepository(path string) (*FileRepository, error) {
	fr := &FileRepository{
		data:     make(map[string]string),
		records:  make([]urlRecord, 0),
		filePath: path,
	}

	// открываем файл
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		// файла нет — это нормально, создадим позже
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
	for _, rec := range records {
		fr.records = append(fr.records, rec)
		fr.data[rec.ShortURL] = rec.OriginalURL
	}

	return fr, nil
}

// Save сохраняет оригинальный URL по shortID и перезаписывает файл.
func (fr *FileRepository) Save(shortID, originalURL string) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	// если уже есть такая запись — просто обновим мапу и файл
	fr.data[shortID] = originalURL

	// добавляем новую запись с "uuid" = порядковый номер
	uuid := strconv.Itoa(len(fr.records) + 1)
	rec := urlRecord{
		UUID:        uuid,
		ShortURL:    shortID,
		OriginalURL: originalURL,
	}
	fr.records = append(fr.records, rec)

	// открываем файл на перезапись
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
func (fr *FileRepository) Get(shortID string) (string, bool) {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	url, ok := fr.data[shortID]
	return url, ok
}

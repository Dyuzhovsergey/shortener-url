package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// urlRecord — запись в файловом хранилище.
//
// Поддерживаем 2 формата:
// 1) Старый: один JSON-массив []urlRecord (историческая версия).
// 2) Новый (оптимизированный): построчный JSON (NDJSON) — одна запись на строке (append-only).
type urlRecord struct {
	RecordID    string `json:"record_id"`
	ShortID     string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id"`
	IsDeleted   bool   `json:"is_deleted"`
}

// FileRepository — реализация Repository с сохранением на диск.
//
// Оптимизация памяти и аллокаций:
// - Больше НЕ храним весь срез records в памяти.
// - Больше НЕ перезаписываем файл целиком при каждом Save/Delete.
// - Пишем только "события" (append-only) по одной JSON-строке.
//
// При старте репозиторий восстанавливает индексы, проигрывая файл (replay).
type FileRepository struct {
	mu sync.RWMutex

	data      map[string]string            // shortID -> originalURL
	reverse   map[string]string            // originalURL -> shortID
	userIndex map[string]map[string]string // userID -> (shortID -> originalURL)
	deleted   map[string]bool              // shortID -> is_deleted

	filePath string
	// nextRecordID нужен только чтобы сохранять совместимость поля record_id.
	// Для работы репозитория он не важен.
	nextRecordID int
}

func NewFileRepository(path string) (*FileRepository, error) {
	fr := &FileRepository{
		data:         make(map[string]string),
		reverse:      make(map[string]string),
		userIndex:    make(map[string]map[string]string),
		deleted:      make(map[string]bool),
		filePath:     path,
		nextRecordID: 1,
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

	format, err := detectFileFormat(f)
	if err != nil {
		return nil, err
	}

	// Возвращаемся в начало файла после детекта.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	switch format {
	case "array":
		var records []urlRecord
		dec := json.NewDecoder(f)
		if err := dec.Decode(&records); err != nil {
			return nil, err
		}
		for _, rec := range records {
			fr.applyRecordLocked(rec)
		}
	case "ndjson":
		scanner := bufio.NewScanner(f)
		// На всякий случай увеличим лимит строки.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var rec urlRecord
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				return nil, err
			}
			fr.applyRecordLocked(rec)
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unknown file format")
	}

	return fr, nil
}

// detectFileFormat определяет формат файла хранилища.
//
// array  -> начинается с '[' (после пропуска пробелов)
// ndjson -> иначе
func detectFileFormat(f *os.File) (string, error) {
	// читаем первые несколько байт, пропуская пробелы/переводы строк
	buf := make([]byte, 1)
	for {
		n, err := f.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "ndjson", nil
			}
			return "", err
		}
		if n == 0 {
			continue
		}
		b := buf[0]
		switch b {
		case ' ', '\n', '\r', '\t':
			continue
		case '[':
			return "array", nil
		default:
			return "ndjson", nil
		}
	}
}

// applyRecordLocked применяет запись к индексам репозитория.
// Вызывать только под lock'ом (инициализация — без lock, но в одном потоке).
func (fr *FileRepository) applyRecordLocked(rec urlRecord) {
	if rec.ShortID == "" {
		return
	}

	// Восстанавливаем record_id, чтобы следующий ID был уникальным.
	if rec.RecordID != "" {
		if id, err := strconv.Atoi(rec.RecordID); err == nil {
			if id >= fr.nextRecordID {
				fr.nextRecordID = id + 1
			}
		}
	}

	// Индексы восстанавливаем всегда, даже если запись помечена как удалённая:
	// это важно, чтобы Get() мог вернуть 410 (ErrDeleted) по shortID.
	if rec.OriginalURL != "" {
		fr.data[rec.ShortID] = rec.OriginalURL
		fr.reverse[rec.OriginalURL] = rec.ShortID
	}

	if rec.UserID != "" && rec.OriginalURL != "" {
		m, ok := fr.userIndex[rec.UserID]
		if !ok {
			m = make(map[string]string)
			fr.userIndex[rec.UserID] = m
		}
		m[rec.ShortID] = rec.OriginalURL
	}

	fr.deleted[rec.ShortID] = rec.IsDeleted
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

	// 4) пишем append-only запись в файл
	rec := urlRecord{
		RecordID:    strconv.Itoa(fr.nextRecordID),
		ShortID:     shortID,
		OriginalURL: originalURL,
		UserID:      userID,
		IsDeleted:   false,
	}
	fr.nextRecordID++

	return fr.appendRecordLocked(rec)
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

// DeleteUserURLs помечает ссылки удалёнными.
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

	// Пишем записи только для реально изменённых shortID.
	for _, id := range shortIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := owned[id]; !ok {
			continue
		}
		if fr.deleted[id] {
			continue
		}

		originalURL := fr.data[id]
		fr.deleted[id] = true

		rec := urlRecord{
			RecordID:    strconv.Itoa(fr.nextRecordID),
			ShortID:     id,
			OriginalURL: originalURL,
			UserID:      userID,
			IsDeleted:   true,
		}
		fr.nextRecordID++

		if err := fr.appendRecordLocked(rec); err != nil {
			return err
		}
	}

	return nil
}

// appendRecordLocked добавляет одну запись в конец файла (NDJSON).
// Вызывать только под fr.mu.Lock().
func (fr *FileRepository) appendRecordLocked(rec urlRecord) error {
	f, err := os.OpenFile(fr.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(rec) // Encode сам добавит "\n" в конце
}

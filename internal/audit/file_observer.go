package audit

import (
	"context"
	"encoding/json"
	"os"
	"sync"
)

// FileObserver пишет события аудита в файл (append-only, по одной JSON-строке на событие).
type FileObserver struct {
	mu sync.Mutex
	f  *os.File
}

// NewFileObserver открывает (или создаёт) файл для записи аудита.
func NewFileObserver(path string) (*FileObserver, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileObserver{f: f}, nil
}

// Observe сериализует событие в JSON и дописывает его в файл отдельной строкой.
func (o *FileObserver) Observe(_ context.Context, event Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	b, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if _, err := o.f.Write(append(b, '\n')); err != nil {
		return err
	}

	// Явный Sync повышает шанс сохранить данные при аварийной остановке.
	return o.f.Sync()
}

// Close закрывает файл.
func (o *FileObserver) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.f == nil {
		return nil
	}
	err := o.f.Close()
	o.f = nil
	return err
}

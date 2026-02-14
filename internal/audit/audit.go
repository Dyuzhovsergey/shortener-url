// Package audit содержит функциональность аудита событий сервиса (паттерн «Наблюдатель»).
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Observer — наблюдатель (приёмник) событий аудита.
// Реализация может писать события в файл, отправлять на удалённый сервер и т.п.
type Observer interface {
	Observe(ctx context.Context, event Event) error
}

// Publisher — субъект (наблюдаемый объект).
// Хранит список наблюдателей и оповещает их о новых событиях.
type Publisher struct {
	mu        sync.RWMutex
	observers []Observer
}

// NewPublisher создаёт Publisher.
func NewPublisher() *Publisher {
	return &Publisher{}
}

// Add подписывает наблюдателя на события.
func (p *Publisher) Add(obs Observer) {
	if obs == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.observers = append(p.observers, obs)
}

// Publish рассылает событие всем наблюдателям.
//
// Если часть приёмников вернула ошибку — вернём агрегированную ошибку,
// но рассылка будет выполнена всем.
func (p *Publisher) Publish(ctx context.Context, event Event) error {
	p.mu.RLock()
	observers := append([]Observer(nil), p.observers...)
	p.mu.RUnlock()

	if len(observers) == 0 {
		return nil
	}

	var errs []error
	for _, obs := range observers {
		if obs == nil {
			continue
		}
		if err := obs.Observe(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("audit observe failed: %w", err))
		}
	}
	if len(errs) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("multiple audit errors: ")
	for i, err := range errs {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(err.Error())
	}
	return errors.New(b.String())
}

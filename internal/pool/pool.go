// Package pool generic
package pool

import "sync"

// Resetter — ограничение для generic-типа: объект обязан иметь метод Reset().
// Важно: чаще всего Reset() генерируется для указателя на структуру (например: func (s *MyStruct) Reset()).
// Тогда тип T будет выглядеть как *MyStruct.
type Resetter interface {
	Reset()
}

// Pool — обёртка над sync.Pool для объектов одного конкретного типа T.
// Перед возвратом в пул (Put) объект всегда сбрасывается через Reset().
type Pool[T Resetter] struct {
	p sync.Pool
}

// New — конструктор пула.
// newFn — функция, которая создаёт новый объект T, если пул пуст.
func New[T Resetter](newFn func() T) *Pool[T] {
	pl := &Pool[T]{}
	pl.p.New = func() any {
		return newFn()
	}
	return pl
}

// Get — получить объект из пула (или новый, если пул пуст).
func (pl *Pool[T]) Get() T {
	// sync.Pool.Get() возвращает any.
	// Мы гарантируем, что в пул кладём только T, поэтому приведение безопасно.
	return pl.p.Get().(T)
}

// Put — вернуть объект в пул.
// ВАЖНО: перед возвратом обязательно сбрасываем состояние объекта через Reset().
func (pl *Pool[T]) Put(v T) {
	// Даже если v — nil-указатель, вызов метода возможен.
	// Нормальная реализация Reset() должна начинаться с проверки "if v == nil { return }".
	v.Reset()
	pl.p.Put(v)
}

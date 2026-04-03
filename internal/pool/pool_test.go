package pool

import (
	"runtime/debug"
	"testing"
)

type demo struct {
	I int
	S []int
	M map[string]int
}

func (d *demo) Reset() {
	if d == nil {
		return
	}
	d.I = 0
	if d.S != nil {
		d.S = d.S[:0]
	}
	clear(d.M)
}

func TestPool_ResetOnPut(t *testing.T) {
	// sync.Pool может очищаться при GC, поэтому отключим авто-GC,
	// чтобы тест был стабильным.
	old := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(old)

	p := New(func() *demo {
		return &demo{
			I: 777,
			S: make([]int, 0, 10),
			M: make(map[string]int),
		}
	})

	x := p.Get()
	if x.I != 777 {
		t.Fatalf("ожидали I=777 у нового объекта, получили %d", x.I)
	}

	// "Загрязняем" объект
	x.I = 42
	x.S = append(x.S, 1, 2, 3)
	x.M["k"] = 1

	// Возвращаем в пул — тут должен выполниться Reset()
	p.Put(x)

	y := p.Get()

	// Проверяем, что состояние сброшено
	if y.I != 0 {
		t.Fatalf("ожидали I=0 после Reset, получили %d", y.I)
	}
	if len(y.S) != 0 {
		t.Fatalf("ожидали len(S)=0 после Reset, получили %d", len(y.S))
	}
	if len(y.M) != 0 {
		t.Fatalf("ожидали len(M)=0 после Reset, получили %d", len(y.M))
	}
}

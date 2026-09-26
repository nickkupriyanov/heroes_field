package sim

import (
	"bytes"
	"testing"
)

// detScript — сценарий действий для тестов детерминизма: шаги направо
// до упора в стену (недопустимые шаги отвергаются и просто пропускаются,
// как игрок, пробующий другое), затем прогулка по залу.
var detScript = []Action{
	Wait(),
	Move(Vec2{X: 1, Y: 0}),
	Move(Vec2{X: 1, Y: 0}),
	Move(Vec2{X: 1, Y: 0}),
	Move(Vec2{X: 1, Y: 0}),
	Move(Vec2{X: 1, Y: 0}), // (7,2)
	Move(Vec2{X: 1, Y: 0}),
	Move(Vec2{X: 1, Y: 0}), // (9,2)
	Move(Vec2{X: 1, Y: 0}), // стена x=10 — отвергается
	Move(Vec2{X: 1, Y: 0}), // снова стена — отвергается
	Wait(),
	Move(Vec2{X: 0, Y: 1}), // (9,3)
	Move(Vec2{X: 0, Y: 1}),
	Move(Vec2{X: 0, Y: 1}),
	Move(Vec2{X: 0, Y: 1}),  // (9,6)
	Move(Vec2{X: -1, Y: 0}), // (8,6)
	Move(Vec2{X: -1, Y: 0}),
	Move(Vec2{X: -1, Y: 0}), // (6,6)
	Wait(),
}

// detRun проигрывает фиксированный сценарий и возвращает канонический JSONL.
func detRun(t *testing.T, seed uint64) []byte {
	t.Helper()
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, ev := NewWorld(s, seed)
	stream := []Event{ev}
	for _, a := range detScript {
		evs, err := w.Step(a)
		if err != nil {
			continue // недопустимое намерение — как игрок, пробующий другое
		}
		stream = append(stream, evs...)
	}
	var buf bytes.Buffer
	if err := EncodeEvents(&buf, stream); err != nil {
		t.Fatalf("кодирование: %v", err)
	}
	return buf.Bytes()
}

// TestDeterminismByteToByte — ядро контракта: одинаковый genesis, seed
// и действия обязаны давать байт-в-байт одинаковый JSONL.
func TestDeterminismByteToByte(t *testing.T) {
	first := detRun(t, 42)
	second := detRun(t, 42)
	if !bytes.Equal(first, second) {
		t.Error("две одинаковые партии дали разные потоки событий")
	}
}

// TestDeterminismMapIterationInvariance — порядок вставки/обхода Go-map
// не должен влиять на результат. Симуляция никогда не итерирует map
// (только срез order/itemOrder), а порядок обхода map в Go случаен
// почти при каждом обходе — поэтому 25 свежих прогонов одной партии
// ловят случайную итерацию практически наверняка.
func TestDeterminismMapIterationInvariance(t *testing.T) {
	reference := detRun(t, 7)
	for run := 1; run < 25; run++ {
		if got := detRun(t, 7); !bytes.Equal(reference, got) {
			t.Fatalf("прогон %d отличается от первого: где-то итерируется map или время", run)
		}
	}
}

// TestDeterminismDifferentSeeds — разные seed дают разные genesis
// (и только: пока в ходах нет случайности). После M4 различия появятся
// и в самих ходах — тест не станет строже случайно.
func TestDeterminismDifferentSeeds(t *testing.T) {
	a := detRun(t, 1)
	b := detRun(t, 2)
	if bytes.Equal(a, b) {
		t.Error("разные seed дали одинаковый поток")
	}
}

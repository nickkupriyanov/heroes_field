package journal

import (
	"strings"
	"testing"

	"afterhero/internal/sim"
)

// TestPhraseBasics — фразы строятся из событий, называют участников по именам
// и не выводят служебные события (TurnEnded не для глаз).
func TestPhraseBasics(t *testing.T) {
	s, err := sim.DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, ev := sim.NewWorld(s, 3)
	stream := []sim.Event{ev}
	for _, a := range []sim.Action{sim.Wait(), sim.Move(sim.Vec2{X: 1, Y: 0})} {
		events, err := w.Step(a)
		if err != nil {
			t.Fatalf("шаг: %v", err)
		}
		stream = append(stream, events...)
	}
	snap := w.Snapshot()

	waited := Phrase(stream[1], snap)
	if !strings.Contains(waited, "гоблин #1") {
		t.Errorf("фраза ожидания не называет актёра: %q", waited)
	}
	moved := Phrase(stream[3], snap)
	if !strings.Contains(moved, "на восток") {
		t.Errorf("фраза шага не называет направление: %q", moved)
	}
	if got := Phrase(stream[len(stream)-1], snap); got != "" {
		t.Errorf("TurnEnded не должен печататься: %q", got)
	}
}

// TestCreatureName — имена постоянны: одно имя — один житель на всю партию,
// труп прошлого тела опознаётся по тому же имени.
func TestCreatureName(t *testing.T) {
	s, err := sim.DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, _ := sim.NewWorld(s, 1)
	snap := w.Snapshot()
	if got := CreatureName(1, snap); got != "гоблин #1" {
		t.Errorf("первое существо зовут %q, ожидается «гоблин #1»", got)
	}
	// Второе по карте — культист в (24,2): порядок ID = порядок чтения карты.
	if got := CreatureName(2, snap); got != "культис #2" {
		t.Errorf("второе существо зовут %q, ожидается «культис #2»", got)
	}
}

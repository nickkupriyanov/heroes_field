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

	waited := phraseOfKind(t, stream, snap, sim.EventWaited, sim.EntityID(1))
	if !strings.Contains(waited, "гоблин #1") {
		t.Errorf("фраза ожидания не называет актёра: %q", waited)
	}
	moved := phraseOfKind(t, stream, snap, sim.EventMoved, sim.EntityID(1))
	if !strings.Contains(moved, "на восток") {
		t.Errorf("фраза шага не называет направление: %q", moved)
	}
	if got := Phrase(stream[len(stream)-1], snap); got != "" {
		t.Errorf("TurnEnded не должен печататься: %q", got)
	}
}

// phraseOfKind — первая фраза события нужного вида от нужного актёра:
// тесты журнала не должны зависеть от количества событий в ходе.
func phraseOfKind(t *testing.T, stream []sim.Event, snap sim.Snapshot, kind sim.EventKind, actor sim.EntityID) string {
	t.Helper()
	for _, e := range stream {
		if e.Kind == kind && e.Actor == actor {
			return Phrase(e, snap)
		}
	}
	t.Fatalf("в потоке нет события вида %d от актёра %d", kind, actor)
	return ""
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

// TestEatCorpsePhrase — фраза еды трупа называет жертву в родительном:
// «гоблин #1 съедает труп гоблина #4» — каннибализм обязан называться
// по имени, иначе это просто ужин.
func TestEatCorpsePhrase(t *testing.T) {
	s, err := sim.DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, _ := sim.NewWorld(s, 5)
	var events []sim.Event
	walk := func(to sim.Vec2) {
		t.Helper()
		for w.Creature(1).Pos != to {
			d := sim.Vec2{X: to.X - w.Creature(1).Pos.X, Y: to.Y - w.Creature(1).Pos.Y}
			if d.X > 1 {
				d.X = 1
			}
			if d.X < -1 {
				d.X = -1
			}
			if d.Y > 1 {
				d.Y = 1
			}
			if d.Y < -1 {
				d.Y = -1
			}
			evs, err := w.Step(sim.Move(d))
			if err != nil {
				t.Fatalf("проводка: %v", err)
			}
			events = append(events, evs...)
		}
	}
	walk(sim.Vec2{X: 3, Y: 5})
	var corpse sim.ItemID
	for i := 0; i < 3; i++ {
		evs, err := w.Step(sim.Attack(sim.Vec2{X: 0, Y: 1}))
		if err != nil {
			t.Fatalf("удар %d: %v", i+1, err)
		}
		events = append(events, evs...)
		for _, e := range evs {
			if e.Kind == sim.EventCorpseCreated {
				corpse = e.Item
			}
		}
	}
	walk(sim.Vec2{X: 3, Y: 6})
	for _, a := range []sim.Action{sim.PickUp(corpse), sim.Use(corpse)} {
		evs, err := w.Step(a)
		if err != nil {
			t.Fatalf("%+v: %v", a, err)
		}
		events = append(events, evs...)
	}
	snap := w.Snapshot()
	var phrase string
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == sim.EventItemUsed {
			phrase = Phrase(events[i], snap)
			break
		}
	}
	if phrase != "гоблин #1 съедает труп гоблина #4" {
		t.Errorf("фраза еды трупа: %q", phrase)
	}
}

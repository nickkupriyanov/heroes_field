package sim

import "testing"

// newFrozenWorld — мир со стоячими жителями: тестам механики хода нужна
// статичная сцена, где позиции не плывут от AI. Поведения AI тестируются
// на живых мирах — в behavior_test.
func newFrozenWorld(t *testing.T, seed uint64) (*World, Event) {
	t.Helper()
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, ev := NewWorld(s, seed)
	w.SetAI(PassiveAI{})
	return w, ev
}

// newParty — стандартная сцена тестов: замороженный мир, seed 42.
func newParty(t *testing.T) *World {
	t.Helper()
	w, _ := newFrozenWorld(t, 42)
	return w
}

// walkTo проводит игрока к клетке шагами по осям и возвращает события
// всех состоявшихся ходов — они нужны тестам с replay, чтобы поток
// не дырявился.
func walkTo(t *testing.T, w *World, to Vec2) []Event {
	t.Helper()
	var events []Event
	player := w.Creature(w.Player())
	for player.Pos != to {
		d := Vec2{}
		if to.X > player.Pos.X {
			d.X = 1
		} else if to.X < player.Pos.X {
			d.X = -1
		}
		if to.Y > player.Pos.Y {
			d.Y = 1
		} else if to.Y < player.Pos.Y {
			d.Y = -1
		}
		evs, err := w.Step(Move(d))
		if err != nil {
			t.Fatalf("проводка к %v из %v: %v", to, player.Pos, err)
		}
		events = append(events, evs...)
		player = w.Creature(w.Player())
	}
	return events
}

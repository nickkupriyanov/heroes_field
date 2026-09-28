package sim

import (
	"errors"
	"testing"
)

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

// TestPickUpAndDrop — полный цикл предмета: подобрал → в инвентаре,
// выбросил → лежит в клетке. Единственный источник правды — Item.Holder.
func TestPickUpAndDrop(t *testing.T) {
	w := newParty(t)
	// Еда #1 лежит в (6,2): идём к ней и поднимаем.
	walkTo(t, w, Vec2{X: 6, Y: 2})
	events, err := w.Step(PickUp(1))
	if err != nil {
		t.Fatalf("PickUp(1): %v", err)
	}
	if events[0].Kind != EventItemPickedUp || events[0].Item != 1 {
		t.Fatalf("события поднятия: %+v", events[0])
	}
	if events[len(events)-1].Kind != EventTurnEnded {
		t.Fatalf("ход не завершён после поднятия")
	}
	if inv := w.InventoryOf(1); len(inv) != 1 || inv[0] != 1 {
		t.Errorf("инвентарь игрока после поднятия: %v", inv)
	}
	if it := w.Item(1); it == nil || it.Holder != 1 {
		t.Errorf("предмет не в руках: %+v", it)
	}

	events, err = w.Step(Drop(1))
	if err != nil {
		t.Fatalf("Drop(1): %v", err)
	}
	if events[0].Kind != EventItemDropped {
		t.Fatalf("события выбрасывания: %+v", events[0])
	}
	if it := w.Item(1); it == nil || it.Holder != 0 || it.Pos != (Vec2{X: 6, Y: 2}) {
		t.Errorf("предмет не вернулся на пол: %+v", it)
	}
	if inv := w.InventoryOf(1); len(inv) != 0 {
		t.Errorf("инвентарь не опустел: %v", inv)
	}
}

// TestPickUpErrors — поднятие чужого и далёкого, выбрасывание не своего:
// ошибки без расхода хода.
func TestPickUpErrors(t *testing.T) {
	w := newParty(t)
	// Предмет #1 в (6,2), игрок в (2,2): поднимать издалека нельзя.
	if _, err := w.Step(PickUp(1)); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("поднятие издалека: %v", err)
	}
	if w.Turn() != 0 {
		t.Errorf("ошибка сожгла ход: turn=%d", w.Turn())
	}
	// Несуществующий предмет.
	if _, err := w.Step(PickUp(99)); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("поднятие несуществующего: %v", err)
	}
	// Выбрасывание того, чего нет в руках.
	if _, err := w.Step(Drop(1)); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("выбрасывание не своего: %v", err)
	}
	// Теперь поднимем по правилам и попробуем выбросить чужое.
	walkTo(t, w, Vec2{X: 6, Y: 2})
	if _, err := w.Step(PickUp(1)); err != nil {
		t.Fatalf("PickUp(1): %v", err)
	}
	if _, err := w.Step(Drop(2)); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("выбрасывание чужого предмета: %v", err)
	}
	if w.Turn() != 5 {
		t.Errorf("после пяти состоявшихся ходов turn=%d, ожидается 5", w.Turn())
	}
}

// TestInventoryDerivation — инвентарь выводится из Holder и переживает
// replay байт-в-байт.
func TestInventoryDerivation(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, genesis := NewWorld(s, 9)
	stream := []Event{genesis}
	stream = append(stream, walkTo(t, w, Vec2{X: 6, Y: 2})...)
	for _, a := range []Action{PickUp(1), Move(Vec2{X: -1, Y: 0}), Drop(1)} {
		evs, err := w.Step(a)
		if err != nil {
			t.Fatalf("%+v: %v", a, err)
		}
		stream = append(stream, evs...)
	}
	if inv := w.InventoryOf(1); len(inv) != 0 {
		t.Fatalf("инвентарь после выбрасывания: %v", inv)
	}
	rep, err := WorldFromEvents(stream)
	if err != nil {
		t.Fatalf("WorldFromEvents: %v", err)
	}
	if got, want := len(rep.InventoryOf(1)), 0; got != want {
		t.Errorf("инвентарь в replay: %d, ожидается %d", got, want)
	}
	// Поднятие на один ход и ходьба с предметом: Holder сопровождает актёра.
	evs, _ := w.Step(PickUp(1))
	stream = append(stream, evs...)
	if it := w.Item(1); it.Holder != 1 {
		t.Fatalf("предмет не в руках: %+v", it)
	}
	rep2, _ := WorldFromEvents(stream)
	if it := rep2.Item(1); it == nil || it.Holder != 1 {
		t.Errorf("replay потерял Holder: %+v", it)
	}
}

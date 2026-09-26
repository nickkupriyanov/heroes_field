package sim

import (
	"errors"
	"reflect"
	"testing"
)

// newParty — вспомогательная функция тестов: мир по умолчанию с фиксированным
// seed. Тесты хода не зависят от случайности (её в M1 ещё нет), но единая
// заготовка не даст этому протухнуть незаметно.
func newParty(t *testing.T) *World {
	t.Helper()
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, _ := NewWorld(s, 42)
	return w
}

// TestStepWait проверяет простейший ход: факт ожидания + завершение хода,
// и только они.
func TestStepWait(t *testing.T) {
	w := newParty(t)
	events, err := w.Step(Wait())
	if err != nil {
		t.Fatalf("Step(Wait): %v", err)
	}
	if len(events) != 2 || events[0].Kind != EventWaited || events[1].Kind != EventTurnEnded {
		t.Fatalf("события хода: %+v", events)
	}
	if events[0].Turn != 1 || events[1].Turn != 1 {
		t.Errorf("номер хода в событиях: %d и %d, ожидается 1", events[0].Turn, events[1].Turn)
	}
	if w.Turn() != 1 {
		t.Errorf("после хода turn=%d, ожидается 1", w.Turn())
	}
	if events[0].Actor != 1 {
		t.Errorf("факт ожидания принадлежит не игроку: %+v", events[0])
	}
}

// TestStepMove проверяет шаг игрока: клетка меняется по событию, факт несёт
// новую позицию и направление.
func TestStepMove(t *testing.T) {
	w := newParty(t)
	events, err := w.Step(Move(Vec2{X: 1, Y: 0}))
	if err != nil {
		t.Fatalf("Step(Move): %v", err)
	}
	if len(events) != 2 || events[0].Kind != EventMoved {
		t.Fatalf("события хода: %+v", events)
	}
	if events[0].Pos == nil || *events[0].Pos != (Vec2{X: 3, Y: 2}) {
		t.Errorf("факт шага несёт позицию %+v, ожидается (3,2)", events[0].Pos)
	}
	if w.Creature(1).Pos != (Vec2{X: 3, Y: 2}) {
		t.Errorf("игрок остался в %+v", w.Creature(1).Pos)
	}
	if w.Turn() != 1 {
		t.Errorf("turn=%d, ожидается 1", w.Turn())
	}
}

// TestStepMoveBlockedWall — шаг в стену: ошибка, ни одного события,
// ход не расходуется.
func TestStepMoveBlockedWall(t *testing.T) {
	w := newParty(t)
	// Игрок в (2,2); проводим его к северной стене границы: (2,1).
	if _, err := w.Step(Move(Vec2{X: 0, Y: -1})); err != nil {
		t.Fatalf("шаг на (2,1): %v", err)
	}
	// Шаг в (2,0) — граница-стена.
	_, err := w.Step(Move(Vec2{X: 0, Y: -1}))
	if !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("шаг в стену: ошибка %v, ожидается ErrInvalidAction", err)
	}
	if w.Turn() != 1 {
		t.Errorf("невалидный шаг сжёг ход: turn=%d", w.Turn())
	}
	if w.Creature(1).Pos != (Vec2{X: 2, Y: 1}) {
		t.Errorf("игрок сдвинулся после ошибки: %+v", w.Creature(1).Pos)
	}
}

// TestStepMoveBlockedCreature — шаг в живого соседа: ошибка без расхода хода.
func TestStepMoveBlockedCreature(t *testing.T) {
	w := newParty(t)
	// Проводим игрока из (2,2) вплотную к гоблину #4 в (4,7):
	// (3,3) → (3,4) → (3,5) → (3,6) занято гоблином #5.
	for _, d := range []Vec2{{X: 1, Y: 1}, {X: 0, Y: 1}, {X: 0, Y: 1}} {
		if _, err := w.Step(Move(d)); err != nil {
			t.Fatalf("проводка к соседу, шаг %+v: %v", d, err)
		}
	}
	if w.Creature(1).Pos != (Vec2{X: 3, Y: 5}) {
		t.Fatalf("проводка не удалась: %+v", w.Creature(1).Pos)
	}
	_, err := w.Step(Move(Vec2{X: 0, Y: 1})) // (3,6) занято гоблином
	if !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("шаг в существо: %v", err)
	}
	if w.Turn() != 3 {
		t.Errorf("невалидный шаг сжёг ход: turn=%d, ожидается 3", w.Turn())
	}
}

// TestStepUnsupportedAction — действие с корректным синтаксисом, но без
// механики (атака, предметы) — валидно по форме и отвергнуто миром.
func TestStepUnsupportedAction(t *testing.T) {
	w := newParty(t)
	if _, err := w.Step(Attack(Vec2{X: 1, Y: 0})); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("атака без боевой механики: %v", err)
	}
	if _, err := w.Step(PickUp(1)); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("поднятие без предметов: %v", err)
	}
	if w.Turn() != 0 {
		t.Errorf("отказ не должен сжигать ход: turn=%d", w.Turn())
	}
}

// TestStepReplayParty — replay партии с настоящими ходами: мир,
// восстановленный из потока, байт-в-байт равен живому.
func TestStepReplayParty(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, genesis := NewWorld(s, 42)
	stream := []Event{genesis}
	for _, a := range []Action{
		Wait(),
		Move(Vec2{X: 1, Y: 0}),
		Move(Vec2{X: 1, Y: 0}),
		Wait(),
		Move(Vec2{X: 0, Y: 1}),
	} {
		evs, err := w.Step(a)
		if err != nil {
			t.Fatalf("шаг %+v: %v", a, err)
		}
		stream = append(stream, evs...)
	}
	rep, err := WorldFromEvents(stream)
	if err != nil {
		t.Fatalf("WorldFromEvents: %v", err)
	}
	if !reflect.DeepEqual(w.Snapshot(), rep.Snapshot()) {
		t.Error("мир после replay партии отличается от живого мира")
	}
}

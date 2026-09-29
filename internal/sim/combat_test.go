package sim

import (
	"errors"
	"reflect"
	"testing"
)

// TestAttackDamage проверяет удар без убийства: два факта (замах и урон),
// здоровье списано только у жертвы.
func TestAttackDamage(t *testing.T) {
	w := newParty(t)
	walkTo(t, w, Vec2{X: 3, Y: 5}) // сосед гоблина #4 в (3,6)
	events, err := w.Step(Attack(Vec2{X: 0, Y: 1}))
	if err != nil {
		t.Fatalf("атака: %v", err)
	}
	// Событие игрока — первое в ходе, конец хода — последнее; между ними NPC.
	if events[0].Kind != EventAttacked || events[1].Kind != EventDamaged {
		t.Fatalf("события атаки: %+v", events[:2])
	}
	if events[len(events)-1].Kind != EventTurnEnded {
		t.Fatalf("ход не завершён: %+v", events[len(events)-1])
	}
	if events[0].Target != 4 || events[1].Target != 4 || events[1].Amount != 2 {
		t.Fatalf("участники и урон: %+v", events[:2])
	}
	if got := w.Creature(4).HP; got != 4 { // 6 − 2
		t.Errorf("здоровье жертвы: %d, ожидается 4", got)
	}
	if got := w.Creature(1).HP; got != 6 {
		t.Errorf("здоровье атакующего изменилось: %d", got)
	}
}

// TestAttackEmptyCell — удар в пустоту: ошибка без расхода хода.
func TestAttackEmptyCell(t *testing.T) {
	w := newParty(t)
	if _, err := w.Step(Attack(Vec2{X: 0, Y: -1})); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("удар в стену прошёл: %v", err)
	}
	if w.Turn() != 0 {
		t.Errorf("ошибка сожгла ход: turn=%d", w.Turn())
	}
}

// TestKillChain проверяет полную цепочку смерти: Died → CorpseCreated →
// выпадение инвентаря, труп не занимает клетку, память мира помнит жертву.
func TestKillChain(t *testing.T) {
	w := newParty(t)
	// Гоблин #4 дойдёт до еды #1 в (6,2) и подберёт её: инвентарь при смерти
	// должен выпасть. act вызывается напрямую: мир — маленький, тест — в пакете.
	for _, d := range []Vec2{
		{X: 0, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: -1},
	} {
		if _, err := w.act(4, Move(d)); err != nil {
			t.Fatalf("проводка гоблина #4: %v", err)
		}
	}
	if _, err := w.act(4, PickUp(1)); err != nil {
		t.Fatalf("гоблин #4 подбирает еду: %v", err)
	}
	if inv := w.InventoryOf(4); len(inv) != 1 {
		t.Fatalf("гоблин #4 без инвентаря: %v", inv)
	}

	// Игрок приходит к (6,2) и убивает гоблина #4 тремя ударами.
	walkTo(t, w, Vec2{X: 5, Y: 2})
	var killEvents []Event
	for i := 0; i < 3; i++ {
		events, err := w.Step(Attack(Vec2{X: 1, Y: 0}))
		if err != nil {
			t.Fatalf("удар %d: %v", i+1, err)
		}
		killEvents = events
	}
	// Третий удар приносит смерть: замах, урон, затем цепочка смерти,
	// затем выжившие NPC (гоблина #4 уже нет — 10 вместо 11), затем TurnEnded.
	if len(killEvents) != 16 { // 5 событий фазы 1 + 10 живых NPC + конец хода
		t.Fatalf("ожидалось 16 событий убийства, есть %d: %+v", len(killEvents), killEvents)
	}
	if killEvents[0].Kind != EventAttacked || killEvents[1].Kind != EventDamaged {
		t.Fatalf("первые события не бой: %+v", killEvents[:2])
	}
	if killEvents[2].Kind != EventDied || killEvents[2].Actor != 1 || killEvents[2].Target != 4 {
		t.Fatalf("факт смерти: %+v", killEvents[2])
	}
	if killEvents[3].Kind != EventCorpseCreated || killEvents[3].Target != 4 {
		t.Fatalf("факт трупа: %+v", killEvents[3])
	}
	corpseID := killEvents[3].Item
	corpse := w.Item(corpseID)
	if corpse == nil || corpse.Kind != ItemCorpse || corpse.Owner != 4 || corpse.Pos != (Vec2{X: 6, Y: 2}) {
		t.Fatalf("труп не зарегистрирован: %+v", corpse)
	}
	if killEvents[4].Kind != EventItemDropped || killEvents[4].Item != 1 || killEvents[4].Actor != 4 {
		t.Fatalf("инвентарь не выпал: %+v", killEvents[4])
	}
	if it := w.Item(1); it == nil || it.Holder != 0 || it.Pos != (Vec2{X: 6, Y: 2}) {
		t.Fatalf("еда не вернулась на пол: %+v", it)
	}
	if w.Creature(4).Alive {
		t.Error("жертва всё ещё жива")
	}
	// Труп — не стена: по клетке смерти можно ходить.
	if w.occupiedBy(Vec2{X: 6, Y: 2}) != 0 {
		t.Error("клетка смерти занята")
	}
}

// TestCombatReplay — партия с убийством восстанавливается из потока:
// труп, здоровье и пустой инвентарь жертвы совпадают с живым миром.
func TestCombatReplay(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, genesis := NewWorld(s, 21)
	w.SetAI(PassiveAI{})
	stream := []Event{genesis}
	stream = append(stream, walkTo(t, w, Vec2{X: 3, Y: 5})...)
	for i := 0; i < 3; i++ {
		evs, err := w.Step(Attack(Vec2{X: 0, Y: 1}))
		if err != nil {
			t.Fatalf("удар %d: %v", i+1, err)
		}
		stream = append(stream, evs...)
	}
	rep, err := WorldFromEvents(stream)
	if err != nil {
		t.Fatalf("WorldFromEvents: %v", err)
	}
	if !reflect.DeepEqual(w.Snapshot(), rep.Snapshot()) {
		t.Error("мир после replay боя отличается от живого")
	}
	if rep.Creature(4).Alive {
		t.Error("в replay жертва выжила")
	}
	// Труп в replay: владелец и место совпадают.
	corpseID := w.nextItem - 1
	corpse := rep.Item(corpseID)
	if corpse == nil || corpse.Kind != ItemCorpse || corpse.Owner != 4 {
		t.Fatalf("труп потерялся в replay: %+v", corpse)
	}
}

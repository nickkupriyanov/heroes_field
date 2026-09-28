package sim

import (
	"reflect"
	"testing"
)

// scriptedAI — контроллер-кукловод для тестов: отвечает по сценарию
// «существо → действие», остальным — ожидание. Так тесты дирижируют
// миром, не трогая симуляцию.
type scriptedAI struct {
	actions map[EntityID]Action
}

func (s scriptedAI) Decide(w *World, id EntityID) Action {
	if a, ok := s.actions[id]; ok {
		return a
	}
	return Wait()
}

// TestNPCPhaseActs проверяет живой конвейер: после хода игрока жители
// действуют по своим намерениям, по возрастанию ID.
func TestNPCPhaseActs(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, _ := NewWorld(s, 5)
	w.SetAI(scriptedAI{actions: map[EntityID]Action{
		4: Move(Vec2{X: 0, Y: -1}), // гоблин #4 из (3,6) в (3,5)
		9: Move(Vec2{X: -1, Y: 0}), // крыса #9 из (15,11) в (14,11)
	}})
	events, err := w.Step(Wait())
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	// События хода: игрок ждёт, затем NPC по возрастанию ID, TurnEnded в конце.
	if events[0].Kind != EventWaited || events[0].Actor != 1 {
		t.Fatalf("первым всегда действует игрок: %+v", events[0])
	}
	if events[len(events)-1].Kind != EventTurnEnded {
		t.Fatalf("последним всегда TurnEnded: %+v", events[len(events)-1])
	}
	// Порядок NPC: гоблин #4 раньше крысы #9.
	var order []EntityID
	for _, e := range events {
		if e.Kind == EventMoved {
			order = append(order, e.Actor)
		}
	}
	if !reflect.DeepEqual(order, []EntityID{4, 9}) {
		t.Fatalf("порядок NPC: %v, ожидается [4 9]", order)
	}
	if w.Creature(4).Pos != (Vec2{X: 3, Y: 5}) {
		t.Errorf("гоблин #4 не сходил: %+v", w.Creature(4).Pos)
	}
	if w.Creature(9).Pos != (Vec2{X: 14, Y: 11}) {
		t.Errorf("крыса #9 не сходила: %+v", w.Creature(9).Pos)
	}
}

// TestNPCInvalidActionFallsToWait — сломанное намерение контроллера
// не ломает ход: NPC теряет ход, как в жизни, а не мир.
func TestNPCInvalidActionFallsToWait(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, _ := NewWorld(s, 6)
	w.SetAI(scriptedAI{actions: map[EntityID]Action{
		4: Move(Vec2{X: 5, Y: 0}),   // синтаксически невозможно: длиннее клетки
		6: Move(Vec2{X: -1, Y: -1}), // семантически невозможно: (3,6) занята гоблином #4
	}})
	events, err := w.Step(Wait())
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	waited := 0
	for _, e := range events {
		if e.Kind == EventWaited && e.Actor == 4 {
			waited++
		}
		if e.Kind == EventMoved {
			t.Fatalf("никто не должен был двигаться: %+v", e)
		}
	}
	if waited != 1 {
		t.Errorf("сломанное намерение не превратилось в ожидание: %d", waited)
	}
}

// TestWorldLog — мир хранит свой поток: события разворачиваются по номерам,
// и в replay поток доступен так же, как в живой партии.
func TestWorldLog(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, genesis := NewWorld(s, 7)
	stream := []Event{genesis}
	for i := 0; i < 3; i++ {
		evs, err := w.Step(Wait())
		if err != nil {
			t.Fatalf("Step: %v", err)
		}
		stream = append(stream, evs...)
	}
	if w.Event(1) == nil || w.Event(1).Kind != EventWorldCreated {
		t.Fatal("первое событие потока — не genesis")
	}
	if got := w.Event(Seq(len(stream))); got == nil || got.Kind != EventTurnEnded {
		t.Fatal("последнее событие потока — не TurnEnded")
	}
	if w.Event(Seq(len(stream)+1)) != nil {
		t.Error("событие за краем потока должно быть nil")
	}
	rep, err := WorldFromEvents(stream)
	if err != nil {
		t.Fatalf("WorldFromEvents: %v", err)
	}
	if rep.Event(1) == nil || rep.Event(1).Kind != EventWorldCreated {
		t.Fatal("replay потерял доступ к потоку")
	}
	if !reflect.DeepEqual(w.Snapshot(), rep.Snapshot()) {
		t.Error("мир после replay отличается от живого")
	}
}

// TestStepEventCount — каноническая форма хода: событие игрока первым,
// по событию на каждого живого NPC, TurnEnded последним.
func TestStepEventCount(t *testing.T) {
	w := newParty(t)
	events, err := w.Step(Wait())
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	// 12 существ: игрок + 11 NPC + TurnEnded = 13 событий.
	if len(events) != 13 {
		t.Fatalf("событий в ходе: %d, ожидается 13", len(events))
	}
}

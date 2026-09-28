package sim

import (
	"reflect"
	"testing"
)

// TestWitnessesRememberAttack — свидетели удара запоминают его номер:
// память — кольцо, и обида входила в него ровно тогда, когда событие
// происходило на глазах.
func TestWitnessesRememberAttack(t *testing.T) {
	w := newParty(t)
	walkTo(t, w, Vec2{X: 3, Y: 5})
	events, err := w.Step(Attack(Vec2{X: 0, Y: 1}))
	if err != nil {
		t.Fatalf("атака: %v", err)
	}
	attacked, damaged := events[0], events[1]
	witnesses := attacked.Witnesses // [1 4 5 6]: игрок, жертва и двое гоблинов
	if len(witnesses) == 0 {
		t.Fatal("атака прошла без свидетелей")
	}
	for _, id := range witnesses {
		c := w.Creature(id)
		if c == nil {
			continue
		}
		if !containsSeq(c.Memory, attacked.Seq) || !containsSeq(c.Memory, damaged.Seq) {
			t.Errorf("свидетель %d не помнит атаку: %v", id, c.Memory)
		}
	}
	// Чужаки не помнят: у культисов и крыс память чиста.
	for _, id := range []EntityID{2, 9} {
		if c := w.Creature(id); c != nil && len(c.Memory) != 0 {
			t.Errorf("несвидетель %d что-то помнит: %v", id, c.Memory)
		}
	}
	// Бытовые факты (ожидания) память не занимают даже у свидетелей.
	for _, id := range witnesses {
		if c := w.Creature(id); c != nil && len(c.Memory) != 2 {
			t.Errorf("память свидетеля %d засорена: %v", id, c.Memory)
		}
	}
}

// containsSeq проверяет вхождение номера события в кольцо памяти.
func containsSeq(mem []Seq, seq Seq) bool {
	for _, s := range mem {
		if s == seq {
			return true
		}
	}
	return false
}

// TestMemoryRingCap — кольцо вытесняет старейшее: мир, где помнят всё,
// не умеет прощать и не умеет забывать — обе способности нужны историям.
func TestMemoryRingCap(t *testing.T) {
	w := newParty(t)
	c := w.Creature(2)
	for i := 0; i < 40; i++ {
		c.remember(Seq(1000 + i))
	}
	if len(c.Memory) != memoryCap {
		t.Fatalf("размер кольца: %d, ожидается %d", len(c.Memory), memoryCap)
	}
	// Старейшие 8 вытеснены, свежие 32 остались по порядку.
	wantFirst, wantLast := Seq(1008), Seq(1039)
	if c.Memory[0] != wantFirst || c.Memory[memoryCap-1] != wantLast {
		t.Errorf("кольцо: первый %d, последний %d, ожидается %d и %d",
			c.Memory[0], c.Memory[memoryCap-1], wantFirst, wantLast)
	}
}

// TestMemorySurvivesReplay — память восстановленного мира совпадает
// с памятью живого: обиды воспроизводятся из потока без допущений.
func TestMemorySurvivesReplay(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, genesis := NewWorld(s, 17)
	stream := []Event{genesis}
	stream = append(stream, walkTo(t, w, Vec2{X: 3, Y: 5})...)
	evs, err := w.Step(Attack(Vec2{X: 0, Y: 1}))
	if err != nil {
		t.Fatalf("атака: %v", err)
	}
	stream = append(stream, evs...)
	rep, err := WorldFromEvents(stream)
	if err != nil {
		t.Fatalf("WorldFromEvents: %v", err)
	}
	if !reflect.DeepEqual(w.Snapshot(), rep.Snapshot()) {
		t.Fatal("миры расходятся — память не пережила replay")
	}
	if !containsSeq(rep.Creature(4).Memory, evs[0].Seq) {
		t.Error("жертва в replay не помнит удар")
	}
}

// TestCorpseEaterRemembered — поедание трупа памятнее самой смерти:
// свидетели запоминают и ужин, и его блюдо.
func TestCorpseEaterRemembered(t *testing.T) {
	w := newParty(t)
	walkTo(t, w, Vec2{X: 3, Y: 5})
	var corpse ItemID
	var used Event
	for i := 0; i < 3; i++ {
		events, err := w.Step(Attack(Vec2{X: 0, Y: 1}))
		if err != nil {
			t.Fatalf("удар %d: %v", i+1, err)
		}
		for _, e := range events {
			if e.Kind == EventCorpseCreated {
				corpse = e.Item
			}
		}
	}
	// Игрок встаёт на труп, поднимает и ест: свидетели #5 и #6 видят.
	if _, err := w.Step(Move(Vec2{X: 0, Y: 1})); err != nil {
		t.Fatalf("шаг на труп: %v", err)
	}
	if _, err := w.Step(PickUp(corpse)); err != nil {
		t.Fatalf("поднятие: %v", err)
	}
	events, err := w.Step(Use(corpse))
	if err != nil {
		t.Fatalf("еда трупа: %v", err)
	}
	for _, e := range events {
		if e.Kind == EventItemUsed {
			used = e
		}
	}
	if used.Seq == 0 {
		t.Fatal("еда трупа не состоялась")
	}
	for _, id := range used.Witnesses {
		if c := w.Creature(id); c != nil && !containsSeq(c.Memory, used.Seq) {
			t.Errorf("свидетель каннибализма %d не запомнил ужин", id)
		}
	}
	if !containsSeq(w.Creature(1).Memory, used.Seq) {
		t.Error("едок не помнит, что ел")
	}
}

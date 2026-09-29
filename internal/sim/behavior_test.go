package sim

import (
	"reflect"
	"testing"
)

func worldFromRows(t *testing.T, rows []string, seed uint64) *World {
	t.Helper()
	s, err := ParseMap(rows)
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	w, _ := NewWorld(s, seed)
	return w
}

func TestAICombatAndTieBreak(t *testing.T) {
	w := worldFromRows(t, []string{"#####", "#rgr#", "#####"}, 1)
	got := (AI{}).Decide(w, 2)
	want := Attack(Vec2{X: -1})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Decide=%+v, want lower-ID target %+v", got, want)
	}
}

func TestAIChasesVisibleEnemy(t *testing.T) {
	w := worldFromRows(t, []string{"#####", "#g.r#", "#####"}, 1)
	got := (AI{}).Decide(w, 1)
	if got.Kind != ActionMove || got.Dir != (Vec2{X: 1}) {
		t.Fatalf("Decide=%+v, want move east", got)
	}
}

func TestAIEatsAndLoots(t *testing.T) {
	w := worldFromRows(t, []string{"#####", "#g.f#", "#####"}, 1)
	w.turn = hungryAt
	w.items[1].Pos = w.creatures[1].Pos
	if got := (AI{}).Decide(w, 1); got != PickUp(1) {
		t.Fatalf("hungry Decide=%+v, want PickUp(1)", got)
	}
	w.items[1].Holder = 1
	w.items[1].Pos = Vec2{}
	if got := (AI{}).Decide(w, 1); got != Use(1) {
		t.Fatalf("fed Decide=%+v, want Use(1)", got)
	}

	w2 := worldFromRows(t, []string{"#####", "#g.w#", "#####"}, 2)
	w2.items[1].Pos = w2.creatures[1].Pos
	if got := (AI{}).Decide(w2, 1); got != PickUp(1) {
		t.Fatalf("weapon Decide=%+v, want PickUp(1)", got)
	}
}

func TestAIWanderDeterministic(t *testing.T) {
	rows := []string{"#####", "#...#", "#.g.#", "#...#", "#####"}
	a, b := worldFromRows(t, rows, 77), worldFromRows(t, rows, 77)
	if got, want := (AI{}).Decide(a, 1), (AI{}).Decide(b, 1); got != want {
		t.Fatalf("same seed wandered differently: %+v != %+v", got, want)
	}
}

func TestRelationAndFearDecay(t *testing.T) {
	w := worldFromRows(t, []string{"#####", "#ggr#", "#####"}, 1)
	pos := w.creatures[1].Pos
	w.emit(Event{Kind: EventAttacked, Actor: 2, Target: 1, Pos: &pos, Witnesses: []EntityID{1}})
	if got := w.Relation(1, 2); got != -5 {
		t.Fatalf("relation after personal attack=%d, want -5", got)
	}
	if got := w.Fear(1, 2); got != 18 {
		t.Fatalf("fresh fear=%d, want 18", got)
	}
	w.turn = 18
	if got := w.Fear(1, 2); got != 0 {
		t.Fatalf("old fear=%d, want 0", got)
	}
}

func TestAIFleesRememberedThreat(t *testing.T) {
	w := worldFromRows(t, []string{"######", "#....#", "#.rg.#", "#....#", "######"}, 1)
	pos := w.creatures[1].Pos
	w.emit(Event{Kind: EventAttacked, Actor: 2, Target: 1, Pos: &pos, Witnesses: []EntityID{1}})
	got := (AI{}).Decide(w, 1)
	if got.Kind != ActionMove || chebyshev(w.creatures[1].Pos.Add(got.Dir), w.creatures[2].Pos) <= 1 {
		t.Fatalf("fear did not increase distance: %+v", got)
	}
}

func TestCannibalismChangesRelationAndFear(t *testing.T) {
	w := worldFromRows(t, []string{"#####", "#ggg#", "#####"}, 2)
	pos := w.creatures[1].Pos
	w.emit(Event{Kind: EventCorpseCreated, Target: 1, Item: 1, Pos: &pos})
	w.items[1].Holder = 2
	w.emit(Event{Kind: EventItemUsed, Actor: 2, Item: 1, Pos: &pos, Witnesses: []EntityID{3}})
	if got := w.Relation(3, 2); got != -10 {
		t.Fatalf("relation to cannibal=%d, want -10", got)
	}
	if got := w.Fear(3, 2); got != 20 {
		t.Fatalf("fear of cannibal=%d, want 20", got)
	}
}

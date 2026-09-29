package sim

import (
	"reflect"
	"testing"
)

func TestPlayerKnowledgePersistsAndTracksClaims(t *testing.T) {
	w := worldFromRows(t, []string{"#####", "#g.r#", "#####"}, 1)
	k := NewPlayerKnowledge(w.Width(), w.Height())
	openPos := w.creatures[2].Pos
	open := w.emit(Event{Kind: EventClaimOpened, Actor: 2, Target: 1, Pos: &openPos})
	k.ObserveEvents(1, []Event{open})
	k.ObserveVisible([]Vec2{{X: 1, Y: 1}}, w.Snapshot())
	first := k.Snapshot()
	if len(first.Targets) != 1 || first.Targets[0].Pos != openPos || !first.Explored[1*w.Width()+1] {
		t.Fatalf("initial knowledge: %+v", first)
	}

	w.creatures[2].Pos = Vec2{X: 2, Y: 1}
	k.ObserveVisible([]Vec2{{X: 2, Y: 1}}, w.Snapshot())
	if got := k.Snapshot().Targets[0].Pos; got != (Vec2{X: 2, Y: 1}) {
		t.Fatalf("visible target position=%v", got)
	}
	close := w.emit(Event{Kind: EventClaimClosed, Target: 2, Cause: uint8(ClaimClosedByOtherDeath)})
	k.ObserveEvents(1, []Event{close})
	if got := k.Snapshot().Targets; len(got) != 0 {
		t.Fatalf("closed marker remains: %+v", got)
	}
}

func TestKnowledgeSnapshotDoesNotAlias(t *testing.T) {
	k := NewPlayerKnowledge(2, 2)
	k.ObserveEvents(1, []Event{{Seq: 1, Kind: EventWorldCreated}})
	k.ObserveVisible([]Vec2{{}}, Snapshot{})
	s := k.Snapshot()
	s.Explored[0] = false
	s.SeenEvents[0] = 99
	if reflect.DeepEqual(s, k.Snapshot()) {
		t.Fatal("knowledge snapshot aliases live projection")
	}
}

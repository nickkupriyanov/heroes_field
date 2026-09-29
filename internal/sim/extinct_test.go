package sim

import (
	"errors"
	"reflect"
	"testing"
)

// TestExtinction — партия без еды заканчивается вымиранием: после льготного
// периода крысы (HP 4) умирают первыми, культисты (HP 5) — вторыми, гоблины
// (HP 6) — последними; последний ход несёт CampaignEnded{Extinction} до TurnEnded.
func TestExtinction(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, genesis := NewWorld(s, 31)
	w.SetAI(PassiveAI{})
	stream := []Event{genesis}
	var last []Event
	for {
		events, err := w.Step(Wait())
		if err != nil {
			if !errors.Is(err, ErrCampaignOver) {
				t.Fatalf("последняя ошибка не конец партии: %v", err)
			}
			break
		}
		stream = append(stream, events...)
		last = events
	}
	// Последний состоявшийся ход: конец партии — предпоследним событием,
	// TurnEnded — последним (канонический порядок фаз 5 → 6).
	n := len(last)
	if n < 2 || last[n-2].Kind != EventCampaignEnded || EndCause(last[n-2].Cause) != EndExtinction {
		t.Fatalf("нет факта вымирания в конце: %+v", last)
	}
	if last[n-1].Kind != EventTurnEnded {
		t.Fatalf("последним событием хода не TurnEnded: %+v", last[n-1])
	}
	snap := w.Snapshot()
	if !snap.Ended || EndCause(snap.End) != EndExtinction {
		t.Errorf("мир не помнит о своём конце: %+v %+v", snap.Ended, snap.End)
	}
	for _, c := range snap.Creatures {
		if c.Alive {
			t.Errorf("%+#v пережил вымирание", c)
		}
	}

	// Replay всей партии: конец партии восстанавливается из потока.
	rep, err := WorldFromEvents(stream)
	if err != nil {
		t.Fatalf("WorldFromEvents: %v", err)
	}
	if !reflect.DeepEqual(w.Snapshot(), rep.Snapshot()) {
		t.Error("мир после replay вымирания отличается от живого")
	}
}

// TestExtinctionTimeline проверяет точный момент первой голодной смерти.
func TestExtinctionTimeline(t *testing.T) {
	w := newParty(t)
	firstDeath := 0
	for w.Turn() < HungerGrace+10 {
		events, err := w.Step(Wait())
		if err != nil {
			break
		}
		for _, e := range events {
			if e.Kind == EventDied && firstDeath == 0 {
				firstDeath = e.Turn
			}
		}
	}
	if firstDeath == 0 {
		t.Fatalf("за %d ходов никто не умер от голода", HungerGrace+10)
	}
	if firstDeath != HungerGrace+KindMaxHP(CreatureRat) {
		t.Errorf("первая смерть на %d-м ходу, ожидался %d-й", firstDeath, HungerGrace+KindMaxHP(CreatureRat))
	}
}

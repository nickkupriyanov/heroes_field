package sim

import (
	"sort"
	"testing"
)

func TestLongHeadlessInvariants(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 10; seed++ {
		w, _ := NewWorld(s, seed)
		for w.Turn() < 500 {
			if w.NeedsBodyChoice() {
				bodies := w.AvailableBodies()
				if len(bodies) == 0 {
					break
				}
				if _, err := w.ChooseBody(bodies[0]); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if snap := w.Snapshot(); snap.Ended {
				if EndCause(snap.End) == EndVictory {
					if _, err := w.ContinueSandbox(); err != nil {
						t.Fatal(err)
					}
					continue
				}
				break
			}
			a := (AI{}).Decide(w, w.Player())
			if _, err := w.Step(a); err != nil {
				if _, err := w.Step(Wait()); err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
			}
			if err := w.ValidateInvariants(); err != nil {
				t.Fatalf("seed %d turn %d: %v", seed, w.Turn(), err)
			}
		}
	}
}

func TestBalanceAcross100Seeds(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatal(err)
	}
	contacts, deaths, sparse := make([]int, 0, 100), make([]int, 0, 100), make([]int, 0, 100)
	for seed := uint64(1); seed <= 100; seed++ {
		w, _ := NewWorld(s, seed)
		contact, death, belowThree := 501, 501, 501
		for w.Turn() < 500 {
			if w.NeedsBodyChoice() {
				bodies := w.AvailableBodies()
				if len(bodies) == 0 {
					break
				}
				if _, err := w.ChooseBody(bodies[0]); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if snap := w.Snapshot(); snap.Ended {
				if EndCause(snap.End) == EndVictory {
					if _, err := w.ContinueSandbox(); err != nil {
						t.Fatal(err)
					}
					continue
				}
				break
			}
			events, err := w.Step((AI{}).Decide(w, w.Player()))
			if err != nil {
				events, err = w.Step(Wait())
			}
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			for _, e := range events {
				if e.Kind == EventAttacked && contact == 501 {
					contact = e.Turn
				}
				if e.Kind == EventDied && death == 501 {
					death = e.Turn
				}
			}
			alive := 0
			for _, c := range w.Snapshot().Creatures {
				if c.Alive {
					alive++
				}
			}
			if alive < 3 && belowThree == 501 {
				belowThree = w.Turn()
			}
		}
		contacts, deaths, sparse = append(contacts, contact), append(deaths, death), append(sparse, belowThree)
	}
	median := func(values []int) int { sort.Ints(values); return values[len(values)/2] }
	if got := median(contacts); got > 30 {
		t.Errorf("median first contact=%d, want <=30", got)
	}
	if got := median(deaths); got > 60 {
		t.Errorf("median first death=%d, want <=60", got)
	}
	if got := median(sparse); got < 200 {
		t.Errorf("median fewer-than-three turn=%d, want >=200", got)
	}
}

package sim

import (
	"errors"
	"reflect"
	"testing"
)

func TestPlayerDeathClaimBodyChoiceReplay(t *testing.T) {
	s, err := ParseMap([]string{"####", "#gr#", "####"})
	if err != nil {
		t.Fatal(err)
	}
	w, genesis := NewWorld(s, 3)
	pos := w.creatures[1].Pos
	setup := w.emit(Event{Kind: EventDamaged, Target: 1, Amount: w.creatures[1].MaxHP - 1, Cause: uint8(DamageCombat), Pos: &pos})
	w.SetAI(scriptedAI{actions: map[EntityID]Action{2: Attack(Vec2{X: -1})}})
	turn, err := w.Step(Wait())
	if err != nil {
		t.Fatalf("fatal turn: %v", err)
	}
	if !w.NeedsBodyChoice() {
		t.Fatal("dead player did not request body choice")
	}
	if _, err := w.Step(Wait()); !errors.Is(err, ErrBodyChoiceRequired) {
		t.Fatalf("Step while bodiless: %v", err)
	}
	claims := w.Claims()
	if len(claims) != 1 || claims[0].Killer != 2 || claims[0].State != ClaimActive {
		t.Fatalf("claim after death: %+v", claims)
	}
	chosen, err := w.ChooseBody(2)
	if err != nil {
		t.Fatalf("ChooseBody: %v", err)
	}
	if w.Player() != 2 || w.NeedsBodyChoice() {
		t.Fatalf("body choice not applied: player=%d awaiting=%v", w.Player(), w.NeedsBodyChoice())
	}
	if got := w.Claims()[0]; got.State != ClaimMissed || got.CloseReason != ClaimClosedByPossession {
		t.Fatalf("possessed killer did not miss claim: %+v", got)
	}
	stream := append([]Event{genesis, setup}, turn...)
	stream = append(stream, chosen...)
	rep, err := WorldFromEvents(stream)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !reflect.DeepEqual(w.Snapshot(), rep.Snapshot()) {
		t.Fatal("body-choice replay differs")
	}
}

func TestVengeanceFulfilledAwardsLegacy(t *testing.T) {
	w := worldFromRows(t, []string{"####", "#gr#", "####"}, 4)
	w.SetAI(PassiveAI{})
	w.emit(Event{Kind: EventClaimOpened, Actor: 2, Target: 1})
	w.creatures[2].HP = 1
	events, err := w.Step(Attack(Vec2{X: 1}))
	if err != nil {
		t.Fatalf("vengeance attack: %v", err)
	}
	if w.LegacyMarks() != 1 {
		t.Fatalf("legacy=%d, want 1", w.LegacyMarks())
	}
	claim := w.Claims()[0]
	if claim.State != ClaimFulfilled || claim.CloseReason != ClaimClosedByFulfillment {
		t.Fatalf("claim=%+v", claim)
	}
	found := false
	for _, e := range events {
		if e.Kind == EventClaimClosed && e.Amount == 1 && e.Actor == 1 && e.Target == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("fulfillment event missing")
	}
}

func TestVictoryPriorityAndSandbox(t *testing.T) {
	w := worldFromRows(t, []string{"####", "#gg#", "####"}, 5)
	w.SetAI(PassiveAI{})
	w.legacy = LegacyGoal
	w.turn = HungerGrace
	for _, id := range w.order {
		w.creatures[id].HP = 1
		w.creatures[id].LastFedTurn = 0
	}
	events, err := w.Step(Wait())
	if err != nil {
		t.Fatalf("final turn: %v", err)
	}
	var ended Event
	for _, e := range events {
		if e.Kind == EventCampaignEnded {
			ended = e
		}
	}
	if EndCause(ended.Cause) != EndVictory {
		t.Fatalf("end=%d, want victory before extinction", ended.Cause)
	}
	continued, err := w.ContinueSandbox()
	if err != nil {
		t.Fatalf("ContinueSandbox: %v", err)
	}
	if w.ended != 0 || !w.sandbox || continued.Kind != EventCampaignContinued {
		t.Fatalf("sandbox not applied: ended=%d sandbox=%v event=%+v", w.ended, w.sandbox, continued)
	}
}

func TestClaimDeduplicatesActiveKiller(t *testing.T) {
	w := worldFromRows(t, []string{"####", "#gr#", "####"}, 6)
	death := Event{Kind: EventDied, Actor: 2, Target: 1}
	w.resolveVengeance([]Event{death})
	w.resolveVengeance([]Event{death})
	if got := len(w.Claims()); got != 1 {
		t.Fatalf("active claims=%d, want 1", got)
	}
}

func TestAcceptancePossessEatFormerBodyAndAvenge(t *testing.T) {
	w := worldFromRows(t, []string{"#####", "#grg#", "#####"}, 8)
	pos := w.creatures[1].Pos
	w.emit(Event{Kind: EventDamaged, Target: 1, Amount: w.creatures[1].MaxHP - 1, Cause: uint8(DamageCombat), Pos: &pos})
	w.SetAI(scriptedAI{actions: map[EntityID]Action{2: Attack(Vec2{X: -1})}})
	if _, err := w.Step(Wait()); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ChooseBody(3); err != nil {
		t.Fatal(err)
	}
	w.SetAI(PassiveAI{})
	for w.creatures[2].Alive {
		if _, err := w.Step(Attack(Vec2{X: -1})); err != nil {
			t.Fatal(err)
		}
	}
	if w.LegacyMarks() != 1 {
		t.Fatalf("legacy=%d after vengeance", w.LegacyMarks())
	}
	if _, err := w.Step(Move(Vec2{X: -1})); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Step(Move(Vec2{X: -1})); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Step(PickUp(1)); err != nil {
		t.Fatalf("pick former body: %v", err)
	}
	if _, err := w.Step(Use(1)); err != nil {
		t.Fatalf("eat former body: %v", err)
	}
	if !w.Item(1).Consumed || w.Item(1).Owner != 1 {
		t.Fatalf("former body not consumed: %+v", w.Item(1))
	}
}

func TestThreeFulfilledClaimsWinCampaign(t *testing.T) {
	w := worldFromRows(t, []string{"######", "#grrr#", "######"}, 9)
	w.SetAI(PassiveAI{})
	for target := EntityID(2); target <= 4; target++ {
		w.emit(Event{Kind: EventClaimOpened, Actor: target, Target: 1})
		w.creatures[target].HP = 1
		if _, err := w.Step(Attack(Vec2{X: 1})); err != nil {
			t.Fatalf("target %d: %v", target, err)
		}
		if target < 4 {
			if _, err := w.Step(Move(Vec2{X: 1})); err != nil {
				t.Fatalf("advance after %d: %v", target, err)
			}
		}
	}
	snap := w.Snapshot()
	if !snap.Ended || EndCause(snap.End) != EndVictory || snap.Legacy != LegacyGoal {
		t.Fatalf("campaign did not win: %+v", snap)
	}
}

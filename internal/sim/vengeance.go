package sim

import "fmt"

const LegacyGoal = 3

// VengeanceClaim is one durable chapter of the campaign history. Claims stay
// in opening order after closing so the ending can recount them.
type VengeanceClaim struct {
	Victim      EntityID         `json:"victim"`
	Killer      EntityID         `json:"killer"`
	State       ClaimState       `json:"state"`
	Opened      Seq              `json:"opened"`
	Closed      Seq              `json:"closed,omitempty"`
	CloseReason ClaimCloseReason `json:"reason,omitempty"`
}

// Claims returns a copy of the campaign's claim history.
func (w *World) Claims() []VengeanceClaim {
	return append([]VengeanceClaim(nil), w.claims...)
}

func (w *World) LegacyMarks() int { return w.legacy }

// NeedsBodyChoice is derived from event-sourced state: an unfinished campaign
// whose current controlled creature is dead is waiting for possession.
func (w *World) NeedsBodyChoice() bool {
	player := w.creatures[w.player]
	return w.ended == 0 && player != nil && !player.Alive
}

func (w *World) AvailableBodies() []EntityID {
	if !w.NeedsBodyChoice() {
		return nil
	}
	var ids []EntityID
	for _, id := range w.order {
		if c := w.creatures[id]; c != nil && c.Alive {
			ids = append(ids, id)
		}
	}
	return ids
}

// ChooseBody possesses an existing living inhabitant without consuming a turn.
func (w *World) ChooseBody(id EntityID) ([]Event, error) {
	if !w.NeedsBodyChoice() {
		return nil, fmt.Errorf("%w: смена тела не требуется", ErrInvalidAction)
	}
	if c := w.creatures[id]; c == nil || !c.Alive {
		return nil, fmt.Errorf("%w: тело %d недоступно", ErrInvalidAction, id)
	}
	old := w.player
	var events []Event
	if w.activeClaim(id) != nil {
		events = append(events, w.emit(Event{
			Kind:   EventClaimClosed,
			Target: id,
			Cause:  uint8(ClaimClosedByPossession),
		}))
	}
	events = append(events, w.emit(Event{Kind: EventBodyChosen, Actor: id, Target: old}))
	return events, nil
}

// ContinueSandbox resumes a victorious world. Extinction has no survivor to
// continue with and cannot enter sandbox.
func (w *World) ContinueSandbox() (Event, error) {
	if w.ended != EndVictory {
		return Event{}, fmt.Errorf("%w: sandbox доступен только после победы", ErrInvalidAction)
	}
	return w.emit(Event{Kind: EventCampaignContinued}), nil
}

func (w *World) activeClaim(killer EntityID) *VengeanceClaim {
	for i := len(w.claims) - 1; i >= 0; i-- {
		if w.claims[i].Killer == killer && w.claims[i].State == ClaimActive {
			return &w.claims[i]
		}
	}
	return nil
}

// resolveVengeance processes deaths in their canonical event order.
func (w *World) resolveVengeance(turnEvents []Event) []Event {
	var deaths []Event
	for _, e := range turnEvents {
		if e.Kind == EventDied {
			deaths = append(deaths, e)
		}
	}
	var out []Event
	for _, death := range deaths {
		if death.Target == w.player && death.Actor != 0 && w.activeClaim(death.Actor) == nil {
			killer := w.creatures[death.Actor]
			if killer != nil {
				pos := killer.Pos
				out = append(out, w.emit(Event{
					Kind: EventClaimOpened, Actor: death.Actor, Target: death.Target, Pos: &pos,
				}))
			}
		}
		claim := w.activeClaim(death.Target)
		if claim == nil {
			continue
		}
		reason := ClaimClosedByOtherDeath
		actor, amount := EntityID(0), 0
		if death.Actor == w.player {
			reason, actor, amount = ClaimClosedByFulfillment, w.player, 1
		}
		out = append(out, w.emit(Event{
			Kind: EventClaimClosed, Actor: actor, Target: claim.Killer,
			Cause: uint8(reason), Amount: amount,
		}))
	}
	return out
}

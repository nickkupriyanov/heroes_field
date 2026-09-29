package sim

// Relation computes observer's current attitude toward subject from faction
// and remembered events. No relationship state is stored: replay and live
// worlds derive the same answer from the same memory ring.
func (w *World) Relation(observer, subject EntityID) int {
	me, other := w.creatures[observer], w.creatures[subject]
	if me == nil || other == nil {
		return 0
	}
	relation := BaseRelation(me.Kind, other.Kind)
	for _, seq := range me.Memory {
		e := w.Event(seq)
		if e == nil || e.Actor != subject {
			continue
		}
		switch e.Kind {
		case EventAttacked:
			target := w.creatures[e.Target]
			switch {
			case e.Target == observer:
				relation -= 25
			case target != nil && target.Kind == me.Kind:
				relation -= 10
			case target != nil:
				relation += 5
			}
		case EventItemUsed:
			if item := w.items[e.Item]; item != nil && item.Kind == ItemCorpse {
				owner := w.creatures[item.Owner]
				if owner != nil && owner.Kind == me.Kind {
					relation -= 30
				}
			}
		}
	}
	return clamp(relation, -100, 100)
}

// Fear computes a decaying fear of subject from observer's memories.
// Every impulse loses one point per elapsed turn and disappears at zero.
func (w *World) Fear(observer, subject EntityID) int {
	me := w.creatures[observer]
	if me == nil || w.creatures[subject] == nil {
		return 0
	}
	fear := 0
	for _, seq := range me.Memory {
		e := w.Event(seq)
		if e == nil || e.Actor != subject {
			continue
		}
		impulse := 0
		switch e.Kind {
		case EventAttacked:
			if e.Target == observer {
				impulse = 18
			}
		case EventDamaged:
			if e.Target == observer && DamageCause(e.Cause) == DamageCombat {
				impulse = 12
			}
		case EventDied:
			impulse = 24
		case EventItemUsed:
			if item := w.items[e.Item]; item != nil && item.Kind == ItemCorpse {
				owner := w.creatures[item.Owner]
				if owner != nil && owner.Kind == me.Kind {
					impulse = 20
				}
			}
		}
		age := w.turn - e.Turn
		if remaining := impulse - age; remaining > 0 {
			fear += remaining
		}
	}
	return clamp(fear, 0, 100)
}

func fearThreshold(k CreatureKind) int {
	switch k {
	case CreatureRat:
		return 12
	case CreatureGoblin:
		return 20
	default:
		return 24
	}
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

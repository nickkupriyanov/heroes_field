package sim

import "fmt"

// ValidateInvariants checks the safety properties used by long headless runs.
func (w *World) ValidateInvariants() error {
	occupied := make(map[Vec2]EntityID)
	for _, id := range w.order {
		c := w.creatures[id]
		if c == nil {
			return fmt.Errorf("creature %d missing", id)
		}
		if c.Alive {
			if other := occupied[c.Pos]; other != 0 {
				return fmt.Errorf("creatures %d and %d share %v", other, id, c.Pos)
			}
			occupied[c.Pos] = id
		}
	}
	for _, id := range w.itemOrder {
		item := w.items[id]
		if item == nil {
			return fmt.Errorf("item %d missing", id)
		}
		if item.Holder != 0 {
			holder := w.creatures[item.Holder]
			if holder == nil || !holder.Alive {
				return fmt.Errorf("item %d held by absent/dead creature %d", id, item.Holder)
			}
		} else if !item.Consumed && w.CellAt(item.Pos.X, item.Pos.Y) != CellFloor {
			return fmt.Errorf("item %d lost at %v", id, item.Pos)
		}
		if item.Kind == ItemCorpse && w.creatures[item.Owner] == nil {
			return fmt.Errorf("corpse %d references missing owner %d", id, item.Owner)
		}
	}
	for _, claim := range w.claims {
		if w.creatures[claim.Victim] == nil || w.creatures[claim.Killer] == nil {
			return fmt.Errorf("claim references missing creature: %+v", claim)
		}
	}
	return nil
}

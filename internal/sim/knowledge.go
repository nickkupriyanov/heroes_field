package sim

import "sort"

type TargetMarker struct {
	Killer EntityID `json:"killer"`
	Pos    Vec2     `json:"pos"`
}

type PlayerKnowledgeSnapshot struct {
	Explored   []bool         `json:"explored"`
	SeenEvents []Seq          `json:"seen_events"`
	Targets    []TargetMarker `json:"targets"`
}

// PlayerKnowledge is a player-facing projection. It never changes World and
// deliberately survives possession of a new body.
type PlayerKnowledge struct {
	width, height int
	explored      []bool
	seen          []Seq
	targets       map[EntityID]Vec2
}

func NewPlayerKnowledge(width, height int) *PlayerKnowledge {
	return &PlayerKnowledge{
		width: width, height: height,
		explored: make([]bool, width*height),
		targets:  make(map[EntityID]Vec2),
	}
}

// ObserveEvents adds facts visible to the body controlled while the batch was
// produced. Campaign meta-events are always player knowledge.
func (k *PlayerKnowledge) ObserveEvents(body EntityID, events []Event) {
	for _, e := range events {
		seen := e.Kind == EventWorldCreated || e.Kind == EventClaimOpened ||
			e.Kind == EventClaimClosed || e.Kind == EventBodyChosen ||
			e.Kind == EventCampaignEnded || e.Kind == EventCampaignContinued ||
			e.Actor == body || containsEntity(e.Witnesses, body)
		if seen {
			k.seen = append(k.seen, e.Seq)
		}
		switch e.Kind {
		case EventClaimOpened:
			if e.Pos != nil {
				k.targets[e.Actor] = *e.Pos
			}
		case EventClaimClosed:
			delete(k.targets, e.Target)
		}
	}
}

// ObserveVisible persists terrain and refreshes active target positions only
// while the killer is in the current body's FOV.
func (k *PlayerKnowledge) ObserveVisible(cells []Vec2, snap Snapshot) {
	visible := make(map[Vec2]bool, len(cells))
	for _, cell := range cells {
		visible[cell] = true
		if cell.X >= 0 && cell.Y >= 0 && cell.X < k.width && cell.Y < k.height {
			k.explored[cell.Y*k.width+cell.X] = true
		}
	}
	active := make(map[EntityID]bool)
	for _, claim := range snap.Claims {
		if claim.State == ClaimActive {
			active[claim.Killer] = true
		}
	}
	for killer := range k.targets {
		if !active[killer] {
			delete(k.targets, killer)
		}
	}
	for _, creature := range snap.Creatures {
		if active[creature.ID] && creature.Alive && visible[creature.Pos] {
			k.targets[creature.ID] = creature.Pos
		}
	}
}

func (k *PlayerKnowledge) Snapshot() PlayerKnowledgeSnapshot {
	s := PlayerKnowledgeSnapshot{
		Explored:   append([]bool(nil), k.explored...),
		SeenEvents: append([]Seq(nil), k.seen...),
	}
	for killer, pos := range k.targets {
		s.Targets = append(s.Targets, TargetMarker{Killer: killer, Pos: pos})
	}
	sort.Slice(s.Targets, func(i, j int) bool { return s.Targets[i].Killer < s.Targets[j].Killer })
	return s
}

func containsEntity(ids []EntityID, target EntityID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

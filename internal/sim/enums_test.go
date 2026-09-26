package sim

import "testing"

// TestEnumStability фиксирует числовые значения всех enum'ов.
//
// Значения попадают в JSONL и являются частью формата: если кто-то
// переупорядочит константы, старые записи станут читаться с другим смыслом.
// Тест делает такое изменение громким провалом, а не тихой порчей replay.
func TestEnumStability(t *testing.T) {
	cases := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"CellWall", uint64(CellWall), 1},
		{"CellFloor", uint64(CellFloor), 2},
		{"CreatureGoblin", uint64(CreatureGoblin), 1},
		{"CreatureRat", uint64(CreatureRat), 2},
		{"CreatureCultist", uint64(CreatureCultist), 3},
		{"ActionMove", uint64(ActionMove), 1},
		{"ActionWait", uint64(ActionWait), 2},
		{"ActionAttack", uint64(ActionAttack), 3},
		{"ActionPickUp", uint64(ActionPickUp), 4},
		{"ActionDrop", uint64(ActionDrop), 5},
		{"ActionUse", uint64(ActionUse), 6},
		{"EventWorldCreated", uint64(EventWorldCreated), 1},
		{"EventMoved", uint64(EventMoved), 2},
		{"EventWaited", uint64(EventWaited), 3},
		{"EventAttacked", uint64(EventAttacked), 4},
		{"EventDamaged", uint64(EventDamaged), 5},
		{"EventDied", uint64(EventDied), 6},
		{"EventCorpseCreated", uint64(EventCorpseCreated), 7},
		{"EventItemDropped", uint64(EventItemDropped), 8},
		{"EventItemPickedUp", uint64(EventItemPickedUp), 9},
		{"EventItemUsed", uint64(EventItemUsed), 10},
		{"EventClaimOpened", uint64(EventClaimOpened), 11},
		{"EventClaimClosed", uint64(EventClaimClosed), 12},
		{"EventBodyChosen", uint64(EventBodyChosen), 13},
		{"EventCampaignEnded", uint64(EventCampaignEnded), 14},
		{"EventTurnEnded", uint64(EventTurnEnded), 15},
		{"ItemFood", uint64(ItemFood), 1},
		{"ItemWeapon", uint64(ItemWeapon), 2},
		{"ItemCorpse", uint64(ItemCorpse), 3},
		{"DeathCombat", uint64(DeathCombat), 1},
		{"DeathHunger", uint64(DeathHunger), 2},
		{"DamageCombat", uint64(DamageCombat), 1},
		{"DamageHunger", uint64(DamageHunger), 2},
		{"ClaimActive", uint64(ClaimActive), 1},
		{"ClaimFulfilled", uint64(ClaimFulfilled), 2},
		{"ClaimMissed", uint64(ClaimMissed), 3},
		{"ClaimClosedByFulfillment", uint64(ClaimClosedByFulfillment), 1},
		{"ClaimClosedByOtherDeath", uint64(ClaimClosedByOtherDeath), 2},
		{"ClaimClosedByPossession", uint64(ClaimClosedByPossession), 3},
		{"EndVictory", uint64(EndVictory), 1},
		{"EndExtinction", uint64(EndExtinction), 2},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, ожидается %d: изменение значений enum ломает формат JSONL", tc.name, tc.got, tc.want)
		}
	}
}

// TestVec2 проверяет арифметику координат, на которой строятся шаги и оценки AI.
func TestVec2(t *testing.T) {
	a := Vec2{X: 3, Y: 4}
	if got := a.Add(Vec2{X: -2, Y: 1}); got != (Vec2{X: 1, Y: 5}) {
		t.Errorf("Add: получили %v, ожидается {1 5}", got)
	}
	if got := a.Manhattan(Vec2{X: 0, Y: 0}); got != 7 {
		t.Errorf("Manhattan: получили %d, ожидается 7", got)
	}
	if got := a.Manhattan(a); got != 0 {
		t.Errorf("Manhattan к себе: получили %d, ожидается 0", got)
	}
}

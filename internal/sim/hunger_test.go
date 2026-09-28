package sim

import (
	"errors"
	"testing"
)

// TestHungerDerived — голод не хранится, а выводится: ход минус последняя еда.
func TestHungerDerived(t *testing.T) {
	w := newParty(t)
	for i := 0; i < 3; i++ {
		if _, err := w.Step(Wait()); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	}
	if got := w.Turn() - w.Creature(1).LastFedTurn; got != 3 {
		t.Errorf("голод игрока: %d, ожидается 3", got)
	}
	snap := w.Snapshot()
	if snap.Creatures[0].Hunger != 3 {
		t.Errorf("голод в снапшоте: %d, ожидается 3", snap.Creatures[0].Hunger)
	}
}

// TestEatResetsHunger — еда обнуляет голод, труп в том числе: свежий —
// просто ужин, чужой — дурная слава (реакции свидетелей в M4).
func TestEatResetsHunger(t *testing.T) {
	w := newParty(t)
	// Три хода голода, затем еда в (6,2).
	for i := 0; i < 3; i++ {
		if _, err := w.Step(Wait()); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	}
	walkTo(t, w, Vec2{X: 6, Y: 2})
	if _, err := w.Step(PickUp(1)); err != nil {
		t.Fatalf("PickUp(1): %v", err)
	}
	events, err := w.Step(Use(1))
	if err != nil {
		t.Fatalf("Use(1): %v", err)
	}
	if events[0].Kind != EventItemUsed || events[0].Item != 1 {
		t.Fatalf("событие еды: %+v", events[0])
	}
	if got := w.Turn() - w.Creature(1).LastFedTurn; got != 0 {
		t.Errorf("голод после еды: %d, ожидается 0", got)
	}
	if it := w.Item(1); !it.Consumed || it.Holder != 0 {
		t.Errorf("еда не помечена съеденной: %+v", it)
	}

	// Оружие не едят.
	if _, err := w.Step(Use(2)); err == nil {
		t.Error("Use(2) прошёл: оружие не еда")
	}
	// Несъедобное «в руках» — несуществующий предмет.
	if _, err := w.Step(Use(99)); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("Use(99): %v", err)
	}
}

// TestStarvation — истощение: после HungerGrace ходов начинается урон,
// смерть безлична (Actor=0), причина — голод. Свидетели видят агонию.
func TestStarvation(t *testing.T) {
	w := newParty(t)
	var sawDamage, sawDeath bool
	for w.Turn() <= HungerGrace+8 {
		events, err := w.Step(Wait())
		if err != nil {
			break // тело игрока умерло от голода — это и есть проверка
		}
		for _, e := range events {
			if e.Kind == EventDamaged && DamageCause(e.Cause) == DamageHunger {
				sawDamage = true
				if e.Actor != 0 {
					t.Errorf("истощение приписано актёру %d", e.Actor)
				}
			}
			if e.Kind == EventDied && DeathCause(e.Cause) == DeathHunger {
				sawDeath = true
				if e.Actor != 0 {
					t.Errorf("голодная смерть приписана актёру %d", e.Actor)
				}
				if len(e.Witnesses) == 0 {
					t.Errorf("голодная смерть %d прошла без свидетелей", e.Target)
				}
			}
		}
	}
	if !sawDamage {
		t.Error("истощение так и не началось")
	}
	if !sawDeath {
		t.Error("никто не умер от голода")
	}
	// Крысы (HP 4) умирают раньше гоблинов (HP 6): первый голодный труп — крыса.
	snap := w.Snapshot()
	if snap.Creatures[8].Alive { // крыса #9: первые из крыс по порядку ID
		t.Error("крыса пережила истощение гоблинов")
	}
}

// TestEatCorpse — игрок убивает, поднимает и съедает труп: предмет
// существует для памяти, но исчезает с карты.
func TestEatCorpse(t *testing.T) {
	w := newParty(t)
	walkTo(t, w, Vec2{X: 3, Y: 5})
	var corpse ItemID
	for i := 0; i < 3; i++ {
		events, err := w.Step(Attack(Vec2{X: 0, Y: 1}))
		if err != nil {
			t.Fatalf("удар %d: %v", i+1, err)
		}
		for _, e := range events {
			if e.Kind == EventCorpseCreated {
				corpse = e.Item
			}
		}
	}
	if corpse == 0 {
		t.Fatal("труп не найден в событиях")
	}
	// Встать на труп, поднять, съесть.
	if _, err := w.Step(Move(Vec2{X: 0, Y: 1})); err != nil {
		t.Fatalf("шаг на труп: %v", err)
	}
	if _, err := w.Step(PickUp(corpse)); err != nil {
		t.Fatalf("поднятие трупа: %v", err)
	}
	events, err := w.Step(Use(corpse))
	if err != nil {
		t.Fatalf("еда трупа: %v", err)
	}
	if events[0].Kind != EventItemUsed {
		t.Fatalf("событие еды трупа: %+v", events[0])
	}
	if it := w.Item(corpse); it == nil || !it.Consumed || it.Owner != 4 {
		t.Fatalf("труп исчез из реестра или потерял жертву: %+v", it)
	}
}

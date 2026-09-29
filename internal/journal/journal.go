// Пакет journal — проекция потока событий в короткие русские фразы.
//
// Журнал ничего не знает о графике и не изменяет мир: событие приходит,
// фраза уходит. Память существ и текст журнала — две проекции одного
// потока (DESIGN §5), поэтому единственный источник фраз — сами события
// со свидетелями и участниками, а не досочинённое состояние.
package journal

import (
	"fmt"

	"afterhero/internal/sim"
)

// KindName — русское имя вида существа (именительный).
func KindName(k sim.CreatureKind) string {
	switch k {
	case sim.CreatureGoblin:
		return "гоблин"
	case sim.CreatureRat:
		return "крыса"
	default:
		return "культис"
	}
}

// kindAcc — винительный падеж: «бьёт кого» — гоблина, крысу, культиса.
func kindAcc(k sim.CreatureKind) string {
	switch k {
	case sim.CreatureGoblin:
		return "гоблина"
	case sim.CreatureRat:
		return "крысу"
	default:
		return "культиса"
	}
}

// kindGen — родительный падеж: «труп кого» — гоблина, крысы, культиса.
func kindGen(k sim.CreatureKind) string {
	switch k {
	case sim.CreatureGoblin:
		return "гоблина"
	case sim.CreatureRat:
		return "крысы"
	default:
		return "культиса"
	}
}

// CreatureName — имя существа вида «гоблин #3» (именительный). Имя вида
// и номер — постоянная пара: по ней история читается насквозь, а труп того
// же гоблина остаётся тем же №3.
func CreatureName(id sim.EntityID, snap sim.Snapshot) string {
	return creatureNameWith(id, snap, KindName)
}

// creatureNameWith собирает имя в нужном падеже.
func creatureNameWith(id sim.EntityID, snap sim.Snapshot, kindOf func(sim.CreatureKind) string) string {
	for _, c := range snap.Creatures {
		if c.ID == id {
			return fmt.Sprintf("%s #%d", kindOf(c.Kind), id)
		}
	}
	return fmt.Sprintf("#%d", id)
}

// dirWord — направление словом: для фраз вида «шагнул на восток».
func dirWord(d *sim.Vec2) string {
	switch {
	case d == nil:
		return "куда-то"
	case d.X < 0 && d.Y < 0:
		return "на северо-запад"
	case d.X == 0 && d.Y < 0:
		return "на север"
	case d.X > 0 && d.Y < 0:
		return "на северо-восток"
	case d.X < 0 && d.Y == 0:
		return "на запад"
	case d.X > 0 && d.Y == 0:
		return "на восток"
	case d.X < 0 && d.Y > 0:
		return "на юго-запад"
	case d.X == 0 && d.Y > 0:
		return "на юг"
	case d.X > 0 && d.Y > 0:
		return "на юго-восток"
	default:
		return "куда-то"
	}
}

// ItemName — имя предмета. Труп зовётся по жертве: «труп гоблина #4» —
// потому что читателя волнует не предмет, а того, кем он был.
func ItemName(id sim.ItemID, snap sim.Snapshot) string {
	for _, it := range snap.Items {
		if it.ID != id {
			continue
		}
		switch it.Kind {
		case sim.ItemFood:
			return "еда"
		case sim.ItemWeapon:
			return "оружие"
		case sim.ItemCorpse:
			// Труп зовётся по жертве в родительном: «труп гоблина #4».
			return "труп " + creatureNameWith(it.Owner, snap, kindGen)
		}
	}
	return fmt.Sprintf("предмет #%d", id)
}

// Phrase превращает событие в одну короткую фразу. Пустая строка —
// событие не для печати (например, TurnEnded: номер хода и так на экране).
func Phrase(e sim.Event, snap sim.Snapshot) string {
	switch e.Kind {
	case sim.EventWorldCreated:
		return "Мир проснулся. Кто-то в нём не доживёт до утра."
	case sim.EventMoved:
		return fmt.Sprintf("%s двигается %s", CreatureName(e.Actor, snap), dirWord(e.Dir))
	case sim.EventWaited:
		return fmt.Sprintf("%s ждёт и слушает темноту", CreatureName(e.Actor, snap))
	case sim.EventAttacked:
		return fmt.Sprintf("%s бьёт %s", CreatureName(e.Actor, snap), creatureNameWith(e.Target, snap, kindAcc))
	case sim.EventDamaged:
		if sim.DamageCause(e.Cause) == sim.DamageHunger {
			return fmt.Sprintf("%s истощён: −%d", CreatureName(e.Target, snap), e.Amount)
		}
		return fmt.Sprintf("%s теряет %d от удара", CreatureName(e.Target, snap), e.Amount)
	case sim.EventDied:
		if sim.DeathCause(e.Cause) == sim.DeathHunger {
			return fmt.Sprintf("%s умирает от голода", CreatureName(e.Target, snap))
		}
		if e.Actor == 0 {
			return fmt.Sprintf("%s погибает", CreatureName(e.Target, snap))
		}
		return fmt.Sprintf("%s убивает %s", CreatureName(e.Actor, snap), creatureNameWith(e.Target, snap, kindAcc))
	case sim.EventCorpseCreated:
		return fmt.Sprintf("от %s остаётся труп", creatureNameWith(e.Target, snap, kindGen))
	case sim.EventItemDropped:
		return fmt.Sprintf("%s роняет %s", CreatureName(e.Actor, snap), ItemName(e.Item, snap))
	case sim.EventItemPickedUp:
		return fmt.Sprintf("%s подбирает %s", CreatureName(e.Actor, snap), ItemName(e.Item, snap))
	case sim.EventItemUsed:
		return fmt.Sprintf("%s съедает %s", CreatureName(e.Actor, snap), ItemName(e.Item, snap))
	case sim.EventClaimOpened:
		return fmt.Sprintf("Цель мести: %s", CreatureName(e.Actor, snap))
	case sim.EventClaimClosed:
		if sim.ClaimCloseReason(e.Cause) == sim.ClaimClosedByFulfillment {
			return fmt.Sprintf("Месть исполнена: %s", CreatureName(e.Target, snap))
		}
		return fmt.Sprintf("Месть упущена: %s", CreatureName(e.Target, snap))
	case sim.EventBodyChosen:
		return fmt.Sprintf("Теперь ты — %s", CreatureName(e.Actor, snap))
	case sim.EventCampaignEnded:
		if sim.EndCause(e.Cause) == sim.EndVictory {
			return "Три мести исполнены. Наследие завоёвано."
		}
		return "Мир вымер."
	case sim.EventCampaignContinued:
		return "Победа осталась позади; мир живёт дальше."
	default:
		return ""
	}
}

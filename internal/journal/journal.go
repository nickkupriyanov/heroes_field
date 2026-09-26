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

// KindName — русское имя вида существа.
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

// CreatureName — имя вида «гоблин #3». Имя вида и номер — постоянная пара:
// по ней история читается насквозь, а труп того же гоблина остаётся
// тем же №3.
func CreatureName(id sim.EntityID, snap sim.Snapshot) string {
	for _, c := range snap.Creatures {
		if c.ID == id {
			return fmt.Sprintf("%s #%d", KindName(c.Kind), id)
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
	default:
		return ""
	}
}

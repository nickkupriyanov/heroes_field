package sim

import (
	"fmt"
	"math/rand/v2"
	"sort"
)

// applyEvent применяет свершившийся факт к состоянию мира.
//
// Это reducer всей игры: replay строит мир, вызывая applyEvent для каждой
// строки JSONL, — без AI и без RNG. Изменять World можно только здесь,
// поэтому любое новое событие обязано научиться применяться в этой
// функции одновременно с появлением в emit-логике.
func (w *World) applyEvent(e Event) {
	// Счётчик событий восстанавливается из самого потока, а не хранится
	// отдельно: replay обязан прийти к тому же nextSeq, что и живой мир.
	w.nextSeq = e.Seq + 1
	// Мир хранит свой поток целиком: живая партия и replay пишут здесь
	// одинаково, поэтому память и отношения выводятся в обоих мирах.
	w.log = append(w.log, e)
	switch e.Kind {
	case EventWorldCreated:
		w.applyGenesis(e.Genesis)
	case EventMoved:
		// Позиция — доверие событию: emitter уже проверил занятость.
		w.creatures[e.Actor].Pos = *e.Pos
	case EventWaited:
		// Факт «время потрачено»: состояние не меняет, но журналу нужен.
	case EventTurnEnded:
		// Номер хода восстанавливается из потока: TurnEnded несёт номер
		// завершившегося хода. Для живого мира это подтверждение,
		// для replay — единственный источник счётчика.
		w.turn = e.Turn
	case EventItemPickedUp:
		// Инвентарь — вывод из Holder: меняется только место предмета.
		if it := w.items[e.Item]; it != nil {
			it.Holder = e.Actor
			it.Pos = Vec2{}
		}
	case EventItemDropped:
		// Предмет покинул руки (или выпал из мёртвых) и лёг в клетку события.
		if it := w.items[e.Item]; it != nil {
			it.Holder = 0
			it.Pos = *e.Pos
		}
	case EventAttacked:
		// Сам замах состояния не меняет: урон списывает Damaged.
		// Факт нужен журналу («кто на кого поднял руку») и памяти.
	case EventDamaged:
		if c := w.creatures[e.Target]; c != nil && c.HP > 0 {
			c.HP -= e.Amount
			if c.HP < 0 {
				c.HP = 0
			}
		}
	case EventDied:
		if c := w.creatures[e.Target]; c != nil {
			c.Alive = false
		}
	case EventCorpseCreated:
		// Труп регистрируется как предмет: ID погибшего живёт в Owner,
		// поэтому «он ел труп гоблина #4» разрешается в имя жертвы.
		w.items[e.Item] = &Item{
			ID:    e.Item,
			Kind:  ItemCorpse,
			Pos:   *e.Pos,
			Owner: e.Target,
		}
		w.itemOrder = append(w.itemOrder, e.Item)
		if e.Item >= w.nextItem {
			w.nextItem = e.Item + 1 // счётчик предметов восстанавливается из потока
		}
	case EventItemUsed:
		// Съеденное остаётся в реестре как consumed: память и журнал
		// продолжают звать его по имени («он съел труп гоблина #4»),
		// а карта больше его не рисует и инвентарь не держит.
		if it := w.items[e.Item]; it != nil {
			it.Consumed = true
			it.Holder = 0
			it.Pos = *e.Pos
		}
		if c := w.creatures[e.Actor]; c != nil {
			c.LastFedTurn = e.Turn // голод — производная от этого поля
		}
	case EventCampaignEnded:
		// Конец партии — тоже состояние из потока: replay обязан знать,
		// что ходов больше не будет.
		w.ended = EndCause(e.Cause)
	default:
		// Неизвестный вид события — это баг формата, а не штатная ситуация.
		// Тихий пропуск молча бы рассинхронизировал миры.
		panic(fmt.Sprintf("sim: apply не умеет применять событие вида %d", e.Kind))
	}
}

// applyGenesis восстанавливает полный мир из payload WorldCreated.
// Новая партия и replay проходят через одну функцию — расхождение
// начальных состояний невозможно по построению.
func (w *World) applyGenesis(g *Genesis) {
	if g == nil {
		panic("sim: WorldCreated без payload")
	}
	w.width = g.Width
	w.height = g.Height
	w.cells = make([]CellKind, len(g.Cells))
	for i, c := range g.Cells {
		w.cells[i] = CellKind(c)
	}
	w.turn = 0
	w.seed = g.Seed
	// RNG воспроизводится из seed. Мир, восстановленный replay'ем, пригоден
	// для проверки и показа, но не для точного продолжения партии: позиция
	// генератора в события не входит. Точное сохранение — за рамками MVP.
	w.rng = rand.New(rand.NewPCG(g.Seed, g.Seed))

	w.creatures = make(map[EntityID]*Creature, len(g.Creatures))
	w.items = make(map[ItemID]*Item, len(g.Items))
	w.order = make([]EntityID, 0, len(g.Creatures))
	w.itemOrder = make([]ItemID, 0, len(g.Items))
	// Жители думают одинаково в любой партии: контроллер по умолчанию — AI.
	w.ai = AI{}
	for _, cg := range g.Creatures {
		w.creatures[cg.ID] = &Creature{
			ID:    cg.ID,
			Kind:  cg.Kind,
			Pos:   cg.Pos,
			HP:    cg.HP,
			MaxHP: cg.MaxHP,
			Alive: true,
		}
		w.order = append(w.order, cg.ID)
	}
	for _, ig := range g.Items {
		w.items[ig.ID] = &Item{
			ID:    ig.ID,
			Kind:  ig.Kind,
			Pos:   ig.Pos,
			Owner: ig.Owner,
		}
		w.itemOrder = append(w.itemOrder, ig.ID)
		if ig.ID >= w.nextItem {
			w.nextItem = ig.ID + 1 // счётчик предметов — тоже вывод из потока
		}
	}
	// Стабильный порядок — по возрастанию ID, независимо от порядка
	// в payload: договор детерминизма не должен зависеть от чужого файла.
	sort.Slice(w.order, func(i, j int) bool { return w.order[i] < w.order[j] })
	sort.Slice(w.itemOrder, func(i, j int) bool { return w.itemOrder[i] < w.itemOrder[j] })

	w.player = g.Player
}

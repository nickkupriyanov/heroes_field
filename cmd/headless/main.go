// Команда headless прогоняет партию без движка и графики: чистая симуляция,
// JSONL на stdout, краткая сводка на stderr.
//
// Зачем: (1) доказательство сборкой, что sim не зависит от Ebitengine;
// (2) материал для replay-тестов; (3) метрики баланса и длинные smoke-прогоны.
//
// Управляющий «игрок» здесь — детерминированный водитель на отдельном
// генераторе (не на rng мира): те же seed и turns всегда дают одинаковый
// прогон, что позволяет сравнивать файлы между запусками.
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"

	"afterhero/internal/sim"
)

// dirs — восемь направлений шага.
var dirs = []sim.Vec2{
	{X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1},
	{X: -1, Y: 0}, {X: 1, Y: 0},
	{X: -1, Y: 1}, {X: 0, Y: 1}, {X: 1, Y: 1},
}

func main() {
	seed := flag.Uint64("seed", 1, "seed партии")
	turns := flag.Int("turns", 100, "сколько ходов сыграть")
	flag.Parse()

	s, err := sim.DefaultScenario()
	if err != nil {
		fail(err)
	}
	w, genesis := sim.NewWorld(s, *seed)
	events := []sim.Event{genesis}

	// Водитель не связан с rng мира: у него свой поток случайности,
	// чтобы решения «игрока» не расходовали энтропию симуляции.
	driver := rand.New(rand.NewPCG(*seed, ^*seed))
	firstContact, firstDeath := 0, 0
	for w.Turn() < *turns {
		if w.NeedsBodyChoice() {
			bodies := w.AvailableBodies()
			if len(bodies) == 0 {
				break
			}
			evs, err := w.ChooseBody(bodies[driver.IntN(len(bodies))])
			if err != nil {
				fail(err)
			}
			events = append(events, evs...)
			continue
		}
		if snap := w.Snapshot(); snap.Ended {
			if sim.EndCause(snap.End) == sim.EndVictory {
				e, err := w.ContinueSandbox()
				if err != nil {
					fail(err)
				}
				events = append(events, e)
				continue
			}
			break
		}
		evs, err := w.Step(driverAction(w, driver))
		if err != nil {
			// Недопустимое намерение не расходует ход: пережидаем.
			evs, err = w.Step(sim.Wait())
			if err != nil {
				fail(err)
			}
		}
		events = append(events, evs...)
		for _, e := range evs {
			if e.Kind == sim.EventAttacked && firstContact == 0 {
				firstContact = e.Turn
			}
			if e.Kind == sim.EventDied && firstDeath == 0 {
				firstDeath = e.Turn
			}
		}
		if err := w.ValidateInvariants(); err != nil {
			fail(err)
		}
	}

	if err := sim.EncodeEvents(os.Stdout, events); err != nil {
		fail(err)
	}
	alive := 0
	for _, c := range w.Snapshot().Creatures {
		if c.Alive {
			alive++
		}
	}
	fmt.Fprintf(os.Stderr, "после героя: seed=%d ходов=%d событий=%d живых=%d контакт=%d смерть=%d\n",
		*seed, w.Turn(), len(events), alive, firstContact, firstDeath)
}

// driverAction выбирает действие водителя: треть хода — ожидание,
// остальное — шаг в случайную сторону. Пока AI нет (M4), этого достаточно,
// чтобы гонять коллизии и конвейер хода.
func driverAction(w *sim.World, r *rand.Rand) sim.Action {
	player := w.Player()
	body := w.Creature(player)
	if body == nil {
		return sim.Wait()
	}
	inv := w.InventoryOf(player)
	for _, itemID := range inv {
		item := w.Item(itemID)
		if body != nil && w.Turn()-body.LastFedTurn >= 6 && (item.Kind == sim.ItemFood || item.Kind == sim.ItemCorpse) {
			return sim.Use(itemID)
		}
	}
	snap := w.Snapshot()
	for _, item := range snap.Items {
		if item.Holder == 0 && !item.Consumed && item.Pos == body.Pos {
			return sim.PickUp(item.ID)
		}
	}
	for _, other := range snap.Creatures {
		if !other.Alive || other.ID == player || w.Relation(player, other.ID) > -10 {
			continue
		}
		dx, dy := other.Pos.X-body.Pos.X, other.Pos.Y-body.Pos.Y
		if dx >= -1 && dx <= 1 && dy >= -1 && dy <= 1 {
			return sim.Attack(sim.Vec2{X: sign(dx), Y: sign(dy)})
		}
	}
	if len(inv) > 1 && r.IntN(20) == 0 {
		return sim.Drop(inv[len(inv)-1])
	}
	if r.IntN(4) == 0 {
		return sim.Wait()
	}
	return sim.Move(dirs[r.IntN(len(dirs))])
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "afterhero-headless:", err)
	os.Exit(1)
}

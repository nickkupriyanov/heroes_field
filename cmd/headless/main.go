// Команда headless прогоняет партию без движка и графики: чистая симуляция,
// JSONL на stdout, краткая сводка на stderr.
//
// Зачем: (1) доказательство сборкой, что sim не зависит от Ebitengine;
// (2) материал для replay-тестов; (3) в M6 — метрики баланса на сотнях seed.
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
	for w.Turn() < *turns {
		evs, err := w.Step(driverAction(driver))
		if err != nil {
			// Недопустимое намерение не расходует ход: пережидаем.
			evs, err = w.Step(sim.Wait())
			if err != nil {
				fail(err)
			}
		}
		events = append(events, evs...)
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
	fmt.Fprintf(os.Stderr, "после героя: seed=%d ходов=%d событий=%d живых=%d\n",
		*seed, w.Turn(), len(events), alive)
}

// driverAction выбирает действие водителя: треть хода — ожидание,
// остальное — шаг в случайную сторону. Пока AI нет (M4), этого достаточно,
// чтобы гонять коллизии и конвейер хода.
func driverAction(r *rand.Rand) sim.Action {
	if r.IntN(3) == 0 {
		return sim.Wait()
	}
	return sim.Move(dirs[r.IntN(len(dirs))])
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "afterhero-headless:", err)
	os.Exit(1)
}

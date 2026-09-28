// Команда afterhero — desktop-сборка: окно Ebitengine и связка чистой
// симуляции с рендером.
//
// Здесь нет игровой логики: только окно, ввод и передача снапшотов
// рендеру. Всё, что можно воспроизвести headless'ом, живёт в internal/sim.
package main

import (
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"afterhero/internal/journal"
	"afterhero/internal/render"
	"afterhero/internal/sim"
)

const (
	windowWidth  = 1280 // окно: карта 1024 + сайдбар 256
	windowHeight = 800  // карта 640 + запас под статус/журнал
	title        = "После героя"
	// journalDepth — сколько последних событий хранит сторона UI
	// для сайдбара: свежие сверху.
	journalDepth = 400
)

// keyDirs — клавиши шага: стрелки, vi-клавиши и нумпад. Все три схемы
// roguelike-каноничны; выбор между ними — дело привычки игрока.
var keyDirs = []struct {
	key ebiten.Key
	dir sim.Vec2
}{
	{ebiten.KeyArrowUp, sim.Vec2{X: 0, Y: -1}},
	{ebiten.KeyArrowDown, sim.Vec2{X: 0, Y: 1}},
	{ebiten.KeyArrowLeft, sim.Vec2{X: -1, Y: 0}},
	{ebiten.KeyArrowRight, sim.Vec2{X: 1, Y: 0}},
	{ebiten.KeyH, sim.Vec2{X: -1, Y: 0}},
	{ebiten.KeyJ, sim.Vec2{X: 0, Y: 1}},
	{ebiten.KeyK, sim.Vec2{X: 0, Y: -1}},
	{ebiten.KeyL, sim.Vec2{X: 1, Y: 0}},
	{ebiten.KeyY, sim.Vec2{X: -1, Y: -1}},
	{ebiten.KeyU, sim.Vec2{X: 1, Y: -1}},
	{ebiten.KeyB, sim.Vec2{X: -1, Y: 1}},
	{ebiten.KeyN, sim.Vec2{X: 1, Y: 1}},
	{ebiten.KeyNumpad7, sim.Vec2{X: -1, Y: -1}},
	{ebiten.KeyNumpad8, sim.Vec2{X: 0, Y: -1}},
	{ebiten.KeyNumpad9, sim.Vec2{X: 1, Y: -1}},
	{ebiten.KeyNumpad4, sim.Vec2{X: -1, Y: 0}},
	{ebiten.KeyNumpad6, sim.Vec2{X: 1, Y: 0}},
	{ebiten.KeyNumpad1, sim.Vec2{X: -1, Y: 1}},
	{ebiten.KeyNumpad2, sim.Vec2{X: 0, Y: 1}},
	{ebiten.KeyNumpad3, sim.Vec2{X: 1, Y: 1}},
}

// Game реализует ebiten.Game: держит мир, рендер и буфер событий для сайдбара.
type Game struct {
	world     *sim.World
	view      *render.Renderer
	events    []sim.Event
	lastError string
}

// Update вызывается каждый кадр: пересчитывает FOV тела и применяет
// нажатия клавиш как намерения. Невалидное намерение не расходует ход —
// сообщение об этом остаётся на панели до следующей попытки.
func (g *Game) Update() error {
	body := g.world.Creature(g.world.Player())
	if body != nil && body.Alive {
		g.view.SetVisible(g.world.VisibleCells(body.Pos, sim.KindSight(body.Kind)))
	}
	if action, ok := g.readAction(); ok {
		events, err := g.world.Step(action)
		if err != nil {
			g.lastError = err.Error()
		} else {
			g.lastError = ""
			g.events = append(g.events, events...)
			if len(g.events) > journalDepth {
				g.events = g.events[len(g.events)-journalDepth:]
			}
		}
	}
	return nil
}

// readAction собирает намерение из нажатий этого кадра.
func (g *Game) readAction() (sim.Action, bool) {
	for _, kd := range keyDirs {
		if inpututil.IsKeyJustPressed(kd.key) {
			return sim.Move(kd.dir), true
		}
	}
	for _, key := range []ebiten.Key{ebiten.KeyPeriod, ebiten.KeySpace, ebiten.KeyNumpad5} {
		if inpututil.IsKeyJustPressed(key) {
			return sim.Wait(), true
		}
	}
	return sim.Action{}, false
}

// Draw отрисовывает кадр: снапшот и сайдбар из последних событий.
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 16, G: 14, B: 18, A: 255})
	snap := g.world.Snapshot()
	g.view.SetSnapshot(snap)
	g.view.SetSidebar(g.buildSidebar(snap))
	g.view.Draw(screen)
}

// buildSidebar собирает панель: состояние тела, свежие фразы журнала
// сверху, ошибка — снизу.
func (g *Game) buildSidebar(snap sim.Snapshot) render.Sidebar {
	body := snapPlayer(snap)
	sb := render.Sidebar{
		Turn:  g.world.Turn(),
		Error: g.lastError,
	}
	if body != nil {
		sb.Body = journal.CreatureName(body.ID, snap)
		sb.HP = body.HP
		sb.MaxHP = body.MaxHP
		sb.Hunger = body.Hunger
	}
	// Свежие события сверху; журнал — это то, что видел текущим телом:
	// мир кишит ожиданиями чужаков, но игрок читает только свою историю.
	// (DESIGN §7: знание игрока строится из событий, увиденных его телами.)
	player := snap.Player
	visibleToPlayer := func(e sim.Event) bool {
		if e.Kind == sim.EventWorldCreated {
			return true // первая строка любой истории
		}
		if e.Actor == player {
			return true
		}
		for _, wid := range e.Witnesses {
			if wid == player {
				return true
			}
		}
		return false
	}
	for i := len(g.events) - 1; i >= 0 && len(sb.Events) < 10; i-- {
		if !visibleToPlayer(g.events[i]) {
			continue
		}
		if phrase := journal.Phrase(g.events[i], snap); phrase != "" {
			sb.Events = append(sb.Events, phrase)
		}
	}
	return sb
}

// snapPlayer достаёт текущее тело игрока из снапшота.
func snapPlayer(snap sim.Snapshot) *sim.CreatureSnapshot {
	for i := range snap.Creatures {
		if snap.Creatures[i].ID == snap.Player {
			return &snap.Creatures[i]
		}
	}
	return nil
}

// Layout фиксирует логическое разрешение: пиксельная графика масштабируется
// от 1280×800, а не растягивается произвольно.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return windowWidth, windowHeight
}

func main() {
	scenario, err := sim.DefaultScenario()
	if err != nil {
		log.Fatalf("сценарий: %v", err)
	}
	world, _ := sim.NewWorld(scenario, 1)
	view, err := render.New(world)
	if err != nil {
		log.Fatalf("рендер: %v", err)
	}
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle(title)
	if err := ebiten.RunGame(&Game{world: world, view: view}); err != nil {
		log.Fatal(err)
	}
}

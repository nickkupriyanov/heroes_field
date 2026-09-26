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

	"afterhero/internal/render"
	"afterhero/internal/sim"
)

const (
	windowWidth  = 1280 // окно: карта 1024 + сайдбар 256
	windowHeight = 800  // карта 640 + запас под статус/журнал
	title        = "После героя"
)

// Game реализует ebiten.Game: держит мир и рендер, связывает ввод с ходами.
type Game struct {
	world *sim.World
	view  *render.Renderer
}

// Update вызывается каждый кадр. Пошаговая логика срабатывает по нажатию
// клавиш (ввод появится в M2.4); FOV тела пересчитывается здесь же —
// это дёшево и держит туман в согласии с последним снапшотом.
func (g *Game) Update() error {
	body := g.world.Creature(g.world.Player())
	if body != nil && body.Alive {
		g.view.SetVisible(g.world.VisibleCells(body.Pos, sim.KindSight(body.Kind)))
	}
	return nil
}

// Draw отрисовывает кадр: снапшот мира через рендер.
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 16, G: 14, B: 18, A: 255})
	g.view.SetSnapshot(g.world.Snapshot())
	g.view.Draw(screen)
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
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle(title)
	if err := ebiten.RunGame(&Game{world: world, view: render.New(world)}); err != nil {
		log.Fatal(err)
	}
}

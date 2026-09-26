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
}

// Update вызывается каждый кадр. Пошаговая логика срабатывает по нажатию
// клавиш (ввод появится в M2.4); пока мир просто живёт в памяти.
func (g *Game) Update() error {
	return nil
}

// Draw отрисовывает кадр: тёмный фон под будущие панели.
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 16, G: 14, B: 18, A: 255})
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
	if err := ebiten.RunGame(&Game{world: world}); err != nil {
		log.Fatal(err)
	}
}

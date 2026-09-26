// Пакет render — единственная графика игры: получает read-only снапшоты
// симуляции и рисует их. Обратной связи нет: рендер не может изменить мир
// ни одним своим методом, поэтому симуляция остаётся воспроизводимой.
package render

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"afterhero/internal/sim"
)

// Геометрия интерфейса: карта 32×20 тайлов по 16 пикселей с масштабом ×2
// занимает 1024×640; правые 256 пикселей — сайдбар состояния.
const (
	tilePix    = 32 // 16 исходных пикселей × масштаб 2
	mapAreaW   = 1024
	sidebarW   = 256
	windowH    = 800
)

// Палитра плейсхолдеров: до оригинального тайлсета (M6) мир рисуется
// цветными прямоугольниками. Палитра подобрана так, чтобы фракции
// читались с первого взгляда: зелёные гоблины, серые крысы, лиловые
// культисты.
var (
	colWall    = color.RGBA{R: 58, G: 58, B: 68, A: 255}
	colFloor   = color.RGBA{R: 28, G: 26, B: 34, A: 255}
	colSidebar = color.RGBA{R: 12, G: 11, B: 16, A: 255}
	colPlayer  = color.RGBA{R: 235, G: 235, B: 235, A: 255}

	kindColor = map[sim.CreatureKind]color.RGBA{
		sim.CreatureGoblin:  {R: 61, G: 139, B: 64, A: 255},
		sim.CreatureRat:     {R: 161, G: 136, B: 127, A: 255},
		sim.CreatureCultist: {R: 142, G: 107, B: 191, A: 255},
	}

	itemColor = map[sim.ItemKind]color.RGBA{
		sim.ItemFood:   {R: 224, G: 164, B: 88, A: 255},
		sim.ItemWeapon: {R: 126, G: 200, B: 227, A: 255},
		sim.ItemCorpse: {R: 138, G: 47, B: 54, A: 255},
	}
)

// Renderer рисует состояние мира из снапшотов.
//
// Терраин берётся из мира один раз при создании: в MVP карта не меняется.
// Всё динамическое — существа, предметы, состояние игрока — приходит
// снапшотом каждый кадр.
type Renderer struct {
	terrain []sim.CellKind
	width   int
	height  int
	snap    sim.Snapshot
}

// New создаёт рендер для мира: фиксирует терраин и геометрию.
func New(w *sim.World) *Renderer {
	return &Renderer{
		terrain: w.Terrain(),
		width:   w.Width(),
		height:  w.Height(),
	}
}

// SetSnapshot обновляет отрисовываемое состояние.
func (r *Renderer) SetSnapshot(s sim.Snapshot) { r.snap = s }

// Draw отрисовывает кадр: сайдбар, терраин, предметы на полу, существ.
// Порядок слоёв важен: предмет и существо могут делить клетку,
// и живое всегда рисуется поверх того, что лежит под ним.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.SubImage(image.Rect(mapAreaW, 0, mapAreaW+sidebarW, windowH)).(*ebiten.Image).Fill(colSidebar)
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			c := r.terrain[y*r.width+x]
			col := colFloor
			if c == sim.CellWall {
				col = colWall
			}
			r.drawTile(screen, x, y, col)
		}
	}
	for _, it := range r.snap.Items {
		if it.Holder != 0 {
			continue // в чьём-то инвентаре — на карте не рисуется
		}
		r.drawTile(screen, it.Pos.X, it.Pos.Y, itemColor[it.Kind])
	}
	for _, c := range r.snap.Creatures {
		if !c.Alive {
			continue // мёртвые — предметы-трупы, их рисует слой предметов
		}
		r.drawTile(screen, c.Pos.X, c.Pos.Y, kindColor[c.Kind])
		if c.ID == r.snap.Player {
			// Тело игрока выделяется рамкой: кто «я» сейчас — вопрос
			// жизни и смерти в игре про переселение.
			r.drawHighlight(screen, c.Pos.X, c.Pos.Y)
		}
	}
}

// drawTile заливает клетку тайла цветом.
func (r *Renderer) drawTile(screen *ebiten.Image, x, y int, col color.Color) {
	rect := image.Rect(x*tilePix, y*tilePix, (x+1)*tilePix, (y+1)*tilePix)
	screen.SubImage(rect).(*ebiten.Image).Fill(col)
}

// drawHighlight рисует светлую рамку вокруг клетки текущего тела игрока.
func (r *Renderer) drawHighlight(screen *ebiten.Image, x, y int) {
	px, py := x*tilePix, y*tilePix
	edge := 3
	top := image.Rect(px, py, px+tilePix, py+edge)
	bottom := image.Rect(px, py+tilePix-edge, px+tilePix, py+tilePix)
	left := image.Rect(px, py, px+edge, py+tilePix)
	right := image.Rect(px+tilePix-edge, py, px+tilePix, py+tilePix)
	for _, rect := range []image.Rectangle{top, bottom, left, right} {
		screen.SubImage(rect).(*ebiten.Image).Fill(colPlayer)
	}
}

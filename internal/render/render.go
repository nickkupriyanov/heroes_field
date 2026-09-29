// Пакет render — единственная графика игры: получает read-only снапшоты
// симуляции и рисует их. Обратной связи нет: рендер не может изменить мир
// ни одним своим методом, поэтому симуляция остаётся воспроизводимой.
package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"afterhero/internal/sim"
)

// Геометрия интерфейса: карта 32×20 тайлов по 16 пикселей с масштабом ×2
// занимает 1024×640; правые 256 пикселей — сайдбар состояния.
const (
	tilePix  = 32 // 16 исходных пикселей × масштаб 2
	mapAreaW = 1024
	sidebarW = 256
	windowH  = 800
)

// Палитра UI дополняет встроенный pixel-art atlas.
var (
	colSidebar = color.RGBA{R: 12, G: 11, B: 16, A: 255}
	colPlayer  = color.RGBA{R: 235, G: 235, B: 235, A: 255}
	colUnseen  = color.RGBA{R: 5, G: 4, B: 7, A: 255} // никогда не видел
	colDim     = color.RGBA{R: 0, G: 0, B: 0, A: 150} // исследовано, но вне взгляда

	// Палитра сайдбара: тихие тона — карта рассказывает историю,
	// панель не должна её перебивать.
	colText  = color.RGBA{R: 190, G: 188, B: 196, A: 255}
	colTitle = color.RGBA{R: 235, G: 235, B: 235, A: 255}
	colHP    = color.RGBA{R: 126, G: 200, B: 128, A: 255}
	colErr   = color.RGBA{R: 226, G: 92, B: 92, A: 255}
	colDimTx = color.RGBA{R: 120, G: 118, B: 128, A: 255}
)

//go:embed assets/tiles.png
var tileAtlasPNG []byte

const (
	tileFloor = iota
	tileWall
	tileGoblin
	tileRat
	tileCultist
	tileFood
	tileWeapon
	tileCorpse
	tileTarget
)

// Sidebar — содержимое правой панели: состояние тела, последние события
// и сообщение об ошибке. Рендер не строит этот текст сам — ему приносят.
type Sidebar struct {
	Turn   int
	Body   string
	HP     int
	MaxHP  int
	Hunger int
	Legacy int
	Claims []string
	Mode   string
	Events []string // свежие сверху
	Error  string
}

type Overlay struct {
	Title string
	Lines []string
	Hint  string
}

// Renderer рисует состояние мира из снапшотов.
//
// Терраин берётся из мира один раз при создании: в MVP карта не меняется.
// Всё динамическое — существа, предметы, состояние игрока — приходит
// снапшотом каждый кадр.
//
// Туман ведётся рендером: explored — терраин, который игрок видел любым
// из своих тел (persists через переселения, DESIGN §7), visible — клетки
// в текущем FOV тела, их считает симуляция и передаёт через SetVisible.
type Renderer struct {
	terrain  []sim.CellKind
	width    int
	height   int
	snap     sim.Snapshot
	explored []bool
	visible  map[sim.Vec2]bool
	targets  []sim.TargetMarker
	face     font.Face
	atlas    *ebiten.Image
	sidebar  *Sidebar
	overlay  *Overlay
}

// New создаёт рендер для мира: фиксирует терраин, геометрию и шрифт.
// Шрифт — Go Regular с полной кириллицей: он встроен в бинарник через
// gofont, игра не зависит от системных шрифтов.
func New(w *sim.World) (*Renderer, error) {
	src, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("шрифт Go Regular: %w", err)
	}
	face, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size: 14,
		DPI:  72,
	})
	if err != nil {
		return nil, fmt.Errorf("отрисовка шрифта: %w", err)
	}
	atlasSource, err := png.Decode(bytes.NewReader(tileAtlasPNG))
	if err != nil {
		return nil, fmt.Errorf("атлас тайлов: %w", err)
	}
	width, height := w.Width(), w.Height()
	return &Renderer{
		terrain:  w.Terrain(),
		width:    width,
		height:   height,
		explored: make([]bool, width*height),
		visible:  map[sim.Vec2]bool{},
		face:     face,
		atlas:    ebiten.NewImageFromImage(atlasSource),
	}, nil
}

// SetSnapshot обновляет отрисовываемое состояние.
func (r *Renderer) SetSnapshot(s sim.Snapshot) { r.snap = s }

// SetVisible принимает клетки текущего FOV тела игрока и вплетает их
// в накопленную исследованность. Вызывается каждый кадр; накапливающая
// семантика — вот где живёт «терраин помнит всё».
func (r *Renderer) SetVisible(cells []sim.Vec2) {
	clear(r.visible)
	for _, c := range cells {
		r.visible[c] = true
	}
}

func (r *Renderer) SetKnowledge(k sim.PlayerKnowledgeSnapshot) {
	if len(k.Explored) == len(r.explored) {
		copy(r.explored, k.Explored)
	}
	r.targets = append(r.targets[:0], k.Targets...)
}

// SetSidebar обновляет содержимое правой панели.
func (r *Renderer) SetSidebar(sb Sidebar) { r.sidebar = &sb }

func (r *Renderer) SetOverlay(overlay *Overlay) { r.overlay = overlay }

// Draw отрисовывает кадр: сайдбар, терраин в тумане, предметы на полу,
// существ. Порядок слоёв важен: предмет и существо могут делить клетку,
// и живое всегда рисуется поверх того, что лежит под ним.
//
// Динамические объекты (существа, предметы) видны только в текущем FOV:
// DESIGN §7 — вне взгляда их старые позиции не показываются. Терраин
// остаётся видимым навсегда, лишь темнея вне взгляда.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.SubImage(image.Rect(mapAreaW, 0, mapAreaW+sidebarW, windowH)).(*ebiten.Image).Fill(colSidebar)
	r.drawSidebar(screen)
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			pos := sim.Vec2{X: x, Y: y}
			switch {
			case !r.explored[y*r.width+x]:
				r.drawTile(screen, x, y, colUnseen)
				continue
			case !r.visible[pos]:
				r.drawSprite(screen, x, y, tileFloor)
				r.drawTile(screen, x, y, colDim)
				continue
			}
			tile := tileFloor
			if r.terrain[y*r.width+x] == sim.CellWall {
				tile = tileWall
			}
			r.drawSprite(screen, x, y, tile)
		}
	}
	for _, it := range r.snap.Items {
		if it.Holder != 0 || it.Consumed || !r.visible[it.Pos] {
			continue // в инвентаре, съеден или в тумане — на карте не рисуется
		}
		r.drawSprite(screen, it.Pos.X, it.Pos.Y, itemTile(it.Kind))
	}
	for _, c := range r.snap.Creatures {
		if !c.Alive || !r.visible[c.Pos] {
			continue // мёртвые — трупы-предметы; живые вне FOV невидимы
		}
		r.drawSprite(screen, c.Pos.X, c.Pos.Y, creatureTile(c.Kind))
		if c.ID == r.snap.Player {
			// Тело игрока выделяется рамкой: кто «я» сейчас — вопрос
			// жизни и смерти в игре про переселение.
			r.drawHighlight(screen, c.Pos.X, c.Pos.Y)
		}
	}
	for _, marker := range r.targets {
		r.drawTarget(screen, marker.Pos.X, marker.Pos.Y)
	}
	if r.overlay != nil {
		r.drawOverlay(screen, *r.overlay)
	}
}

// drawSidebar рисует правую панель: ход, тело, здоровье, свежие события
// и подсказку управления. Ошибка последнего шага — красным: игрок обязан
// понимать, почему ход не состоялся.
func (r *Renderer) drawSidebar(screen *ebiten.Image) {
	if r.sidebar == nil {
		return
	}
	x := mapAreaW + 14
	y := 28
	r.text(screen, x, y, fmt.Sprintf("ХОД %d", r.sidebar.Turn), colTitle)
	y += 26
	r.text(screen, x, y, "Ты — "+r.sidebar.Body, colText)
	y += 20
	r.text(screen, x, y, fmt.Sprintf("Здоровье: %d/%d", r.sidebar.HP, r.sidebar.MaxHP), colHP)
	y += 20
	// Голод растёт каждый ход; за порогом начинается истощение.
	hungerCol := colText
	if r.sidebar.Hunger > sim.HungerGrace {
		hungerCol = colErr
	}
	r.text(screen, x, y, fmt.Sprintf("Голод: %d/%d", r.sidebar.Hunger, sim.HungerGrace), hungerCol)
	y += 20
	r.text(screen, x, y, fmt.Sprintf("Наследие: %d/%d", r.sidebar.Legacy, sim.LegacyGoal), colTitle)
	for _, claim := range r.sidebar.Claims {
		y += 18
		r.text(screen, x, y, claim, colErr)
	}
	if r.sidebar.Mode != "" {
		y += 20
		r.text(screen, x, y, r.sidebar.Mode, colTitle)
	}
	y += 28
	r.text(screen, x, y, "— события —", colDimTx)
	y += 22
	for _, line := range r.sidebar.Events {
		text.Draw(screen, line, r.face, x, y, colText)
		y += 18
	}
	if r.sidebar.Error != "" {
		y += 6
		text.Draw(screen, r.sidebar.Error, r.face, x, y, colErr)
	}
	r.text(screen, x, windowH-38, "F — удар · G — поднять", colDimTx)
	r.text(screen, x, windowH-20, "I — инвентарь · . — ждать", colDimTx)
}

func (r *Renderer) drawOverlay(screen *ebiten.Image, overlay Overlay) {
	rect := image.Rect(160, 110, mapAreaW-160, windowH-110)
	panel := screen.SubImage(rect).(*ebiten.Image)
	panel.Fill(color.RGBA{R: 10, G: 9, B: 14, A: 245})
	x, y := 190, 150
	r.text(screen, x, y, overlay.Title, colTitle)
	y += 36
	for _, line := range overlay.Lines {
		r.text(screen, x, y, line, colText)
		y += 22
	}
	if overlay.Hint != "" {
		r.text(screen, x, windowH-145, overlay.Hint, colDimTx)
	}
}

func (r *Renderer) drawTarget(screen *ebiten.Image, x, y int) {
	r.drawSprite(screen, x, y, tileTarget)
}

func (r *Renderer) drawSprite(screen *ebiten.Image, x, y, tile int) {
	sx, sy := (tile%3)*16, (tile/3)*16
	source := r.atlas.SubImage(image.Rect(sx, sy, sx+16, sy+16)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(float64(x*tilePix), float64(y*tilePix))
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(source, op)
}

func creatureTile(kind sim.CreatureKind) int {
	switch kind {
	case sim.CreatureGoblin:
		return tileGoblin
	case sim.CreatureRat:
		return tileRat
	default:
		return tileCultist
	}
}

func itemTile(kind sim.ItemKind) int {
	switch kind {
	case sim.ItemFood:
		return tileFood
	case sim.ItemWeapon:
		return tileWeapon
	default:
		return tileCorpse
	}
}

// text рисует однострочную надпись.
func (r *Renderer) text(screen *ebiten.Image, x, y int, s string, col color.Color) {
	text.Draw(screen, s, r.face, x, y, col)
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

// Command afterhero is the desktop shell around the deterministic simulation.
package main

import (
	"fmt"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"afterhero/internal/journal"
	"afterhero/internal/render"
	"afterhero/internal/sim"
)

const (
	windowWidth  = 1280
	windowHeight = 800
	title        = "После героя"
	journalDepth = 400
)

type inputMode uint8

const (
	modeNormal inputMode = iota
	modeAttack
	modePickup
	modeInventory
	modeBody
	modeEnd
	modeReplay
)

var keyDirs = []struct {
	key ebiten.Key
	dir sim.Vec2
}{
	{ebiten.KeyArrowUp, sim.Vec2{X: 0, Y: -1}}, {ebiten.KeyArrowDown, sim.Vec2{X: 0, Y: 1}},
	{ebiten.KeyArrowLeft, sim.Vec2{X: -1, Y: 0}}, {ebiten.KeyArrowRight, sim.Vec2{X: 1, Y: 0}},
	{ebiten.KeyH, sim.Vec2{X: -1}}, {ebiten.KeyJ, sim.Vec2{Y: 1}}, {ebiten.KeyK, sim.Vec2{Y: -1}}, {ebiten.KeyL, sim.Vec2{X: 1}},
	{ebiten.KeyY, sim.Vec2{X: -1, Y: -1}}, {ebiten.KeyU, sim.Vec2{X: 1, Y: -1}},
	{ebiten.KeyB, sim.Vec2{X: -1, Y: 1}}, {ebiten.KeyN, sim.Vec2{X: 1, Y: 1}},
	{ebiten.KeyNumpad7, sim.Vec2{X: -1, Y: -1}}, {ebiten.KeyNumpad8, sim.Vec2{Y: -1}},
	{ebiten.KeyNumpad9, sim.Vec2{X: 1, Y: -1}}, {ebiten.KeyNumpad4, sim.Vec2{X: -1}},
	{ebiten.KeyNumpad6, sim.Vec2{X: 1}}, {ebiten.KeyNumpad1, sim.Vec2{X: -1, Y: 1}},
	{ebiten.KeyNumpad2, sim.Vec2{Y: 1}}, {ebiten.KeyNumpad3, sim.Vec2{X: 1, Y: 1}},
}

type Game struct {
	world        *sim.World
	view         *render.Renderer
	knowledge    *sim.PlayerKnowledge
	events       []sim.Event
	seed         uint64
	mode         inputMode
	selection    int
	choices      []sim.ItemID
	bodies       []sim.EntityID
	lastError    string
	replayFrames []sim.Snapshot
	replayIndex  int
}

func newGame(seed uint64) (*Game, error) {
	g := &Game{}
	if err := g.reset(seed); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Game) reset(seed uint64) error {
	scenario, err := sim.DefaultScenario()
	if err != nil {
		return err
	}
	world, genesis := sim.NewWorld(scenario, seed)
	view, err := render.New(world)
	if err != nil {
		return err
	}
	g.world, g.view, g.seed = world, view, seed
	g.knowledge = sim.NewPlayerKnowledge(world.Width(), world.Height())
	g.events = []sim.Event{genesis}
	g.knowledge.ObserveEvents(world.Player(), g.events)
	g.mode, g.selection, g.lastError = modeNormal, 0, ""
	g.choices, g.bodies, g.replayFrames = nil, nil, nil
	return nil
}

func (g *Game) Update() error {
	if g.mode != modeReplay {
		g.observeCurrentView()
	}
	if g.mode == modeNormal {
		switch {
		case g.world.NeedsBodyChoice():
			g.bodies, g.selection, g.mode = g.world.AvailableBodies(), 0, modeBody
		case g.world.Snapshot().Ended:
			g.mode = modeEnd
		}
	}
	switch g.mode {
	case modeNormal:
		g.updateNormal()
	case modeAttack:
		g.updateAttack()
	case modePickup:
		g.updatePickup()
	case modeInventory:
		g.updateInventory()
	case modeBody:
		g.updateBodyChoice()
	case modeEnd:
		return g.updateEnd()
	case modeReplay:
		g.updateReplay()
	}
	return nil
}

func (g *Game) observeCurrentView() {
	body := g.world.Creature(g.world.Player())
	if body == nil || !body.Alive {
		g.view.SetVisible(nil)
		g.view.SetKnowledge(g.knowledge.Snapshot())
		return
	}
	cells := g.world.VisibleCells(body.Pos, sim.KindSight(body.Kind))
	g.knowledge.ObserveVisible(cells, g.world.Snapshot())
	g.view.SetVisible(cells)
	g.view.SetKnowledge(g.knowledge.Snapshot())
}

func (g *Game) updateNormal() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.mode = modeAttack
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.choices = g.floorItems()
		switch len(g.choices) {
		case 0:
			g.lastError = "под ногами нечего поднимать"
		case 1:
			g.runAction(sim.PickUp(g.choices[0]))
		default:
			g.selection, g.mode = 0, modePickup
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.choices = g.world.InventoryOf(g.world.Player())
		if len(g.choices) == 0 {
			g.lastError = "инвентарь пуст"
		} else {
			g.selection, g.mode = 0, modeInventory
		}
		return
	}
	if dir, ok := readDirection(); ok {
		g.runAction(sim.Move(dir))
		return
	}
	for _, key := range []ebiten.Key{ebiten.KeyPeriod, ebiten.KeySpace, ebiten.KeyNumpad5} {
		if inpututil.IsKeyJustPressed(key) {
			g.runAction(sim.Wait())
			return
		}
	}
}

func (g *Game) updateAttack() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.mode = modeNormal
		return
	}
	if dir, ok := readDirection(); ok {
		g.mode = modeNormal
		g.runAction(sim.Attack(dir))
	}
}

func (g *Game) updatePickup() {
	if g.updateMenuNavigation() {
		g.mode = modeNormal
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		item := g.choices[g.selection]
		g.mode = modeNormal
		g.runAction(sim.PickUp(item))
	}
}

func (g *Game) updateInventory() {
	if g.updateMenuNavigation() {
		g.mode = modeNormal
		return
	}
	item := g.choices[g.selection]
	if inpututil.IsKeyJustPressed(ebiten.KeyU) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.mode = modeNormal
		g.runAction(sim.Use(item))
	} else if inpututil.IsKeyJustPressed(ebiten.KeyD) {
		g.mode = modeNormal
		g.runAction(sim.Drop(item))
	}
}

func (g *Game) updateBodyChoice() {
	if len(g.bodies) == 0 {
		return
	}
	g.updateMenuNavigation()
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		old := g.world.Player()
		events, err := g.world.ChooseBody(g.bodies[g.selection])
		if err != nil {
			g.lastError = err.Error()
			return
		}
		g.recordEvents(old, events)
		g.mode = modeNormal
	}
}

func (g *Game) updateEnd() error {
	end := sim.EndCause(g.world.Snapshot().End)
	if end == sim.EndVictory && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		body := g.world.Player()
		e, err := g.world.ContinueSandbox()
		if err != nil {
			g.lastError = err.Error()
			return nil
		}
		g.recordEvents(body, []sim.Event{e})
		g.mode = modeNormal
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		return g.reset(g.seed + 1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.buildReplay()
		g.mode = modeReplay
	}
	return nil
}

func (g *Game) updateReplay() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.mode = modeEnd
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) && g.replayIndex > 0 {
		g.replayIndex--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) && g.replayIndex+1 < len(g.replayFrames) {
		g.replayIndex++
	}
}

func (g *Game) updateMenuNavigation() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) && g.selection > 0 {
		g.selection--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		limit := len(g.choices)
		if g.mode == modeBody {
			limit = len(g.bodies)
		}
		if g.selection+1 < limit {
			g.selection++
		}
	}
	return false
}

func (g *Game) runAction(action sim.Action) {
	body := g.world.Player()
	events, err := g.world.Step(action)
	if err != nil {
		g.lastError = err.Error()
		return
	}
	g.lastError = ""
	g.recordEvents(body, events)
}

func (g *Game) recordEvents(body sim.EntityID, events []sim.Event) {
	g.events = append(g.events, events...)
	g.knowledge.ObserveEvents(body, events)
	// MVP campaigns are short; retain the complete canonical stream for replay.
}

func (g *Game) floorItems() []sim.ItemID {
	snap := g.world.Snapshot()
	body := snapPlayer(snap)
	if body == nil {
		return nil
	}
	var ids []sim.ItemID
	for _, item := range snap.Items {
		if item.Holder == 0 && !item.Consumed && item.Pos == body.Pos {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func readDirection() (sim.Vec2, bool) {
	for _, kd := range keyDirs {
		if inpututil.IsKeyJustPressed(kd.key) {
			return kd.dir, true
		}
	}
	return sim.Vec2{}, false
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 16, G: 14, B: 18, A: 255})
	snap := g.world.Snapshot()
	if g.mode == modeReplay && len(g.replayFrames) > 0 {
		snap = g.replayFrames[g.replayIndex]
		g.showWholeMap()
	}
	g.view.SetSnapshot(snap)
	g.view.SetSidebar(g.buildSidebar(snap))
	g.view.SetOverlay(g.buildOverlay(snap))
	g.view.Draw(screen)
}

func (g *Game) buildSidebar(snap sim.Snapshot) render.Sidebar {
	body := snapPlayer(snap)
	sb := render.Sidebar{Turn: snap.Turn, Legacy: snap.Legacy, Error: g.lastError, Mode: g.modeName()}
	if body != nil {
		sb.Body, sb.HP, sb.MaxHP, sb.Hunger = journal.CreatureName(body.ID, snap), body.HP, body.MaxHP, body.Hunger
	}
	knowledge := g.knowledge.Snapshot()
	for _, marker := range knowledge.Targets {
		sb.Claims = append(sb.Claims, fmt.Sprintf("месть: #%d @ %d,%d", marker.Killer, marker.Pos.X, marker.Pos.Y))
	}
	for i := len(knowledge.SeenEvents) - 1; i >= 0 && len(sb.Events) < 10; i-- {
		if e := g.world.Event(knowledge.SeenEvents[i]); e != nil {
			if phrase := journal.Phrase(*e, g.world.Snapshot()); phrase != "" {
				sb.Events = append(sb.Events, phrase)
			}
		}
	}
	return sb
}

func (g *Game) buildOverlay(snap sim.Snapshot) *render.Overlay {
	switch g.mode {
	case modeAttack:
		return &render.Overlay{Title: "АТАКА", Lines: []string{"Выбери направление."}, Hint: "стрелки/vi — удар · Esc — отмена"}
	case modePickup:
		return g.itemOverlay("ПОДНЯТЬ", snap, "Enter — поднять · Esc — отмена")
	case modeInventory:
		return g.itemOverlay("ИНВЕНТАРЬ", snap, "U/Enter — использовать · D — выбросить")
	case modeBody:
		lines := []string{"Текущее тело погибло. Кто продолжит историю?"}
		for i, id := range g.bodies {
			c := creatureSnapshot(snap, id)
			prefix := "  "
			if i == g.selection {
				prefix = "> "
			}
			lines = append(lines, fmt.Sprintf("%s%s  HP %d/%d  голод %d  вещей %d", prefix, journal.CreatureName(id, snap), c.HP, c.MaxHP, c.Hunger, len(c.Inventory)))
		}
		return &render.Overlay{Title: "ВЫБОР НОВОГО ТЕЛА", Lines: lines, Hint: "↑/↓ — выбор · Enter — переселиться"}
	case modeEnd:
		return g.endOverlay(snap)
	case modeReplay:
		return &render.Overlay{Title: fmt.Sprintf("ПОВТОР  %d/%d", g.replayIndex+1, len(g.replayFrames)), Hint: "←/→ — ходы · Esc — итог"}
	}
	return nil
}

func (g *Game) itemOverlay(title string, snap sim.Snapshot, hint string) *render.Overlay {
	lines := make([]string, 0, len(g.choices))
	for i, id := range g.choices {
		prefix := "  "
		if i == g.selection {
			prefix = "> "
		}
		lines = append(lines, prefix+journal.ItemName(id, snap))
	}
	return &render.Overlay{Title: title, Lines: lines, Hint: hint}
}

func (g *Game) endOverlay(snap sim.Snapshot) *render.Overlay {
	fulfilled, missed := 0, 0
	for _, claim := range snap.Claims {
		if claim.State == sim.ClaimFulfilled {
			fulfilled++
		}
		if claim.State == sim.ClaimMissed {
			missed++
		}
	}
	title, hint := "ВЫМИРАНИЕ", "R — повтор · N — новая партия"
	if sim.EndCause(snap.End) == sim.EndVictory {
		title, hint = "НАСЛЕДИЕ ЗАВОЁВАНО", "C — sandbox · R — повтор · N — новая партия"
	}
	lines := []string{fmt.Sprintf("Seed: %d", g.seed), fmt.Sprintf("Ходов: %d", snap.Turn), fmt.Sprintf("Меток наследия: %d", snap.Legacy), fmt.Sprintf("Местей исполнено: %d, упущено: %d", fulfilled, missed)}
	return &render.Overlay{Title: title, Lines: lines, Hint: hint}
}

func (g *Game) buildReplay() {
	g.replayFrames = nil
	for i, e := range g.events {
		if i == 0 || e.Kind == sim.EventTurnEnded {
			if w, err := sim.WorldFromEvents(g.events[:i+1]); err == nil {
				g.replayFrames = append(g.replayFrames, w.Snapshot())
			}
		}
	}
	g.replayIndex = len(g.replayFrames) - 1
	if g.replayIndex < 0 {
		g.replayIndex = 0
	}
}

func (g *Game) showWholeMap() {
	var cells []sim.Vec2
	for y := 0; y < g.world.Height(); y++ {
		for x := 0; x < g.world.Width(); x++ {
			cells = append(cells, sim.Vec2{X: x, Y: y})
		}
	}
	g.view.SetVisible(cells)
	all := make([]bool, g.world.Width()*g.world.Height())
	for i := range all {
		all[i] = true
	}
	g.view.SetKnowledge(sim.PlayerKnowledgeSnapshot{Explored: all})
}

func (g *Game) modeName() string {
	switch g.mode {
	case modeAttack:
		return "режим: атака"
	case modePickup:
		return "режим: подбор"
	case modeInventory:
		return "режим: инвентарь"
	case modeBody:
		return "режим: переселение"
	case modeReplay:
		return "режим: повтор"
	default:
		return ""
	}
}

func snapPlayer(snap sim.Snapshot) *sim.CreatureSnapshot { return creatureSnapshot(snap, snap.Player) }
func creatureSnapshot(snap sim.Snapshot, id sim.EntityID) *sim.CreatureSnapshot {
	for i := range snap.Creatures {
		if snap.Creatures[i].ID == id {
			return &snap.Creatures[i]
		}
	}
	return nil
}
func (g *Game) Layout(_, _ int) (int, int) { return windowWidth, windowHeight }

func main() {
	game, err := newGame(1)
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle(title)
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

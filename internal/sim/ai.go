package sim

// Controller — источник намерений для существа: игрок (ввод) или AI.
//
// Контроллер не может менять мир: он лишь предлагает намерение, а мир
// проверяет и применяет его сам. Так AI и игрок проходят один и тот же
// путь — Action → validate → resolve → Event, — и различаются только
// тем, кто решает.
type Controller interface {
	// Decide возвращает намерение существа id на текущий ход.
	Decide(w *World, id EntityID) Action
}

// PassiveAI — контроллер-наблюдатель: все жители пережидают. Полезен
// тестам (неподвижная сцена для механики хода) и будущим режимам-спектаклям.
type PassiveAI struct{}

// Decide — ожидание: мир живёт, жители смотрят.
func (PassiveAI) Decide(w *World, id EntityID) Action {
	_ = w
	_ = id
	return Wait()
}

// AI — контроллер жителей мира.
//
// Дисциплина восприятия (DESIGN §6): житель знает только собственный FOV
// и собственную память; полное состояние мира — знание разработчика.
// Оценки — целые числа (дробные баллы не добавляют смысла, зато добавляют
// вопросов о стабильности между платформами), тай-брейк фиксирован:
// score по убыванию, затем приоритет поведения по возрастанию, затем
// меньший ID цели. Одинаковый мир, память и позиция RNG дают одинаковое
// решение — всегда.
//
// Поведения: еда, атака/преследование, оружие, блуждание и бегство
// от угроз, выведенных из памяти.
type AI struct{}

// Пороги жизни жителя. Подобраны под темп DESIGN §11: голод 6 при
// истощении после 10 — житель ищет еду, пока ещё есть силы её нести.
const (
	hungryAt  = 6   // с этого голода еда становится целью
	hostileAt = -10 // отношение ниже этого — враг
)

// Приоритеты поведения для тай-брейка: меньше — ближе к инстинкту.
const (
	prioFlee = iota + 1
	prioEat
	prioFight
	prioLoot
	prioWander
)

// candidate — взвешенное намерение: score, приоритет, цель и само действие.
type candidate struct {
	score  int
	prio   int
	target uint32
	action Action
}

// Decide выбирает действие жителя: собирает кандидатов от поведений
// и берёт сильнейшего по цепочке тай-брейка.
func (AI) Decide(w *World, id EntityID) Action {
	c := w.Creature(id)
	if c == nil || !c.Alive {
		return Wait()
	}
	hunger := w.turn - c.LastFedTurn
	var cs []candidate
	add := func(score, prio int, target uint32, a Action) {
		if score > 0 {
			cs = append(cs, candidate{score, prio, target, a})
		}
	}

	// A remembered threat overrides appetite and anger once fear crosses the
	// species threshold. Among visible threats the strongest wins, then ID.
	visible := w.visibleCreatures(id)
	var threat EntityID
	threatFear := 0
	for _, other := range visible {
		fear := w.Fear(id, other)
		if fear > threatFear || fear == threatFear && fear > 0 && other < threat {
			threat, threatFear = other, fear
		}
	}
	if threatFear >= fearThreshold(c.Kind) {
		if d, ok := w.fleeStep(c.Pos, w.creatures[threat].Pos); ok {
			add(100+threatFear, prioFlee, uint32(threat), Move(d))
		}
	}

	// Утоление голода: еда из рук вкуснее (не тратишь ход на путь),
	// под ногами — почти так же, на виду — по мере расстояния.
	if hunger >= hungryAt {
		if food, ok := w.foodInInventory(id); ok {
			add(60+hunger, prioEat, uint32(food), Use(food))
		}
		for _, itemID := range w.visibleFloorItems(id) {
			it := w.items[itemID]
			if it.Kind != ItemFood && it.Kind != ItemCorpse {
				continue
			}
			dist := c.Pos.Manhattan(it.Pos)
			if dist == 0 {
				add(60+hunger, prioEat, uint32(itemID), PickUp(itemID))
			} else if d, ok := w.moveToward(c.Pos, it.Pos); ok {
				add(40+hunger-dist, prioEat, uint32(itemID), Move(d))
			}
		}
	}

	// Враги по фракционной базе и обидам памяти.
	for _, other := range visible {
		enemy := w.creatures[other]
		if w.Relation(id, other) > hostileAt {
			continue
		}
		dist := chebyshev(c.Pos, enemy.Pos)
		if dist <= 1 {
			add(70, prioFight, uint32(other), Attack(directionTo(c.Pos, enemy.Pos)))
		} else if d, ok := w.moveToward(c.Pos, enemy.Pos); ok {
			add(40-dist, prioFight, uint32(other), Move(d))
		}
	}

	// Оружие под ногами, если в руках нет железа.
	if !w.HoldingWeapon(id) {
		for _, itemID := range w.visibleFloorItems(id) {
			if it := w.items[itemID]; it.Kind == ItemWeapon && it.Pos == c.Pos {
				add(30, prioLoot, uint32(itemID), PickUp(itemID))
			}
		}
	}

	// Блуждание: житель без целей всё же двигается — стоя на месте,
	// еды не находят и врагов не встречают.
	if d, ok := w.wanderStep(id); ok {
		add(5, prioWander, 0, Move(d))
	}

	if len(cs) == 0 {
		return Wait()
	}
	best := cs[0]
	for _, cand := range cs[1:] {
		if cand.score > best.score ||
			(cand.score == best.score && (cand.prio < best.prio ||
				(cand.prio == best.prio && cand.target < best.target))) {
			best = cand
		}
	}
	return best.action
}

// fleeStep chooses the first deterministic step that maximizes distance from
// a threat. A sideways step is allowed when walls make increasing distance
// impossible; stepping closer never is.
func (w *World) fleeStep(from, threat Vec2) (Vec2, bool) {
	bestDist := chebyshev(from, threat)
	var best Vec2
	found := false
	for _, d := range neighborDirs {
		n := from.Add(d)
		if w.CellAt(n.X, n.Y) != CellFloor || w.occupiedBy(n) != 0 {
			continue
		}
		dist := chebyshev(n, threat)
		if !found && dist == bestDist || dist > bestDist {
			best, bestDist, found = d, dist, true
		}
	}
	return best, found
}

// BaseRelation — фракционная база: свой вид +20, чужой −30.
func BaseRelation(a, b CreatureKind) int {
	if a == b {
		return 20
	}
	return -30
}

// neighborDirs — восемь направлений в фиксированном порядке: любые
// переборы AI идут только по этому срезу, никогда по map.
var neighborDirs = []Vec2{
	{X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1},
	{X: -1, Y: 0}, {X: 1, Y: 0},
	{X: -1, Y: 1}, {X: 0, Y: 1}, {X: 1, Y: 1},
}

// visibleCreatures — живые существа в FOV жителя, по возрастанию ID.
// Видимость симметрична, так что «я вижу его» здесь равносильно
// «он видит меня» — честная встреча.
func (w *World) visibleCreatures(id EntityID) []EntityID {
	me := w.creatures[id]
	if me == nil {
		return nil
	}
	var ids []EntityID
	for _, other := range w.order {
		if other == id {
			continue
		}
		c := w.creatures[other]
		if c == nil || !c.Alive {
			continue
		}
		if w.visible(me.Pos, c.Pos, KindSight(me.Kind)) {
			ids = append(ids, other)
		}
	}
	return ids
}

// visibleFloorItems — предметы на полу в FOV жителя, по возрастанию ID.
// Съеденное и носимое не видно: его больше нет на земле.
func (w *World) visibleFloorItems(id EntityID) []ItemID {
	me := w.creatures[id]
	if me == nil {
		return nil
	}
	var ids []ItemID
	for _, itemID := range w.itemOrder {
		it := w.items[itemID]
		if it == nil || it.Holder != 0 || it.Consumed {
			continue
		}
		if w.visible(me.Pos, it.Pos, KindSight(me.Kind)) {
			ids = append(ids, itemID)
		}
	}
	return ids
}

// foodInInventory — первая еда или труп в руках (меньший ID):
// завтрак не выбирают, его берут.
func (w *World) foodInInventory(id EntityID) (ItemID, bool) {
	for _, itemID := range w.InventoryOf(id) {
		if k := w.items[itemID].Kind; k == ItemFood || k == ItemCorpse {
			return itemID, true
		}
	}
	return 0, false
}

// moveToward выбирает соседний шаг, максимально сокращающий путь к цели.
// Перебор по neighborDirs с строгим улучшением: детерминизм без вопросов.
// Шагов через стены и спин союзников нет: жадность ограничена географией.
func (w *World) moveToward(from, to Vec2) (Vec2, bool) {
	bestDist := from.Manhattan(to)
	var best Vec2
	found := false
	for _, d := range neighborDirs {
		n := from.Add(d)
		if w.CellAt(n.X, n.Y) != CellFloor || w.occupiedBy(n) != 0 {
			continue
		}
		if dist := n.Manhattan(to); dist < bestDist {
			best, bestDist, found = d, dist, true
		}
	}
	return best, found
}

// wanderStep — случайный допустимый шаг: единственное место, где житель
// доверяется жребию (позиция RNG — часть состояния мира, поэтому
// блуждание воспроизводимо).
func (w *World) wanderStep(id EntityID) (Vec2, bool) {
	c := w.creatures[id]
	if c == nil {
		return Vec2{}, false
	}
	var options []Vec2
	for _, d := range neighborDirs {
		n := c.Pos.Add(d)
		if w.CellAt(n.X, n.Y) == CellFloor && w.occupiedBy(n) == 0 {
			options = append(options, d)
		}
	}
	if len(options) == 0 {
		return Vec2{}, false
	}
	return options[w.rng.IntN(len(options))], true
}

// directionTo — направление шага от from к to (для атаки по соседу).
func directionTo(from, to Vec2) Vec2 {
	d := Vec2{}
	if to.X > from.X {
		d.X = 1
	} else if to.X < from.X {
		d.X = -1
	}
	if to.Y > from.Y {
		d.Y = 1
	} else if to.Y < from.Y {
		d.Y = -1
	}
	return d
}

// chebyshev — расстояние Чебышёва (шагов до клетки, включая диагонали).
func chebyshev(a, b Vec2) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

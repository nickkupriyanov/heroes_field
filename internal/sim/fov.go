package sim

// KindSight — дальность зрения вида в клетках (радиус Чебышёва).
// Крысы близоруки и осторожны, культисты по привычке озираются шире всех,
// гоблины — посередине: это задаёт, кто сколько «заметёт» за один взгляд.
func KindSight(k CreatureKind) int {
	switch k {
	case CreatureGoblin:
		return 6
	case CreatureRat:
		return 5
	default:
		return 7
	}
}

// visible — симметричная видимость между клетками: расстояние Чебышёва
// не больше radius, и обе целочисленные линии (туда и обратно) не проходят
// по стенам. Симметрия обязательна: если свидетель видел преступление,
// то и преступник мог видеть свидетеля — иначе «он на меня напал, а я его
// не видел» стало бы возможным в одном и том же мире.
//
// Стена в конечной точке не мешает: лицо стены видно, заглянуть за неё — нет.
func (w *World) visible(from, to Vec2, radius int) bool {
	dx := from.X - to.X
	if dx < 0 {
		dx = -dx
	}
	dy := from.Y - to.Y
	if dy < 0 {
		dy = -dy
	}
	if dx > radius || dy > radius {
		return false
	}
	return lineClear(w, from, to) && lineClear(w, to, from)
}

// lineClear проводит целочисленную линию Брезенхэма от a до b и сообщает,
// не пересекает ли она стен. Конечные клетки не проверяются: путь «в»
// и «из» клетки — разные вопросы проходимости и видимости.
func lineClear(w *World, a, b Vec2) bool {
	x, y := a.X, a.Y
	dx := b.X - a.X
	dy := b.Y - a.Y
	sx, sy := 1, 1
	if dx < 0 {
		sx = -1
		dx = -dx
	}
	if dy < 0 {
		sy = -1
		dy = -dy
	}
	err := dx - dy
	for {
		if x == b.X && y == b.Y {
			return true // дошли до цели: её собственная стена не блокирует взгляд на неё
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
		if x == b.X && y == b.Y {
			return true
		}
		if w.CellAt(x, y) == CellWall {
			return false
		}
	}
}

// VisibleCells возвращает клетки, видимые из from при радиусе radius.
// Порядок построчный и детерминированный: рендер строит из него туман,
// а будущие проверки «кто что видел» — списки свидетелей.
func (w *World) VisibleCells(from Vec2, radius int) []Vec2 {
	var cells []Vec2
	for y := from.Y - radius; y <= from.Y+radius; y++ {
		for x := from.X - radius; x <= from.X+radius; x++ {
			if x < 0 || y < 0 || x >= w.width || y >= w.height {
				continue
			}
			if w.visible(from, Vec2{X: x, Y: y}, radius) {
				cells = append(cells, Vec2{X: x, Y: y})
			}
		}
	}
	return cells
}

// witnesses собирает живых существ, видящих событие в клетке pos.
// Обход по order (возрастание ID) — событие рождается каноническим.
// Актёр события — всегда среди свидетелей: нельзя сделать что-то
// и не заметить этого.
func (w *World) witnesses(pos Vec2) []EntityID {
	var ids []EntityID
	for _, id := range w.order {
		c := w.creatures[id]
		if c == nil || !c.Alive {
			continue
		}
		if w.visible(c.Pos, pos, KindSight(c.Kind)) {
			ids = append(ids, id)
		}
	}
	return ids
}

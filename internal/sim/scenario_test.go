package sim

import "testing"

// countSpawns считает записи сценария по виду — вспомогательная функция
// тестов, чтобы проверять состав карты компактно.
func countSpawns(s *Scenario, kind CreatureKind) int {
	n := 0
	for _, sp := range s.Spawns {
		if sp.Kind == kind {
			n++
		}
	}
	return n
}

func countItems(s *Scenario, kind ItemKind) int {
	n := 0
	for _, it := range s.Items {
		if it.Kind == kind {
			n++
		}
	}
	return n
}

// TestDefaultScenario проверяет ручную карту: размеры, состав населения
// и предметов, уникальность позиций и замкнутость границ стенами.
// Карта — часть баланса и genesis, поэтому её свойства фиксируются жёстко.
func TestDefaultScenario(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	if s.Width != 32 || s.Height != 20 {
		t.Fatalf("размер карты %d×%d, ожидается 32×20", s.Width, s.Height)
	}
	if got := len(s.Spawns); got != 12 {
		t.Errorf("существ %d, ожидается 12", got)
	}
	for kind, want := range map[CreatureKind]int{
		CreatureGoblin:  4,
		CreatureRat:     4,
		CreatureCultist: 4,
	} {
		if got := countSpawns(s, kind); got != want {
			t.Errorf("существ вида %d: %d, ожидается %d", kind, got, want)
		}
	}
	for kind, want := range map[ItemKind]int{
		ItemFood:   6,
		ItemWeapon: 2,
	} {
		if got := countItems(s, kind); got != want {
			t.Errorf("предметов категории %d: %d, ожидается %d", kind, got, want)
		}
	}

	// Игрок начинает за первого встреченного гоблина — (2,2), кладовая.
	first := s.Spawns[s.PlayerSpawn]
	if first.Kind != CreatureGoblin || first.Pos != (Vec2{X: 2, Y: 2}) {
		t.Errorf("стартовое тело игрока: %+v, ожидается гоблин в (2,2)", first)
	}

	// Никакие два существа не делят клетку — иначе collide-логика хода
	// стартует с уже нарушенным инвариантом.
	seen := map[Vec2]bool{}
	for _, sp := range s.Spawns {
		if seen[sp.Pos] {
			t.Errorf("два существа в клетке %v", sp.Pos)
		}
		seen[sp.Pos] = true
	}

	// Границы замкнуты стенами: изнутри нельзя «выйти» наружу.
	for x := 0; x < s.Width; x++ {
		if s.Cell(x, 0) != CellWall || s.Cell(x, s.Height-1) != CellWall {
			t.Errorf("граница y=0 или y=%d не закрыта при x=%d", s.Height-1, x)
		}
	}
	for y := 0; y < s.Height; y++ {
		if s.Cell(0, y) != CellWall || s.Cell(s.Width-1, y) != CellWall {
			t.Errorf("граница x=0 или x=%d не закрыта при y=%d", s.Width-1, y)
		}
	}
}

// TestScenarioConnectivity обходит карту в ширину от стартового тела
// игрока и требует, чтобы каждое существо и каждый предмет были достижимы.
// Герметичная зона — тише, чем гроб: игра начнётся, но половина мира
// не вступит в историю.
func TestScenarioConnectivity(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	start := s.Spawns[s.PlayerSpawn].Pos
	visited := map[Vec2]bool{start: true}
	queue := []Vec2{start}
	for len(queue) > 0 {
		pos := queue[0]
		queue = queue[1:]
		for _, d := range directions() {
			next := pos.Add(d)
			if !visited[next] && s.Walkable(next) {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	for _, sp := range s.Spawns {
		if !visited[sp.Pos] {
			t.Errorf("существо %+v недостижимо от игрока", sp)
		}
	}
	for _, it := range s.Items {
		if !visited[it.Pos] {
			t.Errorf("предмет %+v недостижим от игрока", it)
		}
	}
}

// TestParseMapErrors проверяет, что кривая карта отвергается с ошибкой,
// а не проходит молча: строгий парсер — часть защиты genesis.
func TestParseMapErrors(t *testing.T) {
	cases := []struct {
		name string
		rows []string
	}{
		{"пустая карта", nil},
		{"карта без столбцов", []string{""}},
		{"непрямоугольная", []string{"###", "#.."}},
		{"неизвестный символ", []string{"###", "#x#", "###"}},
	}
	for _, tc := range cases {
		if _, err := ParseMap(tc.rows); err == nil {
			t.Errorf("%s: карта %v ошибочно принята", tc.name, tc.rows)
		}
	}
	if _, err := ParseMap([]string{"####", "#.g#", "####"}); err != nil {
		t.Errorf("корректная минимальная карта отвергнута: %v", err)
	}
}

// directions — восемь соседних смещений, общий ресурс тестов и AI.
func directions() []Vec2 {
	return []Vec2{
		{X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1},
		{X: -1, Y: 0}, {X: 1, Y: 0},
		{X: -1, Y: 1}, {X: 0, Y: 1}, {X: 1, Y: 1},
	}
}

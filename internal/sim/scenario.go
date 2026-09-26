package sim

import "fmt"

// defaultMap — ручная карта вертикального среза, 32 столбца × 20 строк.
//
// Зоны слева направо и сверху вниз:
//
//   - северо-запад — кладовая гоблинов: запасы еды и четыре гоблина,
//     единственная дверь ведёт в центральный зал (пролом в стене на юге
//     открывается в коридор);
//   - центр — большой зал с редкими колоннами: он соединяет все зоны,
//     поэтому встречи и конфликты зарождаются здесь;
//   - северо-восток за стеной с одной дверью — заброшенное святилище
//     культистов с затёртым оружием;
//   - юг — крысиные тоннели: лабиринт узких проходов, где прячутся крысы.
//
// Символы: '#' — стена, '.' — пол, 'g'/'r'/'c' — гоблин/крыса/культис
// (существо стоит на полу), 'f' — еда, 'w' — оружие.
//
// Игрок начинает за первого встреченного при построчном чтении гоблина —
// это (2,2), сразу у входа в кладовую.
var defaultMap = []string{
	"################################",
	"#.........#..........#.........#",
	"#.g...f...#..........#..c...c..#",
	"#.......f.#....##....#.........#",
	"#............#....#..#.......w.#",
	"#.........#...f......#.........#",
	"#..g....g.#....................#",
	"#...g.....#..........#....c....#",
	"#####.#####..........#..c......#",
	"#..............................#",
	"#####.####............#####.####",
	"#.........#....r...............#",
	"#.........#.f..........##......#",
	"#..............................#",
	"#...r............f.............#",
	"#............##................#",
	"#.......................r......#",
	"#.....r.....................f..#",
	"#.w............................#",
	"################################",
}

// CreatureSpawn — запись сценария о существе: вид и стартовая клетка.
// Состояние (здоровье, память) назначается позже, в NewWorld: сценарий
// отвечает только за то, кто и где стоит в момент WorldCreated.
type CreatureSpawn struct {
	Kind CreatureKind
	Pos  Vec2
}

// ItemSpawn — предмет, лежащий на карте в начале партии.
type ItemSpawn struct {
	Kind ItemKind
	Pos  Vec2
}

// Scenario — разобранная конфигурация исходного мира.
//
// Сценарий — это ещё не мир: нет ID, нет случайности, нет состояния. Он
// детерминированно превращается в мир в NewWorld, который назначает ID
// в порядке чтения карты и эмитит WorldCreated. Такое разделение делает
// genesis-событие полным описанием партии: replay не нужен ни сценарий,
// ни какие-либо внешние данные.
type Scenario struct {
	Width       int
	Height      int
	Cells       []CellKind // row-major: индекс Y*Width+X
	Spawns      []CreatureSpawn
	Items       []ItemSpawn
	PlayerSpawn int // индекс в Spawns: стартовое тело игрока
}

// DefaultScenario возвращает сценарий «После героя» — единственную карту
// вертикального среза. Ошибка возможна только при ошибке в самой карте,
// то есть это самопроверка: кривой defaultMap не запустится в тестах.
func DefaultScenario() (*Scenario, error) {
	return ParseMap(defaultMap)
}

// ParseMap разбирает ASCII-карту в сценарий.
//
// Правила разбора строгие, потому что карта — часть баланса и genesis:
// кривая карта должна падать громко, а не запускать партию с недостижимой
// кладовой. Проверяются прямоугольность и известность каждого символа;
// существа и предметы добавляются в порядке чтения (сверху вниз, слева
// направо) — этот порядок фиксирует назначение ID в NewWorld.
func ParseMap(rows []string) (*Scenario, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("карта пуста")
	}
	width := len(rows[0])
	if width == 0 {
		return nil, fmt.Errorf("карта без столбцов")
	}
	s := &Scenario{
		Width:  width,
		Height: len(rows),
		Cells:  make([]CellKind, 0, width*len(rows)),
	}
	for y, row := range rows {
		if len(row) != width {
			return nil, fmt.Errorf("карта не прямоугольна: строка %d шириной %d, ожидается %d", y, len(row), width)
		}
		for x, ch := range row {
			switch ch {
			case '#':
				s.Cells = append(s.Cells, CellWall)
			case '.':
				s.Cells = append(s.Cells, CellFloor)
			case 'g', 'r', 'c':
				s.Cells = append(s.Cells, CellFloor) // существо стоит на полу
				s.Spawns = append(s.Spawns, CreatureSpawn{Kind: creatureKindOf(ch), Pos: Vec2{X: x, Y: y}})
			case 'f', 'w':
				s.Cells = append(s.Cells, CellFloor) // предмет лежит на полу
				s.Items = append(s.Items, ItemSpawn{Kind: itemKindOf(ch), Pos: Vec2{X: x, Y: y}})
			default:
				return nil, fmt.Errorf("неизвестный символ %q в строке %d, столбце %d", ch, y, x)
			}
		}
	}
	if len(s.Spawns) == 0 {
		return nil, fmt.Errorf("на карте нет существ")
	}
	return s, nil
}

// Cell возвращает тип клетки; клетка вне карты трактуется как стена.
// Договорённость упрощает все проверки границ: движение «в никуда»
// блокируется тем же правилом, что и движение в стену.
func (s *Scenario) Cell(x, y int) CellKind {
	if x < 0 || y < 0 || x >= s.Width || y >= s.Height {
		return CellWall
	}
	return s.Cells[y*s.Width+x]
}

// Walkable сообщает, может ли существо стоять в клетке.
func (s *Scenario) Walkable(pos Vec2) bool {
	return s.Cell(pos.X, pos.Y) == CellFloor
}

// creatureKindOf переводит символ карты в вид существа.
func creatureKindOf(ch rune) CreatureKind {
	switch ch {
	case 'g':
		return CreatureGoblin
	case 'r':
		return CreatureRat
	default:
		return CreatureCultist
	}
}

// itemKindOf переводит символ карты в категорию предмета.
func itemKindOf(ch rune) ItemKind {
	switch ch {
	case 'f':
		return ItemFood
	default:
		return ItemWeapon
	}
}

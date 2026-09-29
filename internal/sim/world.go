package sim

import "math/rand/v2"

// Genesis — payload события WorldCreated: полное исходное состояние партии.
//
// Здесь намеренно нет ни map, ни float, ни чего-либо, что сериализуется
// недетерминированно: genesis — первая строка JSONL, и одинаковые партии
// обязаны начинаться байт-в-байт одинаково. Все массивы упорядочены
// по возрастанию ID (или по чтению карты — что то же самое).
type Genesis struct {
	Seed      uint64            `json:"seed"`
	Width     int               `json:"w"`
	Height    int               `json:"h"`
	Cells     []uint8           `json:"cells"` // значения CellKind, row-major
	Creatures []CreatureGenesis `json:"creatures"`
	Items     []ItemGenesis     `json:"items"`
	Player    EntityID          `json:"player"` // стартовое тело игрока
}

// CreatureGenesis — существо в момент WorldCreated. Память и инвентарь
// не сериализуются: в начале партии они пусты у всех.
type CreatureGenesis struct {
	ID    EntityID     `json:"id"`
	Kind  CreatureKind `json:"kind"`
	Pos   Vec2         `json:"pos"`
	HP    int          `json:"hp"`
	MaxHP int          `json:"maxhp"`
}

// ItemGenesis — предмет, лежащий на карте в начале партии.
// Owner нужен только трупам, которых в genesis нет, но поле остаётся,
// чтобы формат не менялся, когда трупы появятся в партиях.
type ItemGenesis struct {
	ID    ItemID   `json:"id"`
	Kind  ItemKind `json:"kind"`
	Pos   Vec2     `json:"pos"`
	Owner EntityID `json:"owner,omitempty"`
}

// World — пошаговое состояние мира и единственный владелец изменяемых данных.
//
// Все изменения проходят через emit: событие получает канонические поля
// и применяется к состоянию. Случайность — только через rng, созданный из
// seed партии; «время» — только номер хода. Поэтому одинаковый genesis
// и одинаковые действия дают байт-в-байт одинаковый поток событий.
//
// Порядок обработки существ — срез order по возрастанию ID; map creatures
// служит только поиском по ID и никогда не итерируется: порядок обхода map
// в Go не стабилен и стал бы дырой в детерминизме.
type World struct {
	width, height int
	cells         []CellKind

	seed uint64
	rng  *rand.Rand

	turn     int
	nextSeq  Seq
	nextItem ItemID   // следующий свободный ID предмета (трупы рождаются и после genesis)
	ended    EndCause // 0 — партия идёт; победа/вымирание кодируют конец
	sandbox  bool
	legacy   int
	claims   []VengeanceClaim

	log []Event    // полный поток событий: источник памяти и отношений (M4.4)
	ai  Controller // контроллер NPC; не сериализуется: сущность, не состояние

	creatures map[EntityID]*Creature
	items     map[ItemID]*Item
	order     []EntityID
	itemOrder []ItemID

	player EntityID
}

// NewWorld создаёт мир из сценария и seed и возвращает мир вместе
// с genesis-событием WorldCreated.
//
// Мир строится единственным путём — применением собственного genesis-
// события, — поэтому «начало партии» и «мир, восстановленный из JSONL»
// не могут разойтись: это одна и та же функция applyGenesis.
func NewWorld(s *Scenario, seed uint64) (*World, Event) {
	w := &World{
		nextSeq:   1,
		creatures: make(map[EntityID]*Creature),
		items:     make(map[ItemID]*Item),
	}
	g := &Genesis{
		Seed:   seed,
		Width:  s.Width,
		Height: s.Height,
		Cells:  make([]uint8, len(s.Cells)),
	}
	for i, c := range s.Cells {
		g.Cells[i] = uint8(c)
	}
	// ID назначаются в порядке чтения карты: слева направо, сверху вниз.
	// Порядок чтения — часть формата: он задаёт порядок хода NPC
	// (по возрастанию ID) и потому не может быть «просто удобным».
	for _, sp := range s.Spawns {
		hp := KindMaxHP(sp.Kind)
		g.Creatures = append(g.Creatures, CreatureGenesis{
			ID:    EntityID(len(g.Creatures) + 1),
			Kind:  sp.Kind,
			Pos:   sp.Pos,
			HP:    hp,
			MaxHP: hp,
		})
	}
	for _, it := range s.Items {
		g.Items = append(g.Items, ItemGenesis{
			ID:   ItemID(len(g.Items) + 1),
			Kind: it.Kind,
			Pos:  it.Pos,
		})
	}
	g.Player = g.Creatures[s.PlayerSpawn].ID
	ev := w.emit(Event{Kind: EventWorldCreated, Genesis: g})
	return w, ev
}

// emit присваивает событию канонические поля (версия схемы, сквозной номер,
// номер хода), приводит его к канонической форме, применяет к состоянию
// и возвращает готовый факт.
//
// Единственная дверь к изменению мира: тот, кто эмитит событие, описывает
// происшедшее, а apply видит весь поток и держит производные состояния
// (счётчики, память) в согласии с ним.
func (w *World) emit(e Event) Event {
	e.SchemaVersion = SchemaVersion
	e.Seq = w.nextSeq
	e.Turn = w.turn
	e = e.canonicalize()
	w.applyEvent(e)
	return e
}

// SetAI подменяет контроллер NPC: для тестов (кукловодение) и особых
// режимов. Мир не сохраняет контроллер в события — это сущность процесса,
// а не состояния партии.
func (w *World) SetAI(c Controller) { w.ai = c }

// Event возвращает событие по сквозному номеру или nil.
//
// Мир хранит весь свой поток: память существ — это список номеров,
// и без возможности развернуть номер в событие «он видел, как его били»
// оставалась бы списком лотерейных билетов.
func (w *World) Event(seq Seq) *Event {
	if seq == 0 || seq > Seq(len(w.log)) {
		return nil
	}
	return &w.log[seq-1]
}

// Turn возвращает номер текущего хода; 0 — ходов ещё не было.
func (w *World) Turn() int { return w.turn }

// Seed identifies the deterministic campaign genesis.
func (w *World) Seed() uint64 { return w.seed }

// Player возвращает ID текущего тела игрока.
func (w *World) Player() EntityID { return w.player }

// Creature возвращает существо по ID или nil, если такого нет.
func (w *World) Creature(id EntityID) *Creature { return w.creatures[id] }

// Item возвращает предмет по ID или nil, если такого нет.
func (w *World) Item(id ItemID) *Item { return w.items[id] }

// CellAt возвращает тип клетки; вне карты — стена, как и в Scenario.
func (w *World) CellAt(x, y int) CellKind {
	if x < 0 || y < 0 || x >= w.width || y >= w.height {
		return CellWall
	}
	return w.cells[y*w.width+x]
}

// Width и Height — размеры карты в клетках.
func (w *World) Width() int  { return w.width }
func (w *World) Height() int { return w.height }

// Terrain возвращает клетки карты в row-major порядке. Срез неизменен
// в течение партии и передаётся без копии: рендер берёт его один раз
// и не мутирует — копия 640 клеток каждый кадр не стоит нарушенного
// договора. Когда появится разрушаемый терраин, accessor станет копией
// или исчезнет в пользу снапшотов.
func (w *World) Terrain() []CellKind { return w.cells }

// InventoryOf выводит инвентарь существа из Item.Holder — единственного
// источника правды о том, что у кого. Порядок — по возрастанию ID
// предметов: вывод детерминирован, а «что подобрано раньше» историям
// этого MVP пока не рассказывает.
func (w *World) InventoryOf(id EntityID) []ItemID {
	var inv []ItemID
	for _, itemID := range w.itemOrder {
		if it := w.items[itemID]; it != nil && it.Holder == id {
			inv = append(inv, itemID)
		}
	}
	return inv
}

// HoldingWeapon сообщает, есть ли у существа оружие: урон в ближнем бою
// растёт от любого оружия в руках (одна рука, один удар — а гоблин
// не умеет жонглировать).
func (w *World) HoldingWeapon(id EntityID) bool {
	for _, itemID := range w.InventoryOf(id) {
		if w.items[itemID].Kind == ItemWeapon {
			return true
		}
	}
	return false
}

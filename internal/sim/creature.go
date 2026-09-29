package sim

// Creature — текущее состояние существа в мире.
//
// Существо — единственный «персонаж» мира: его здоровье, инвентарь и память
// переживают смерть игрока (в отличие от самого игрока). Память — кольцевой
// буфер ID памятных событий: обиды входят, стареют и вытесняются новыми,
// поэтому у мира нет отдельного хранилища «отношений» — они вычисляются
// из памяти по мере надобности.
//
// Инвентарь намеренно не хранится здесь: единственный источник правды —
// Item.Holder, а инвентарь существа выводится из него (World.InventoryOf).
type Creature struct {
	ID          EntityID
	Kind        CreatureKind
	Pos         Vec2
	HP          int
	MaxHP       int
	LastFedTurn int // ход последней еды; голод = текущий ход минус это число
	Alive       bool
	Memory      []Seq // ID памятных событий, последние в конце, не более memoryCap
}

// memoryCap — вместимость кольца памяти.
const memoryCap = 32

// HungerGrace — сколько ходов существо живёт без еды до начала истощения.
// Бой даёт ранние смерти, а голод не даёт концовке застыть навсегда. Значение
// зафиксировано 100-seed тестом: медианно меньше трёх живых не остаётся раньше
// 200-го хода.
const HungerGrace = 185

func (c *Creature) remember(seq Seq) {
	c.Memory = append(c.Memory, seq)
	if len(c.Memory) > memoryCap {
		c.Memory = c.Memory[len(c.Memory)-memoryCap:]
	}
}

func KindMaxHP(k CreatureKind) int {
	switch k {
	case CreatureGoblin:
		return 6
	case CreatureRat:
		return 4
	default:
		return 5
	}
}

func KindDamage(k CreatureKind) int {
	switch k {
	case CreatureGoblin:
		return 2
	case CreatureRat:
		return 1
	default:
		return 2
	}
}

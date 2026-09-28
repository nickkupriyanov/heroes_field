package sim

// Snapshot — канонический слепок состояния мира.
//
// Две обязанности: строгое сравнение миров в тестах (живой мир против
// восстановленного replay'ем) и read-only представление для рендера.
// Для обеих важна копия насквозь: срезы инвентаря и памяти копируются,
// чтобы слепок не(alias) не мутировал вместе с живым миром.
//
// Терраин намеренно не входит: в MVP карта не меняется, и её копия в каждом
// слепке лишь раздувала бы сравнение. Если появятся разрушаемые стены —
// поле появится вместе с событием разрушения.
type Snapshot struct {
	Turn      int                `json:"turn"`
	NextSeq   Seq                `json:"nextSeq"`
	Player    EntityID           `json:"player"`
	Width     int                `json:"w"`
	Height    int                `json:"h"`
	Ended     bool               `json:"ended,omitempty"` // партия завершена
	End       uint8              `json:"end,omitempty"`   // причина конца (EndCause), когда Ended
	Creatures []CreatureSnapshot `json:"creatures"`       // по возрастанию ID
	Items     []ItemSnapshot     `json:"items"`           // по возрастанию ID
}

// CreatureSnapshot — существо в слепке. Порядок Inventory и Memory
// сохраняется как в мире: он семантический (что подобрано раньше,
// что замечено позже), а не произвольный.
type CreatureSnapshot struct {
	ID        EntityID     `json:"id"`
	Kind      CreatureKind `json:"kind"`
	Pos       Vec2         `json:"pos"`
	HP        int          `json:"hp"`
	MaxHP     int          `json:"maxhp"`
	Hunger    int          `json:"hunger"` // выводится из хода последней еды
	Alive     bool         `json:"alive"`
	Inventory []ItemID     `json:"inv,omitempty"`
	Memory    []Seq        `json:"mem,omitempty"`
}

// ItemSnapshot — предмет в слепке.
type ItemSnapshot struct {
	ID       ItemID   `json:"id"`
	Kind     ItemKind `json:"kind"`
	Holder   EntityID `json:"holder,omitempty"`
	Pos      Vec2     `json:"pos"`
	Owner    EntityID `json:"owner,omitempty"`
	Consumed bool     `json:"consumed,omitempty"`
}

// Snapshot снимает состояние мира: срезы order/itemOrder уже упорядочены
// по возрастанию ID, поэтому слепок каноничен без дополнительной сортировки.
func (w *World) Snapshot() Snapshot {
	s := Snapshot{
		Turn:    w.turn,
		NextSeq: w.nextSeq,
		Player:  w.player,
		Width:   w.width,
		Height:  w.height,
		Ended:   w.ended != 0,
		End:     uint8(w.ended),
	}
	for _, id := range w.order {
		c := w.creatures[id]
		cs := CreatureSnapshot{
			ID:        c.ID,
			Kind:      c.Kind,
			Pos:       c.Pos,
			HP:        c.HP,
			MaxHP:     c.MaxHP,
			Hunger:    w.turn - c.LastFedTurn, // производная, не состояние
			Alive:     c.Alive,
			Inventory: w.InventoryOf(c.ID), // выводится из Item.Holder на месте
			Memory:    append([]Seq(nil), c.Memory...),
		}
		s.Creatures = append(s.Creatures, cs)
	}
	for _, id := range w.itemOrder {
		it := w.items[id]
		s.Items = append(s.Items, ItemSnapshot{
			ID:       it.ID,
			Kind:     it.Kind,
			Holder:   it.Holder,
			Pos:      it.Pos,
			Owner:    it.Owner,
			Consumed: it.Consumed,
		})
	}
	return s
}

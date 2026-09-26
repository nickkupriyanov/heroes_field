package sim

// Event — свершившийся факт: единица потока, из которой строится всё —
// состояние мира (через apply), память существ, журнал и будущие replay/save.
//
// Форма deliberately плоская: один struct на все виды фактов, а не иерархия
// интерфейсов. Плоская форма даёт фиксированный порядок полей при сериализации
// (encoding/json выводит поля в порядке объявления), лёгкий byte-в-byte
// диф текущих и старых партий и простое добавление новых видов фактов
// без ломания формата.
//
// Поля-участники опциональны: нулевое значение ID означает «участника нет»,
// поэтому omitempty убирает нерелевантные поля из каждой строки JSONL.
// Pos и Dir — указатели, потому что omitempty не умеет опускать структуры:
// структура не бывает «пустой» с точки зрения encoding/json.
//
// Cause — полисемантично: в зависимости от Kind это DeathCause, DamageCause,
// ClaimCloseReason или EndCause. Все они uint8 со стабильными значениями,
// а плоское поле не разрастает формат.
type Event struct {
	SchemaVersion int        `json:"v"`                // версия схемы потока
	Seq           Seq        `json:"seq"`              // сквозной номер события
	Turn          int        `json:"turn"`             // номер хода свершения
	Kind          EventKind  `json:"kind"`             // вид факта
	Actor         EntityID   `json:"actor,omitempty"`  // кто совершил
	Target        EntityID   `json:"target,omitempty"` // над кем/чем совершено
	Item          ItemID     `json:"item,omitempty"`   // предмет-участник
	Pos           *Vec2      `json:"pos,omitempty"`    // место свершения
	Dir           *Vec2      `json:"dir,omitempty"`    // направление (шаг, удар)
	Amount        int        `json:"amount,omitempty"` // число (урон, метка)
	Cause         uint8      `json:"cause,omitempty"`  // причина (по виду факта)
	Witnesses     []EntityID `json:"wit,omitempty"`    // кто видел, отсортировано
}

// Memorable сообщает, попадает ли факт вида k в память существ.
//
// Память — кольцевой буфер, и состав его содержимого — решение дизайна, а не
// техника. Правила отбора:
//
//   - бытовые факты (Moved, Waited) не запоминаются: они утопили бы кольцо
//     и вытеснили обиды, ради которых память существует;
//   - метафакты игрока (BodyChosen, ClaimOpened/Closed, CampaignEnded,
//     WorldCreated) не запоминаются: существа не понимают ни переселения
//     душ, ни чьих-то целей мести — это знание игрока, а не обитателя;
//   - всё, что можно увидеть и затаить (удары, смерти, чужие руки на
//     предметах, поедание трупов), — запоминается.
func (k EventKind) Memorable() bool {
	switch k {
	case EventAttacked, EventDamaged, EventDied, EventCorpseCreated,
		EventItemDropped, EventItemPickedUp, EventItemUsed:
		return true
	}
	return false
}

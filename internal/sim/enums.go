package sim

// Все enum'ы этого файла сериализуются в JSONL целыми числами, поэтому их
// значения — часть формата. Вставка нового значения в середину списка —
// несовместимое изменение: это ломает replay старых партий. Новые значения
// добавляются только в конец; тест TestEnumStability ловит любые сдвиги.

// SchemaVersion — версия схемы потока событий. Инкрементируется при любом
// несовместимом изменении формата; каждое событие несёт её, чтобы replay
// мог отказаться от чужой версии, а не молча воспроизвести ерунду.
const SchemaVersion = 2

// CellKind — тип клетки карты.
type CellKind uint8

const (
	CellWall  CellKind = 1 // непроходимая стена, блокирует движение и обзор
	CellFloor CellKind = 2 // проходимый пол
)

// CreatureKind — вид существа. В MVP вид совпадает с фракцией: три вида —
// три враждующие стороны, поэтому отдельного типа «фракция» нет.
type CreatureKind uint8

const (
	CreatureGoblin  CreatureKind = 1 // гоблин, кладовая
	CreatureRat     CreatureKind = 2 // крыса, тоннели
	CreatureCultist CreatureKind = 3 // культист, святилище
)

// ActionKind — намерение существа на текущий ход. Действия проверяются
// в два шага: синтаксис (например, шаг не длиннее одной клетки) — сразу
// в Action; семантика против мира (клетка свободна, цель существует) —
// в World.Step.
type ActionKind uint8

const (
	ActionMove   ActionKind = 1 // шаг на соседнюю клетку
	ActionWait   ActionKind = 2 // пропуск хода
	ActionAttack ActionKind = 3 // ближняя атака соседа
	ActionPickUp ActionKind = 4 // поднять предмет с текущей клетки
	ActionDrop   ActionKind = 5 // выбросить предмет из инвентаря на клетку
	ActionUse    ActionKind = 6 // употребить предмет из инвентаря (съесть еду)
)

// EventKind — свершившийся факт потока. Журнал и память существ — проекции
// этого потока, поэтому состав kinds отражает то, что важно помнить
// и о чём важно рассказать, а не внутренние пересчёты состояния.
type EventKind uint8

const (
	EventWorldCreated      EventKind = 1  // genesis: полная конфигурация исходного мира
	EventMoved             EventKind = 2  // существо сменило клетку
	EventWaited            EventKind = 3  // существо переждало ход
	EventAttacked          EventKind = 4  // атака: актёр и цель
	EventDamaged           EventKind = 5  // получен урон (бой или голод)
	EventDied              EventKind = 6  // существо погибло
	EventCorpseCreated     EventKind = 7  // из погибшего возник предмет-труп
	EventItemDropped       EventKind = 8  // предмет покинул инвентарь/клетку
	EventItemPickedUp      EventKind = 9  // предмет поднят с клетки
	EventItemUsed          EventKind = 10 // предмет употреблён (еда съедена)
	EventClaimOpened       EventKind = 11 // создана активная цель мести
	EventClaimClosed       EventKind = 12 // цель мести закрыта (любым исходом)
	EventBodyChosen        EventKind = 13 // игрок переселился в новое тело
	EventCampaignEnded     EventKind = 14 // кампания завершена (победа/вымирание)
	EventTurnEnded         EventKind = 15 // ход завершён, номер хода увеличен
	EventCampaignContinued EventKind = 16 // после победы включён sandbox
)

// ItemKind — категория предмета.
type ItemKind uint8

const (
	ItemFood   ItemKind = 1 // еда: употребление сбрасывает голод
	ItemWeapon ItemKind = 2 // оружие: увеличивает урон в ближнем бою
	ItemCorpse ItemKind = 3 // труп: его можно съесть, свидетели это запомнят
)

// DeathCause — причина смерти. Безличные причины (голод) не порождают цель
// мести, поэтому различие важно не только для журнала.
type DeathCause uint8

const (
	DeathCombat DeathCause = 1 // убит другим существом
	DeathHunger DeathCause = 2 // истощён голодом
)

// DamageCause — источник урона, отдан от причины смерти, чтобы событие
// урона само объясняло себя без обращения к соседним событиям.
type DamageCause uint8

const (
	DamageCombat DamageCause = 1 // ранен в ближнем бою
	DamageHunger DamageCause = 2 // истощение от голода
)

// ClaimState — состояние цели мести.
type ClaimState uint8

const (
	ClaimActive    ClaimState = 1 // месть ждёт исполнения
	ClaimFulfilled ClaimState = 2 // игрок убил цель — метка наследия получена
	ClaimMissed    ClaimState = 3 // цель погибла иначе или стала телом игрока
)

// ClaimCloseReason — почему цель мести закрылась. Дублирует итог, но хранит
// точную причину для итоговой хронологии кампании.
type ClaimCloseReason uint8

const (
	ClaimClosedByFulfillment ClaimCloseReason = 1 // убийство цели управляемым персонажем
	ClaimClosedByOtherDeath  ClaimCloseReason = 2 // цель погибла не от руки игрока
	ClaimClosedByPossession  ClaimCloseReason = 3 // цель выбрана новым телом игрока
)

// EndCause — причина завершения кампании.
type EndCause uint8

const (
	EndVictory    EndCause = 1 // набраны три метки наследия
	EndExtinction EndCause = 2 // не осталось живых существ
)

package sim

import "errors"

// ErrInvalidAction — действие не прошло проверку. World.Step возвращает эту
// ошибку, не создавая событий и не расходуя ход: игрок просто пробует другое.
var ErrInvalidAction = errors.New("invalid action")

// Action — намерение существа на текущий ход.
//
// Действие — это только желание: оно ничего не меняет в мире и проверяется
// в два приёма. Validate проверяет синтаксис без знания мира (шаг не длиннее
// клетки, у поднятия указан предмет); World.Step дополнительно проверяет
// семантику против текущего состояния: свободна ли клетка, жива ли цель,
// лежит ли предмет под ногами. Так ввод и AI формируют намерения одинаково
// и не могут «протолкнуть» некорректное изменение напрямую.
type Action struct {
	Kind ActionKind // что именно хочет сделать существо
	Dir  Vec2       // Move/Attack: направление на соседнюю клетку
	Item ItemID     // PickUp/Drop/Use: предмет из инвентаря или с клетки
}

// Move — шаг на соседнюю клетку в направлении d.
func Move(d Vec2) Action { return Action{Kind: ActionMove, Dir: d} }

// Wait — переждать ход.
func Wait() Action { return Action{Kind: ActionWait} }

// Attack — ближняя атака соседа в направлении d.
func Attack(d Vec2) Action { return Action{Kind: ActionAttack, Dir: d} }

// PickUp — поднять предмет с текущей клетки.
func PickUp(item ItemID) Action { return Action{Kind: ActionPickUp, Item: item} }

// Drop — выбросить предмет из инвентаря на текущую клетку.
func Drop(item ItemID) Action { return Action{Kind: ActionDrop, Item: item} }

// Use — употребить предмет из инвентаря (например, съесть еду).
func Use(item ItemID) Action { return Action{Kind: ActionUse, Item: item} }

// stepOK проверяет, что d — шаг ровно на одну из восьми соседних клеток.
// Диагонали разрешены: это упрощает и ввод, и обход узких тоннелей.
func stepOK(d Vec2) bool {
	if d.X < -1 || d.X > 1 || d.Y < -1 || d.Y > 1 {
		return false
	}
	return d != (Vec2{}) // шаг «на месте» — это Wait, а не Move
}

// Validate проверяет синтаксис действия без обращения к миру.
// Причины: поймать очевидно сломанное намерение ещё до симуляции и держать
// правила формы действий в одном месте — и для игрока, и для AI.
func (a Action) Validate() error {
	switch a.Kind {
	case ActionMove, ActionAttack:
		if !stepOK(a.Dir) || a.Item != 0 {
			return ErrInvalidAction
		}
	case ActionWait:
		if a.Dir != (Vec2{}) || a.Item != 0 {
			return ErrInvalidAction
		}
	case ActionPickUp, ActionDrop, ActionUse:
		if a.Item == 0 || a.Dir != (Vec2{}) {
			return ErrInvalidAction
		}
	default:
		return ErrInvalidAction // вид действия не существует — значение не из enum
	}
	return nil
}

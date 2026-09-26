package sim

import "fmt"

// Step применяет ход игрока и последующие фазы хода, возвращая все события хода.
//
// Канонический порядок фаз — часть формата (DESIGN §4):
//
//  1. действие игрока — намеренное преимущество первого действия;
//  2. действия живых NPC по возрастанию EntityID; список фиксируется
//     в начале фазы: родившиеся в ходе начнут со следующего, погибшие
//     посреди фазы пропускаются;
//  3. upkeep: рост голода, урон и смерти от истощения;
//  4. закрытие целей мести, чьи носители погибли;
//  5. проверка победы, вымирания и необходимости смены тела;
//  6. завершение хода — событием TurnEnded, несущим номер завершившегося
//     хода: apply увеличивает turn по нему.
//
// Невалидное действие — синтаксически или семантически — возвращает ошибку
// без единого события и без расхода хода: игрок просто пробует другое.
// Фазы 3–5 заполняются по мере вех (M3, M5); конвейер и его порядок
// фиксируются сейчас, чтобы события никогда не меняли свой порядок
// в зависимости от стадии разработки.
func (w *World) Step(playerAction Action) ([]Event, error) {
	if err := playerAction.Validate(); err != nil {
		return nil, err
	}
	player := w.creatures[w.player]
	if player == nil || !player.Alive {
		// В M5 смерть игрока открывает выбор нового тела, а не ошибку;
		// до тех пор тело игрока умереть не может.
		return nil, fmt.Errorf("%w: тело игрока мертво", ErrInvalidAction)
	}

	var events []Event

	// Ход начинается здесь: все события хода, от первого шага игрока
	// до TurnEnded, несут один и тот же номер. Счётчик восстанавливается
	// replay'ем из TurnEnded, поэтому Step — единственное место,
	// где он двигается напрямую.
	//
	// Откат при ошибке корректен благодаря инварианту act: либо ошибка
	// без единого события, либо события без ошибки. Ни одно событие
	// не эмитится до полного прохождения проверок.
	w.turn++
	evs, err := w.act(w.player, playerAction)
	if err != nil {
		w.turn-- // шаг не состоялся — ход не начат
		return nil, err
	}
	events = append(events, evs...)

	// Фаза 2: действия NPC в стабильном порядке.
	// Снимок порядка делается до фазы: срез order может удлинниться
	// в течение хода, но участники этого хода фиксируются заранее.
	npcOrder := append([]EntityID(nil), w.order...)
	for _, id := range npcOrder {
		if id == w.player {
			continue // игрок уже acted в фазе 1
		}
		c := w.creatures[id]
		if c == nil || !c.Alive {
			continue
		}
		// Utility-AI подключается в M4; до того мирные жители не действуют.
	}

	// Фаза 3: upkeep (голод и истощение — M3.3).
	// Фаза 4: закрытие целей мести (M5).
	// Фаза 5: победа/вымирание/смена тела (M3.4, M5).

	// Фаза 6: ход завершён. TurnEnded — всегда последнее событие хода.
	events = append(events, w.emit(Event{Kind: EventTurnEnded}))
	return events, nil
}

// act применяет действие существа после семантической проверки против мира.
//
// Один и тот же код обслуживает игрока и NPC: у намерений нет владельца,
// у правил — есть. Проверки семантики здесь, а не в Action.Validate,
// потому что для них нужно состояние мира: занятость клетки, живость цели.
//
// Инвариант: act возвращает либо ошибку без единого события (ничего
// не произошло), либо события без ошибки. Step опирается на это при откате
// номера хода, поэтому проверки всегда предшествуют эмиссии.
func (w *World) act(id EntityID, a Action) ([]Event, error) {
	c := w.creatures[id]
	switch a.Kind {
	case ActionWait:
		pos := c.Pos
		return []Event{w.emit(Event{Kind: EventWaited, Actor: id, Pos: &pos})}, nil
	case ActionMove:
		to := c.Pos.Add(a.Dir)
		if w.CellAt(to.X, to.Y) != CellFloor {
			return nil, fmt.Errorf("%w: клетка %v — стена", ErrInvalidAction, to)
		}
		if w.occupiedBy(to) != 0 {
			return nil, fmt.Errorf("%w: клетка %v занята", ErrInvalidAction, to)
		}
		dir := a.Dir
		return []Event{w.emit(Event{Kind: EventMoved, Actor: id, Pos: &to, Dir: &dir})}, nil
	default:
		// Атака и предметы приходят в M3; для них sintaxис уже валиден,
		// но мир ещё не умеет их применять.
		return nil, fmt.Errorf("%w: действие %d ещё не поддержано", ErrInvalidAction, a.Kind)
	}
}

// occupiedBy возвращает ID живого существа в клетке или 0, если клетка свободна.
// Предметы на полу не занимают клетку: на них встают, чтобы поднять.
func (w *World) occupiedBy(p Vec2) EntityID {
	for _, id := range w.order {
		c := w.creatures[id]
		if c != nil && c.Alive && c.Pos == p {
			return id
		}
	}
	return 0
}

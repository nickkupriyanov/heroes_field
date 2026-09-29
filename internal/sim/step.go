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
// Все шесть фаз входят в формат и не меняют порядок между прогонами.
func (w *World) Step(playerAction Action) ([]Event, error) {
	if w.ended != 0 {
		return nil, fmt.Errorf("%w: %s", ErrCampaignOver, endWord(w.ended))
	}
	if w.NeedsBodyChoice() {
		return nil, ErrBodyChoiceRequired
	}
	if err := playerAction.Validate(); err != nil {
		return nil, err
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
		// Контроллер предлагает намерение; мир проверяет его сам.
		// Сломанное намерение (синтаксис или семантика) не зависает,
		// а превращается в потерянный ход — как в жизни.
		action := w.ai.Decide(w, id)
		if err := action.Validate(); err != nil {
			action = Wait()
		}
		evs, err := w.act(id, action)
		if err != nil {
			evs, err = w.act(id, Wait())
		}
		if err == nil {
			events = append(events, evs...)
		}
	}

	// Фаза 3: upkeep — голод растёт как производная (ход − последняя еда),
	// а истощение списывает здоровье. Порядок — по возрастанию ID:
	// умирание посреди upkeep не меняет порядок остальных.
	upkeepOrder := append([]EntityID(nil), w.order...)
	for _, id := range upkeepOrder {
		c := w.creatures[id]
		if c == nil || !c.Alive {
			continue
		}
		if w.turn-c.LastFedTurn > HungerGrace {
			pos := c.Pos
			events = append(events, w.emit(Event{
				Kind:      EventDamaged,
				Target:    id,
				Amount:    1,
				Cause:     uint8(DamageHunger),
				Pos:       &pos,
				Witnesses: w.witnesses(pos),
			}))
			// emit применяет сразу: здоровье уже списано.
			if c.HP <= 0 {
				events = append(events, w.kill(id, 0, DeathHunger)...)
			}
		}
	}

	// Фаза 4: открытие и закрытие целей мести.
	events = append(events, w.resolveVengeance(events)...)

	// Фаза 5: проверка результата (DESIGN §2): победа > вымирание >
	// смена тела > следующий ход.
	alive := 0
	for _, id := range w.order {
		if c := w.creatures[id]; c != nil && c.Alive {
			alive++
		}
	}
	if !w.sandbox && w.legacy >= LegacyGoal {
		events = append(events, w.emit(Event{
			Kind: EventCampaignEnded, Cause: uint8(EndVictory), Amount: w.legacy,
		}))
	} else if alive == 0 {
		events = append(events, w.emit(Event{
			Kind:  EventCampaignEnded,
			Cause: uint8(EndExtinction),
		}))
	}

	// Фаза 6: ход завершён. TurnEnded — всегда последнее событие хода.
	events = append(events, w.emit(Event{Kind: EventTurnEnded}))
	return events, nil
}

// endWord — русское слово исхода для сообщения об ошибке.
func endWord(c EndCause) string {
	if c == EndVictory {
		return "победа"
	}
	return "вымирание"
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
		return []Event{w.emit(Event{
			Kind:      EventWaited,
			Actor:     id,
			Pos:       &pos,
			Witnesses: w.witnesses(pos),
		})}, nil
	case ActionMove:
		to := c.Pos.Add(a.Dir)
		if w.CellAt(to.X, to.Y) != CellFloor {
			return nil, fmt.Errorf("%w: клетка %v — стена", ErrInvalidAction, to)
		}
		if w.occupiedBy(to) != 0 {
			return nil, fmt.Errorf("%w: клетка %v занята", ErrInvalidAction, to)
		}
		dir := a.Dir
		return []Event{w.emit(Event{
			Kind:      EventMoved,
			Actor:     id,
			Pos:       &to,
			Dir:       &dir,
			Witnesses: w.witnesses(to),
		})}, nil
	case ActionPickUp:
		it := w.items[a.Item]
		if it == nil || it.Holder != 0 || it.Pos != c.Pos {
			// Предмет должен лежать именно под ногами: «поднять то, что
			// видно в двух клетках» — это не поднятие, а воровство на расстоянии.
			return nil, fmt.Errorf("%w: предмет %d не под рукой", ErrInvalidAction, a.Item)
		}
		pos := it.Pos
		return []Event{w.emit(Event{
			Kind:      EventItemPickedUp,
			Actor:     id,
			Item:      a.Item,
			Pos:       &pos,
			Witnesses: w.witnesses(pos),
		})}, nil
	case ActionDrop:
		it := w.items[a.Item]
		if it == nil || it.Holder != id {
			return nil, fmt.Errorf("%w: предмет %d не у тебя", ErrInvalidAction, a.Item)
		}
		pos := c.Pos
		return []Event{w.emit(Event{
			Kind:      EventItemDropped,
			Actor:     id,
			Item:      a.Item,
			Pos:       &pos,
			Witnesses: w.witnesses(pos),
		})}, nil
	case ActionAttack:
		to := c.Pos.Add(a.Dir)
		victimID := w.occupiedBy(to)
		if victimID == 0 {
			// Бить можно только живых соседей: труп — уже предмет,
			// стена — не собеседник.
			return nil, fmt.Errorf("%w: там некого бить", ErrInvalidAction)
		}
		damage := KindDamage(c.Kind)
		if w.HoldingWeapon(id) {
			damage++ // любая железка в руках стоит лишней единицы урона
		}
		dir, pos := a.Dir, w.creatures[victimID].Pos
		events := []Event{
			w.emit(Event{
				Kind:      EventAttacked,
				Actor:     id,
				Target:    victimID,
				Pos:       &pos,
				Dir:       &dir,
				Witnesses: w.witnesses(pos),
			}),
			w.emit(Event{
				Kind:      EventDamaged,
				Actor:     id,
				Target:    victimID,
				Amount:    damage,
				Cause:     uint8(DamageCombat),
				Pos:       &pos,
				Witnesses: w.witnesses(pos),
			}),
		}
		// emit применяет события сразу: после Damaged здоровье уже списано.
		if w.creatures[victimID].HP <= 0 {
			events = append(events, w.kill(victimID, id, DeathCombat)...)
		}
		return events, nil
	case ActionUse:
		it := w.items[a.Item]
		if it == nil || it.Holder != id {
			return nil, fmt.Errorf("%w: предмет %d не у тебя", ErrInvalidAction, a.Item)
		}
		if it.Kind != ItemFood && it.Kind != ItemCorpse {
			// Оружие не едят: его поднимают для другого.
			return nil, fmt.Errorf("%w: это не еда", ErrInvalidAction)
		}
		pos := c.Pos
		return []Event{w.emit(Event{
			Kind:      EventItemUsed,
			Actor:     id,
			Item:      a.Item,
			Pos:       &pos,
			Witnesses: w.witnesses(pos),
		})}, nil
	default:
		// Все шесть действий мира поддержаны; сюда попадают только
		// несуществующие значения вида — защита от искажённых намерений.
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

// kill проводит цепочку смерти: Died, CorpseCreated и выпадение инвентаря
// по одному предмету. Убийца нужен цели мести и журналу; Actor=0 у Died
// означает безличную смерть.
//
// Порядок фиксирован: сначала сама смерть, потом труп, потом вещи —
// так свидетели видят причину раньше её материальных последствий,
// и replay читается как протокол вскрытия.
func (w *World) kill(victim, killer EntityID, cause DeathCause) []Event {
	v := w.creatures[victim]
	pos := v.Pos
	var events []Event
	events = append(events, w.emit(Event{
		Kind:      EventDied,
		Actor:     killer,
		Target:    victim,
		Cause:     uint8(cause),
		Pos:       &pos,
		Witnesses: w.witnesses(pos),
	}))
	// Труп — предмет с ID погибшего: свидетели каннибализма узнают жертву
	// именно через эту связь.
	events = append(events, w.emit(Event{
		Kind:      EventCorpseCreated,
		Target:    victim,
		Item:      w.nextItem,
		Pos:       &pos,
		Witnesses: w.witnesses(pos),
	}))
	// Инвентарь выпадает в клетку смерти в порядке itemOrder.
	for _, itemID := range w.InventoryOf(victim) {
		events = append(events, w.emit(Event{
			Kind:      EventItemDropped,
			Actor:     victim,
			Item:      itemID,
			Pos:       &pos,
			Witnesses: w.witnesses(pos),
		}))
	}
	return events
}

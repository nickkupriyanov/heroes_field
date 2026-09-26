package sim

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// TestEventJSONRoundTrip проверяет, что событие переживает сериализацию без
// потерь: это контракт replay — каждая строка JSONL должна восстанавливаться
// в то же событие.
func TestEventJSONRoundTrip(t *testing.T) {
	full := Event{
		SchemaVersion: SchemaVersion,
		Seq:           42,
		Turn:          7,
		Kind:          EventAttacked,
		Actor:         3,
		Target:        5,
		Item:          2,
		Pos:           &Vec2{X: 12, Y: 4},
		Dir:           &Vec2{X: -1, Y: 0},
		Amount:        3,
		Cause:         uint8(DamageCombat),
		Witnesses:     []EntityID{1, 5, 9},
	}
	minimal := Event{
		SchemaVersion: SchemaVersion,
		Seq:           43,
		Turn:          7,
		Kind:          EventWaited,
		Actor:         5,
	}
	for _, e := range []Event{full, minimal} {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("маршалинг события %+v: %v", e, err)
		}
		var back Event
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("разбор %s: %v", data, err)
		}
		if !reflect.DeepEqual(back, e) {
			t.Errorf("round-trip изменил событие: было %+v, стало %+v", e, back)
		}
	}
}

// TestEventJSONFieldOrder фиксирует порядок полей в JSON. Порядок — часть
// канонической формы: два одинаковых прогона обязаны давать байт-в-байт
// одинаковый JSONL, а прочитать поток глазами должно быть возможно без
// скачков туда-сюда.
func TestEventJSONFieldOrder(t *testing.T) {
	e := Event{
		SchemaVersion: SchemaVersion,
		Seq:           1,
		Turn:          1,
		Kind:          EventAttacked,
		Actor:         3,
		Target:        5,
		Item:          2,
		Pos:           &Vec2{X: 12, Y: 4},
		Dir:           &Vec2{X: -1, Y: 0},
		Amount:        3,
		Cause:         1,
		Witnesses:     []EntityID{1, 5, 9},
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("маршалинг: %v", err)
	}
	keys := []string{`"v":`, `"seq":`, `"turn":`, `"kind":`, `"actor":`, `"target":`, `"item":`, `"pos":`, `"dir":`, `"amount":`, `"cause":`, `"wit":`}
	prev := -1
	for _, k := range keys {
		i := bytes.Index(data, []byte(k))
		if i < 0 {
			t.Fatalf("поле %s отсутствует в %s", k, data)
		}
		if i < prev {
			t.Errorf("поле %s стоит не по порядку в %s", k, data)
		}
		prev = i
	}
}

// TestEventOmitEmpty проверяет, что нерелевантные поля исчезают из строки:
// краткие факты не должны тащить за собой пустые хвосты полей.
func TestEventOmitEmpty(t *testing.T) {
	e := Event{SchemaVersion: SchemaVersion, Seq: 1, Turn: 1, Kind: EventWaited, Actor: 5}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("маршалинг: %v", err)
	}
	for _, absent := range []string{`"target":`, `"item":`, `"pos":`, `"dir":`, `"amount":`, `"cause":`, `"wit":`} {
		if bytes.Contains(data, []byte(absent)) {
			t.Errorf("пустое поле %s не опущено: %s", absent, data)
		}
	}
}

// TestMemorable фиксирует отбор фактов в память существ: бытовые факты
// не должны вытеснять обиды из кольца, а метафакты игрока не должны
// быть доступны обитателям мира.
func TestMemorable(t *testing.T) {
	memorable := map[EventKind]bool{
		EventAttacked:      true,
		EventDamaged:       true,
		EventDied:          true,
		EventCorpseCreated: true,
		EventItemDropped:   true,
		EventItemPickedUp:  true,
		EventItemUsed:      true,
	}
	for k := EventKind(1); k <= EventTurnEnded; k++ {
		if got, want := k.Memorable(), memorable[k]; got != want {
			t.Errorf("Memorable(%d) = %v, ожидается %v", k, got, want)
		}
	}
}

// TestActionValidate проверяет синтаксическую валидацию намерений:
// мировой семантики тут нет — только форма.
func TestActionValidate(t *testing.T) {
	valid := []Action{
		Move(Vec2{X: 1, Y: 0}),
		Move(Vec2{X: -1, Y: -1}),
		Wait(),
		Attack(Vec2{X: 0, Y: 1}),
		PickUp(3),
		Drop(3),
		Use(3),
	}
	for _, a := range valid {
		if err := a.Validate(); err != nil {
			t.Errorf("действие %+v отвергнуто синтаксисом: %v", a, err)
		}
	}
	invalid := []Action{
		{},                 // вид действия не задан
		{Kind: ActionMove}, // шаг без направления
		{Kind: ActionMove, Dir: Vec2{X: 2, Y: 0}},          // длиннее клетки
		{Kind: ActionMove, Dir: Vec2{X: 1, Y: 1}, Item: 2}, // шаг с предметом
		{Kind: ActionWait, Dir: Vec2{X: 1, Y: 0}},          // ожидание с направлением
		{Kind: ActionAttack},                               // удар без направления
		{Kind: ActionPickUp},                               // поднятие без предмета
		{Kind: ActionDrop, Dir: Vec2{X: 0, Y: -1}},         // выброс с направлением
		{Kind: ActionUse},                                  // употребление без предмета
		{Kind: ActionKind(99)},                             // несуществующий вид
	}
	for _, a := range invalid {
		if err := a.Validate(); err == nil {
			t.Errorf("действие %+v ошибочно принято", a)
		}
	}
}

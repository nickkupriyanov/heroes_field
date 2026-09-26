package sim

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestNewWorldGenesis проверяет genesis-событие и мир, который из него
// построен: событие — полный самодостаточный слепок партии, поэтому
// всё содержимое проверяется жёстко.
func TestNewWorldGenesis(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, ev := NewWorld(s, 42)
	if ev.Kind != EventWorldCreated {
		t.Fatalf("вид события %d, ожидается WorldCreated", ev.Kind)
	}
	if ev.Seq != 1 || ev.Turn != 0 || ev.SchemaVersion != SchemaVersion {
		t.Errorf("канонические поля genesis: seq=%d turn=%d v=%d", ev.Seq, ev.Turn, ev.SchemaVersion)
	}
	if ev.Actor != 0 || ev.Target != 0 || ev.Genesis == nil {
		t.Errorf("у genesis не должно быть участников без payload")
	}
	g := ev.Genesis
	if g.Seed != 42 || g.Width != 32 || g.Height != 20 || len(g.Cells) != 640 {
		t.Errorf("метрики genesis: %+v, клеток %d", g, len(g.Cells))
	}
	if len(g.Creatures) != 12 || len(g.Items) != 8 {
		t.Errorf("в genesis существ %d и предметов %d, ожидается 12 и 8", len(g.Creatures), len(g.Items))
	}
	if g.Player != 1 {
		t.Errorf("стартовое тело игрока %d, ожидается 1", g.Player)
	}

	// ID назначаются в порядке чтения карты: второй встреченный житель —
	// культист в (24,2), а не второй гоблин. Порядок задаёт очередь хода NPC,
	// поэтому фиксируется тестом.
	second := g.Creatures[1]
	if second.ID != 2 || second.Kind != CreatureCultist || second.Pos != (Vec2{X: 24, Y: 2}) {
		t.Errorf("второе существо %+v, ожидается культист #2 в (24,2)", second)
	}
	last := g.Creatures[len(g.Creatures)-1]
	if last.ID != 12 || last.Kind != CreatureRat || last.Pos != (Vec2{X: 6, Y: 17}) {
		t.Errorf("последнее существо %+v, ожидается крыса #12 в (6,17)", last)
	}

	// Мир соответствует своему событию — единственный путь построения.
	if w.Turn() != 0 || w.Player() != 1 {
		t.Errorf("состояние мира: turn=%d player=%d, ожидается 0 и 1", w.Turn(), w.Player())
	}
	first := w.Creature(1)
	if first == nil || first.Kind != CreatureGoblin || first.Pos != (Vec2{X: 2, Y: 2}) {
		t.Errorf("первое существо %+v, ожидается гоблин в (2,2)", first)
	}
	if first.HP != first.MaxHP || first.HP != KindMaxHP(CreatureGoblin) || !first.Alive {
		t.Errorf("гоблин рождается целым и живым: %+v", first)
	}
	weapon := w.Item(3)
	if weapon == nil || weapon.Kind != ItemWeapon || weapon.Pos != (Vec2{X: 29, Y: 4}) {
		t.Errorf("предмет #3 %+v, ожидается оружие в (29,4)", weapon)
	}
	if w.CellAt(0, 0) != CellWall || w.CellAt(2, 2) != CellFloor {
		t.Errorf("клетки мира не соответствуют карте сценария")
	}
}

// TestGenesisJSONRoundTrip: genesis — первая строка каждого JSONL,
// потеря хотя бы поля при сериализации ломает replay всех партий.
func TestGenesisJSONRoundTrip(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	_, ev := NewWorld(s, 7)
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("маршалинг genesis: %v", err)
	}
	var back Event
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("разбор genesis: %v", err)
	}
	if !reflect.DeepEqual(back, ev) {
		t.Errorf("round-trip изменил genesis-событие")
	}
	// Genesis присутствует и стоит последним полем строки.
	if back.Genesis == nil || back.Genesis.Seed != 7 {
		t.Errorf("genesis потерян при round-trip")
	}
}

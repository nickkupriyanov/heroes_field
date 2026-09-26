package sim

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// roundTripJSON — вспомогательная функция тестов: сериализует значение,
// читает обратно и сверяет через DeepEqual. Живёт в тестах, потому что
// строгость нужна только самим проверкам.
func roundTripJSON[T any](v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	var back T
	if err := json.Unmarshal(data, &back); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	if !reflect.DeepEqual(v, back) {
		return fmt.Errorf("round-trip изменил значение: %s", data)
	}
	return nil
}

// TestReplayGenesis — главная проверка reducer'а: мир, восстановленный
// из genesis-события, байт-в-байт равен миру, построенному NewWorld'ом.
// Единственный путь построения — applyGenesis — гарантирует это по
// построению; тест держит гарантию под замком.
func TestReplayGenesis(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, ev := NewWorld(s, 99)
	rep, err := WorldFromEvents([]Event{ev})
	if err != nil {
		t.Fatalf("WorldFromEvents: %v", err)
	}
	if !reflect.DeepEqual(w.Snapshot(), rep.Snapshot()) {
		t.Error("мир после replay отличается от живого мира")
	}
}

// TestSnapshotJSONRoundTrip: слепок сериализуется без потерь — на нём
// строятся и сравнение миров, и будущий показ состояния.
func TestSnapshotJSONRoundTrip(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	w, _ := NewWorld(s, 5)
	if err := roundTripJSON(w.Snapshot()); err != nil {
		t.Errorf("round-trip слепка: %v", err)
	}
}

// TestReplayRejectsBadStreams — replay отказывается от битых потоков
// до того, как применит хотя бы часть: пустой поток, поток не с genesis
// первым, чужая версия схемы, дыра в нумерации.
func TestReplayRejectsBadStreams(t *testing.T) {
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	_, genesis := NewWorld(s, 5)

	if _, err := WorldFromEvents(nil); err == nil {
		t.Error("пустой поток принят")
	}
	notGenesis := genesis
	notGenesis.Kind = EventWaited
	if _, err := WorldFromEvents([]Event{notGenesis}); err == nil {
		t.Error("поток без genesis первым принят")
	}
	wrongVersion := genesis
	wrongVersion.SchemaVersion = SchemaVersion + 1
	if _, err := WorldFromEvents([]Event{wrongVersion}); err == nil {
		t.Error("чужая версия схемы принята")
	}
	// Событие с дырой в нумерации: seq должен проверяться до apply,
	// иначе reducer молча примет чужой порядок.
	gap := genesis
	gap.Seq = 2 // genesis обязан быть первым
	if _, err := WorldFromEvents([]Event{gap}); err == nil {
		t.Error("дыра в нумерации seq принята")
	}
}

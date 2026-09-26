package sim

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// sampleEvents строит поток из genesis-события и пары синтетических
// фактов: достаточно, чтобы проверить форму, не дожидаясь ходов.
func sampleEvents(t *testing.T) []Event {
	t.Helper()
	s, err := DefaultScenario()
	if err != nil {
		t.Fatalf("DefaultScenario: %v", err)
	}
	_, ev := NewWorld(s, 11)
	return []Event{
		ev,
		{SchemaVersion: SchemaVersion, Seq: 2, Turn: 1, Kind: EventWaited, Actor: 1},
		{SchemaVersion: SchemaVersion, Seq: 3, Turn: 1, Kind: EventMoved, Actor: 1,
			Pos: &Vec2{X: 3, Y: 2}, Dir: &Vec2{X: 1, Y: 0}, Witnesses: []EntityID{2, 1, 5}},
	}
}

// TestJSONLByteToByte — ядро детерминизма: одинаковые события обязаны
// кодироваться в одинаковые байты, а путешествие через файл не должно
// менять ни одного байта.
func TestJSONLByteToByte(t *testing.T) {
	events := sampleEvents(t)
	var buf1, buf2 bytes.Buffer
	if err := EncodeEvents(&buf1, events); err != nil {
		t.Fatalf("кодирование: %v", err)
	}
	if err := EncodeEvents(&buf2, events); err != nil {
		t.Fatalf("кодирование: %v", err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("повторное кодирование того же потока даёт разные байты")
	}

	// Чтение осушает буфер, поэтому снимок первых байтов делается до него.
	first := append([]byte(nil), buf1.Bytes()...)
	back, err := ReadEvents(&buf1)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	if !reflect.DeepEqual(back, events) {
		t.Error("round-trip изменил события")
	}
	var buf3 bytes.Buffer
	if err := EncodeEvents(&buf3, back); err != nil {
		t.Fatalf("повторное кодирование: %v", err)
	}
	if !bytes.Equal(first, buf3.Bytes()) {
		t.Error("кодирование после round-trip изменило байты")
	}
}

// TestJSONLLinePerEvent проверяет структуру файла: строк ровно столько,
// сколько событий, и никаких пустых.
func TestJSONLLinePerEvent(t *testing.T) {
	events := sampleEvents(t)
	var buf bytes.Buffer
	if err := EncodeEvents(&buf, events); err != nil {
		t.Fatalf("кодирование: %v", err)
	}
	if got := strings.Count(buf.String(), "\n"); got != len(events) {
		t.Errorf("строк %d, ожидается %d", got, len(events))
	}
}

// TestReadEventsContract фиксирует контракт чтения: пустой поток — это
// ноль событий без ошибки (валидность «в потоке должен быть genesis»
// проверяет replay, а не декодер); мусор и пустые строки посреди потока —
// ошибки с номером строки, потому что diff'ят файл люди.
func TestReadEventsContract(t *testing.T) {
	events, err := ReadEvents(strings.NewReader(""))
	if err != nil || len(events) != 0 {
		t.Errorf("пустой поток: события %v, ошибка %v", events, err)
	}
	if _, err := ReadEvents(strings.NewReader("{\"kind\":1}\n\n{\"kind\":2}\n")); err == nil {
		t.Error("пустая строка в середине потока не поймана")
	} else if !strings.Contains(err.Error(), "строка 2") {
		t.Errorf("ошибка не называет строку: %v", err)
	}
	if _, err := ReadEvents(strings.NewReader("это не json\n")); err == nil {
		t.Error("мусорная строка прочитана без ошибки")
	}
}

// TestCanonicalizeSortsWitnesses проверяет, что событие рождается уже
// каноническим: свидетели отсортированы при создании, а не «дочищаются»
// при кодировании.
func TestCanonicalizeSortsWitnesses(t *testing.T) {
	ev := Event{Kind: EventWaited, Actor: 9, Witnesses: []EntityID{7, 3, 9, 1}}.canonicalize()
	if !reflect.DeepEqual(ev.Witnesses, []EntityID{1, 3, 7, 9}) {
		t.Errorf("свидетели не отсортированы: %v", ev.Witnesses)
	}
}

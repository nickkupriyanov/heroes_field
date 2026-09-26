package sim

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// EncodeEvents пишет события в виде JSON Lines: одна JSON-строка —
// одно событие, каждая завершается переводом строки.
//
// Каноническая форма достигается без ручной сборки строк: порядок полей
// задаётся объявлением структуры Event, коллекции в событиях — только
// отсортированные срезы, а SetEscapeHTML(false) исключает лишние
// escape-последовательности. В итоге два одинаковых прогона партии
// дают байт-в-байт одинаковый файл, который можно сравнивать обычным diff.
func EncodeEvents(w io.Writer, events []Event) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for i := range events {
		if err := enc.Encode(events[i]); err != nil {
			return fmt.Errorf("кодирование события %d: %w", events[i].Seq, err)
		}
	}
	return nil
}

// ReadEvents читает поток JSON Lines и возвращает события в порядке
// следования.
//
// Ошибка сопровождается номером строки: JSONL читают и diff'ят люди,
// поэтому указание строки превращает «битый файл» в конкретное место
// для правки. Пустые строки запрещены: в канонической форме их не бывает.
func ReadEvents(r io.Reader) ([]Event, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var events []Event
	line := 0
	for sc.Scan() {
		line++
		row := sc.Bytes()
		if len(row) == 0 {
			return nil, fmt.Errorf("строка %d пуста", line)
		}
		var e Event
		if err := json.Unmarshal(row, &e); err != nil {
			return nil, fmt.Errorf("строка %d: %w", line, err)
		}
		events = append(events, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("чтение потока: %w", err)
	}
	return events, nil
}

// canonicalize приводит событие к канонической форме: свидетели —
// в порядке возрастания ID. Вызывается в emit, поэтому событие рождается
// уже каноническим, и кодирование не «дочищает» его задним числом.
func (e Event) canonicalize() Event {
	if len(e.Witnesses) > 1 {
		sort.Slice(e.Witnesses, func(i, j int) bool { return e.Witnesses[i] < e.Witnesses[j] })
	}
	return e
}

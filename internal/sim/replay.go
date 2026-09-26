package sim

import "fmt"

// WorldFromEvents восстанавливает мир из потока событий, применяя каждое
// событие через reducer — без AI, без RNG, без сценария.
//
// Это и есть replay: файл JSONL, снятый с партии, разворачивается обратно
// в мир. Второго пути построения мира здесь нет и не должно появиться:
// если replay расходится с живым миром — баг в apply, а не здесь.
func WorldFromEvents(events []Event) (*World, error) {
	if len(events) == 0 {
		return nil, fmt.Errorf("поток пуст: нет genesis-события")
	}
	if events[0].Kind != EventWorldCreated {
		return nil, fmt.Errorf("первое событие имеет вид %d, ожидается WorldCreated", events[0].Kind)
	}
	w := &World{}
	for i := range events {
		e := &events[i]
		// Проверки делаются до применения: битый поток не должен успеть
		// испортить полпути мир перед ошибкой.
		if e.SchemaVersion != SchemaVersion {
			return nil, fmt.Errorf("событие seq=%d имеет версию схемы %d, ожидается %d", e.Seq, e.SchemaVersion, SchemaVersion)
		}
		if e.Seq != Seq(i+1) {
			return nil, fmt.Errorf("нарушена нумерация: событие №%d несёт seq=%d", i+1, e.Seq)
		}
		w.applyEvent(*e)
	}
	return w, nil
}

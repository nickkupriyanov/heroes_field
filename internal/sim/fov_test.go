package sim

import "testing"

// TestKindSight фиксирует дальность зрения видов: свидетели и туман
// зависят от этих чисел, а баланс «первый контакт к 30-му ходу» — подавно.
func TestKindSight(t *testing.T) {
	for _, tc := range []struct {
		kind CreatureKind
		want int
	}{
		{CreatureGoblin, 6},
		{CreatureRat, 5},
		{CreatureCultist, 7},
	} {
		if got := KindSight(tc.kind); got != tc.want {
			t.Errorf("KindSight(%d) = %d, ожидается %d", tc.kind, got, tc.want)
		}
	}
}

// TestVisibleBasics — базовые свойства взгляда: стена блокирует, но видна;
// расстояние обрезает; дальняя сторона стены — невидима.
func TestVisibleBasics(t *testing.T) {
	w := newParty(t)
	// Кладовая открыта: из (2,2) видна еда в (6,2).
	if !w.visible(Vec2{X: 2, Y: 2}, Vec2{X: 6, Y: 2}, 6) {
		t.Error("открытая кладовая: (6,2) не видна из (2,2)")
	}
	// Стена x=10 сама видна изнутри кладовой.
	if !w.visible(Vec2{X: 9, Y: 2}, Vec2{X: 10, Y: 2}, 6) {
		t.Error("лицо стены не видно")
	}
	// За стену не заглянешь: (11,2) скрыто за (10,2).
	if w.visible(Vec2{X: 9, Y: 2}, Vec2{X: 11, Y: 2}, 6) {
		t.Error("взгляд прошёл сквозь стену")
	}
	// Радиус обрезает даже открытое пространство.
	if w.visible(Vec2{X: 2, Y: 2}, Vec2{X: 8, Y: 2}, 5) {
		t.Error("взгляд дальше радиуса")
	}
	// Свою клетку видно всегда.
	if !w.visible(Vec2{X: 2, Y: 2}, Vec2{X: 2, Y: 2}, 0) {
		t.Error("собственная клетка не видна")
	}
}

// TestVisibleSymmetry — видимость обязана быть симметричной: свидетель
// видел событие тогда и только тогда, когда участник мог видеть свидетеля.
// Перебор всех пар клеток в радиусе 7 на всей карте.
func TestVisibleSymmetry(t *testing.T) {
	w := newParty(t)
	for y := 0; y < w.Height(); y++ {
		for x := 0; x < w.Width(); x++ {
			a := Vec2{X: x, Y: y}
			for dy := -7; dy <= 7; dy++ {
				for dx := -7; dx <= 7; dx++ {
					b := Vec2{X: x + dx, Y: y + dy}
					if b.X < 0 || b.Y < 0 || b.X >= w.Width() || b.Y >= w.Height() {
						continue
					}
					ab := w.visible(a, b, 7)
					ba := w.visible(b, a, 7)
					if ab != ba {
						t.Fatalf("асимметрия: (%v)→(%v)=%v, (%v)→(%v)=%v", a, b, ab, b, a, ba)
					}
				}
			}
		}
	}
}

// TestVisibleCells проверяет форму FOV: содержит точку наблюдения,
// обрезается радиусом, не проникает за стены.
func TestVisibleCells(t *testing.T) {
	w := newParty(t)
	cells := w.VisibleCells(Vec2{X: 2, Y: 2}, KindSight(CreatureGoblin))
	set := map[Vec2]bool{}
	for _, c := range cells {
		set[c] = true
	}
	if !set[Vec2{X: 2, Y: 2}] {
		t.Error("точка наблюдения не входит в FOV")
	}
	if !set[Vec2{X: 8, Y: 2}] {
		t.Error("(8,2) на открытой кладовой не видна при радиусе 6")
	}
	if set[Vec2{X: 9, Y: 2}] {
		t.Error("(9,2) за пределами радиуса попала в FOV")
	}
	if set[Vec2{X: 12, Y: 2}] {
		t.Error("взгляд проник за стену кладовой")
	}
	if len(cells) == 0 || len(cells) > 13*13 {
		t.Errorf("подозрительный размер FOV: %d", len(cells))
	}
}

// TestWitnessesOfPantryEvent — свидетели события в кладовой: все гоблины
// рядом видят, чужаки из тоннелей и святилища — нет. Список канонический:
// по возрастанию ID, актёр включён.
func TestWitnessesOfPantryEvent(t *testing.T) {
	w := newParty(t)
	// Событие в (2,2) — там стоит игрок-гоблин #1. Гоблины #4 (3,6),
	// #5 (8,6) и #6 (4,7) в пределах видимости; крысы и культисты — нет.
	got := w.witnesses(Vec2{X: 2, Y: 2})
	want := []EntityID{1, 4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("свидетели %v, ожидается %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("свидетели %v, ожидается %v", got, want)
		}
	}
}

// TestWitnessesInEvents — события хода несут свидетелей: факт без очевидцев
// в этом мире не бывает, даже ожидание кто-нибудь да заметит.
func TestWitnessesInEvents(t *testing.T) {
	w := newParty(t)
	events, err := w.Step(Wait())
	if err != nil {
		t.Fatalf("Step(Wait): %v", err)
	}
	wit := events[0].Witnesses
	if len(wit) == 0 {
		t.Fatal("у события нет ни одного свидетеля")
	}
	for i := 1; i < len(wit); i++ {
		if wit[i-1] >= wit[i] {
			t.Fatalf("свидетели не отсортированы: %v", wit)
		}
	}
	if wit[0] != 1 {
		t.Errorf("актёр не включён в свидетели: %v", wit)
	}
}

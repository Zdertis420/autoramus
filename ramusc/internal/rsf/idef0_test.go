package rsf_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

func model(t *testing.T) *rsf.Model {
	t.Helper()
	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(f)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFunctions(t *testing.T) {
	got := model(t).Functions()

	want := []struct {
		name   string
		typ    int
		bounds rsf.Rect
	}{
		{"Изготовление юбки", rsf.TypeOperation, rsf.Rect{X: 288, Y: 147, Width: 186, Height: 144}},
		{"Раскрой материала", rsf.TypeProcess, rsf.Rect{X: 90, Y: 102, Width: 138, Height: 72}},
		{"Сшивание деталей", rsf.TypeProcess, rsf.Rect{X: 369, Y: 201, Width: 102, Height: 75}},
		{"Добавление фурнитуры", rsf.TypeProcess, rsf.Rect{X: 624, Y: 288, Width: 108, Height: 84}},
	}
	if len(got) != len(want) {
		t.Fatalf("работ %d, ожидалось %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		f := got[i]
		if f.Name != w.name {
			t.Errorf("работа %d называется %q, ожидалось %q", i, f.Name, w.name)
		}
		if f.Type != w.typ {
			t.Errorf("%s: тип %d, ожидался %d", w.name, f.Type, w.typ)
		}
		if f.Bounds == nil || *f.Bounds != w.bounds {
			t.Errorf("%s: границы %+v, ожидались %+v", w.name, f.Bounds, w.bounds)
		}
	}

	// Корневая работа лежит под элементом модели, остальные — под ней.
	root := got[0]
	if root.Parent == -1 {
		t.Error("у корневой работы должен быть родитель — элемент модели")
	}
	for _, f := range got[1:] {
		if f.Parent != root.ID {
			t.Errorf("%s: родитель %d, ожидался %d", f.Name, f.Parent, root.ID)
		}
	}
}

func TestStreams(t *testing.T) {
	streams := model(t).Streams()
	if len(streams) != 14 {
		t.Fatalf("потоков %d, ожидалось 14", len(streams))
	}
	names := make(map[string]bool, len(streams))
	for _, s := range streams {
		names[s.Name] = true
	}
	for _, want := range []string{"Ткань", "Фурнитура", "Юбка", "Правила изготовления швейного изделия"} {
		if !names[want] {
			t.Errorf("нет потока %q", want)
		}
	}
}

func TestSectors(t *testing.T) {
	sectors := model(t).Sectors()
	// 45 живых сегментов; ещё 6 строк помечены удалёнными и в модель не входят.
	if len(sectors) != 45 {
		t.Fatalf("секторов %d, ожидалось 45", len(sectors))
	}

	var onBorder, onFunction, withPoints int
	for _, s := range sectors {
		if s.Stream < 0 {
			t.Errorf("сектор %d ни к какому потоку не привязан", s.ID)
		}
		for _, end := range []*rsf.Border{s.Start, s.End} {
			switch {
			case end.OnFunction():
				onFunction++
			case end.OnBorder():
				onBorder++
			}
			if end != nil && end.TunnelSoft {
				t.Errorf("сектор %d: в этой модели туннелей быть не должно", s.ID)
			}
		}
		if len(s.Points) > 0 {
			withPoints++
		}
	}
	if onFunction == 0 || onBorder == 0 || withPoints == 0 {
		t.Errorf("концов на блоках %d, на краю листа %d, сегментов с точками %d",
			onFunction, onBorder, withPoints)
	}
}

// TestICOM — та самая сводка, по которой чинились примеры: у каждой работы
// должны быть все четыре стороны.
func TestICOM(t *testing.T) {
	m := model(t)
	icom := m.ICOM()

	for _, f := range m.Functions() {
		sides := icom[f.ID]
		if sides == nil {
			t.Errorf("%s: нет ни одной стрелки", f.Name)
			continue
		}
		for _, side := range []rsf.Side{rsf.SideLeft, rsf.SideTop, rsf.SideBottom, rsf.SideRight} {
			if len(sides.Side(side)) == 0 {
				t.Errorf("%s: пустая сторона %s", f.Name, side.ICOM())
			}
		}
	}

	root := m.Functions()[0]
	if got, want := len(icom[root.ID].Mechanism), 5; got != want {
		t.Errorf("у корневой работы %d механизмов, ожидалось %d: %v",
			got, want, icom[root.ID].Mechanism)
	}
	if got := icom[root.ID].Control; len(got) != 1 || got[0] != "Правила изготовления швейного изделия" {
		t.Errorf("управление корневой работы = %v", got)
	}
}

// TestEveryFixtureParses — разбор модели идёт по всему перечню. Подробные
// утверждения выше остаются привязаны к «Изготовлению юбки»: они описывают
// именно её содержимое. Здесь проверяется то, что верно для любой модели.
func TestEveryFixtureParses(t *testing.T) {
	for _, model := range fixtures.All() {
		t.Run(model.Name, func(t *testing.T) {
			f, err := rsf.Open(model.Path)
			if err != nil {
				t.Fatal(err)
			}
			m, err := rsf.NewModel(f)
			if err != nil {
				t.Fatal(err)
			}

			if m.FunctionQualifier == 0 {
				t.Error("квалификатор работ не найден")
			}
			if len(m.Functions()) == 0 {
				t.Error("в модели нет ни одной работы")
			}
			// Стороны ICOM не обязаны быть у каждой работы, но отображение
			// обязано строиться без паники на любой модели.
			if m.ICOM() == nil {
				t.Error("ICOM не собрался")
			}
		})
	}
}

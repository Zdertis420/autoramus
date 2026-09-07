package decompile_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

const fixture = "../../../examples/ИзготовлениеЮбки.rsf"

func decompiled(t *testing.T) *ir.Model {
	t.Helper()
	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := rsf.NewModel(f)
	if err != nil {
		t.Fatal(err)
	}
	m, err := decompile.Model(source)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestModelHeader(t *testing.T) {
	m := decompiled(t)
	if m.Name.Name != "Изготовление юбки" {
		t.Errorf("model = %q", m.Name.Name)
	}
	if m.Author != "Ножкин А.М." {
		t.Errorf("author = %q", m.Author)
	}
	if m.Page != "A4" {
		t.Errorf("page = %q", m.Page)
	}
	if len(m.Flows) != 14 {
		t.Errorf("потоков %d, ожидалось 14", len(m.Flows))
	}
}

// TestFunctionOrder — работы идут в порядке дерева: корень, затем дети
// по цепочке PREVIOUS_ELEMENT_ID.
func TestFunctionOrder(t *testing.T) {
	m := decompiled(t)
	want := []string{
		"Изготовление юбки",
		"Раскрой материала",
		"Сшивание деталей",
		"Добавление фурнитуры",
	}
	if len(m.Functions) != len(want) {
		t.Fatalf("работ %d, ожидалось %d", len(m.Functions), len(want))
	}
	for i, name := range want {
		if got := m.Functions[i].Name.Name; got != name {
			t.Errorf("работа %d = %q, ожидалась %q", i, got, name)
		}
	}

	root := m.Functions[0]
	if root.Of.Set() {
		t.Errorf("у корневой работы не должно быть of: %+v", root.Of)
	}
	for _, f := range m.Functions[1:] {
		if f.Of.Name != root.Name.Name {
			t.Errorf("%s: of = %q", f.Name.Name, f.Of.Name)
		}
	}
}

// TestICOMSurvives — ICOM корневой работы должен совпасть с тем, что лежит
// в файле; именно по этой таблице чинились примеры.
func TestICOMSurvives(t *testing.T) {
	m := decompiled(t)

	sides := map[string][]string{}
	for _, l := range m.Links {
		switch {
		case l.To.Set() && l.To.Name == "Изготовление юбки":
			sides[l.SideName()] = append(sides[l.SideName()], l.Flow.Name)
		case l.From.Set() && l.From.Name == "Изготовление юбки":
			sides["out"] = append(sides["out"], l.Flow.Name)
		}
	}

	want := map[string][]string{
		"in":      {"Ткань", "Фурнитура"},
		"control": {"Правила изготовления швейного изделия"},
		"mechanism": {"Швея-закройшица", "Раскройное оборудование", "Выкройка",
			"Швейная машинка", "Швеёно-вышивальгая машинка"},
		"out": {"Юбка"},
	}
	for side, expected := range want {
		got := sides[side]
		if len(got) != len(expected) {
			t.Errorf("%s: %v, ожидалось %v", side, got, expected)
			continue
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Errorf("%s[%d] = %q, ожидалось %q", side, i, got[i], expected[i])
			}
		}
	}
}

// TestLayout — координаты всех четырёх блоков и только выразимые ломаные,
// половина которых лежит на контекстной диаграмме.
func TestLayout(t *testing.T) {
	m := decompiled(t)
	if m.Layout == nil {
		t.Fatal("раскладка не заполнена")
	}
	if len(m.Layout.Functions) != 4 {
		t.Errorf("блоков в раскладке %d, ожидалось 4", len(m.Layout.Functions))
	}
	if len(m.Layout.Arrows) != 18 {
		t.Errorf("ломаных %d, ожидалось 18", len(m.Layout.Arrows))
	}

	var context int
	for _, a := range m.Layout.Arrows {
		if a.Context {
			context++
		}
		if len(a.Points) < 2 {
			t.Errorf("ломаная %s короче двух точек", a.Flow.Name)
		}
		if a.On.Name == "" {
			t.Errorf("ломаная %s не называет диаграмму", a.Flow.Name)
		}
	}
	if context != 9 {
		t.Errorf("сегментов контекстной диаграммы %d, ожидалось 9", context)
	}

	first := m.Layout.Functions[0]
	if first.Function.Name != "Изготовление юбки" || first.X.Val != 288 || first.Width.Val != 186 {
		t.Errorf("первый блок раскладки: %+v", first)
	}
}

// TestOrphanFlows — три потока в файле остались без стрелок после
// переименования: рядом живут их двойники в именительном падеже, у которых
// сегменты есть. Именно на эти три валидатор даёт unused_flow, и шапка
// декомпилированного документа обязана их назвать.
func TestOrphanFlows(t *testing.T) {
	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := rsf.NewModel(f)
	if err != nil {
		t.Fatal(err)
	}

	got := decompile.OrphanFlows(source)
	want := []string{"Раскройного оборудования", "Выкройки", "Швейной машинки"}
	if len(got) != len(want) {
		t.Fatalf("потоков без стрелок %d, ожидалось %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("поток %d = %q, ожидался %q", i, got[i], want[i])
		}
	}
}

func TestSkippedIsReported(t *testing.T) {
	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := rsf.NewModel(f)
	if err != nil {
		t.Fatal(err)
	}
	skipped, total := decompile.Skipped(source)
	if total != 45 || skipped != 27 {
		t.Errorf("пропущено %d из %d, ожидалось 27 из 45", skipped, total)
	}
}

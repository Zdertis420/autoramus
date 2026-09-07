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

// TestLayout — геометрия переносится целиком: координаты всех блоков и все
// 45 сегментов, сгруппированные по стрелкам. Раньше 27 из них записать было
// нечем, теперь узлы ветвления выражаются в языке.
func TestLayout(t *testing.T) {
	m := decompiled(t)
	if m.Layout == nil {
		t.Fatal("раскладка не заполнена")
	}
	if len(m.Layout.Functions) != 4 {
		t.Errorf("блоков в раскладке %d, ожидалось 4", len(m.Layout.Functions))
	}

	// Стрелок 11: у трёх потоков из четырнадцати нет ни одного сегмента.
	if len(m.Layout.Arrows) != 11 {
		t.Errorf("стрелок %d, ожидалось 11", len(m.Layout.Arrows))
	}

	var segments, context, dangling int
	kinds := map[string]int{}
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			segments++
			if s.Context {
				context++
			}
			if s.On.Name == "" {
				t.Errorf("сегмент потока «%s» не называет диаграмму", a.Flow.Name)
			}
			if len(s.Points) < 2 {
				t.Errorf("сегмент потока «%s» короче двух точек", a.Flow.Name)
			}
			for _, e := range []*ir.Endpoint{s.From, s.To} {
				if e == nil {
					dangling++
					continue
				}
				kinds[e.Kind()]++
			}
		}
	}

	if segments != 45 {
		t.Errorf("сегментов %d, ожидалось 45", segments)
	}
	if context != 9 {
		t.Errorf("сегментов контекстной диаграммы %d, ожидалось 9", context)
	}
	// Ровно та же раскладка концов, что видна в самом файле.
	want := map[string]int{ir.EndpointFunction: 26, ir.EndpointBorder: 35, ir.EndpointNode: 12}
	for kind, n := range want {
		if kinds[kind] != n {
			t.Errorf("концов вида %s: %d, ожидалось %d", kind, kinds[kind], n)
		}
	}
	if dangling != 17 {
		t.Errorf("висящих концов %d, ожидалось 17", dangling)
	}

	first := m.Layout.Functions[0]
	if first.Function.Name != "Изготовление юбки" || first.X.Val != 288 || first.Width.Val != 186 {
		t.Errorf("первый блок раскладки: %+v", first)
	}
}

// TestBranchingNodes — механизм «Швея-закройшица» ветвится на три работы,
// и это единственное, чем такое ветвление выражается: общий узел у сегментов.
func TestBranchingNodes(t *testing.T) {
	m := decompiled(t)

	var arrow *ir.ArrowLayout
	for _, a := range m.Layout.Arrows {
		if a.Flow.Name == "Швея-закройшица" {
			arrow = a
		}
	}
	if arrow == nil {
		t.Fatal("стрелка механизма не нашлась")
	}

	used := map[string]int{}
	for _, s := range arrow.Segments {
		for _, e := range []*ir.Endpoint{s.From, s.To} {
			if e.Kind() == ir.EndpointNode {
				used[e.Node.Name]++
			}
		}
	}
	if len(used) != 2 {
		t.Fatalf("узлов %d, ожидалось 2: %v", len(used), used)
	}
	for name, n := range used {
		if n < 2 {
			t.Errorf("узел %s упомянут %d раз: он ничего не сшивает", name, n)
		}
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

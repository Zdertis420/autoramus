package ir_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/syntax"
)

const doc = `
model: Изготовление юбки
flows: [Ткань, Юбка]

functions:
  - name: Изготовление юбки
    in: [Ткань]
    control: [Правила]
    mechanism: [Швея]
    out: [Юбка]

  - name: "  Раскрой   материала "
    of: Изготовление юбки
    out: [Юбка]

links:
  - flow: Юбка
    from: Раскрой материала
    to: Изготовление юбки
    side: control
`

func build(t *testing.T) *ir.Model {
	t.Helper()
	root, diags := syntax.Load([]byte(doc))
	if diags.HasErrors() {
		t.Fatalf("документ не разобрался: %+v", diags)
	}
	return ir.Build(root)
}

// TestBuildLowersICOM — обе формы записи связи дают одинаковые Link (Р7).
func TestBuildLowersICOM(t *testing.T) {
	m := build(t)

	// Четыре связи от сахара первой работы, одна от второй и одна явная.
	if got, want := len(m.Links), 6; got != want {
		t.Fatalf("связей %d, ожидалось %d", got, want)
	}

	sides := map[string]string{}
	for _, l := range m.Links {
		if l.To.Set() && l.To.Name == "Изготовление юбки" {
			sides[l.Flow.Name] = l.SideName()
		}
	}
	for flow, want := range map[string]string{
		"Ткань":   ir.SideIn,
		"Правила": ir.SideControl,
		"Швея":    ir.SideMechanism,
		"Юбка":    ir.SideControl, // из явной секции links
	} {
		if got := sides[flow]; got != want {
			t.Errorf("сторона потока «%s» = %q, ожидалась %q", flow, got, want)
		}
	}

	// У выхода конец — источник, стороны у него нет.
	var out *ir.Link
	for _, l := range m.Links {
		if l.Sugar && l.From.Set() && l.From.Name == "Изготовление юбки" {
			out = l
		}
	}
	if out == nil {
		t.Fatal("выход не опустился в связь")
	}
	if out.To.Set() {
		t.Errorf("у выхода не должно быть конца to: %+v", out.To)
	}
}

// TestBuildNormalizesNames — пробелы по краям и внутри имени схлопываются,
// регистр не трогается (Р4).
func TestBuildNormalizesNames(t *testing.T) {
	m := build(t)

	f := m.Function("Раскрой материала")
	if f == nil {
		t.Fatal("работа с нормализованным именем не нашлась")
	}
	if f.Name.Raw != "  Раскрой   материала " {
		t.Errorf("исходное написание потеряно: %q", f.Name.Raw)
	}
	if f.Name.Pos.Line == 0 || f.Name.Path == "" {
		t.Errorf("имя без позиции или пути: %+v", f.Name)
	}
	if f.Of.Name != "Изготовление юбки" {
		t.Errorf("родитель = %q", f.Of.Name)
	}
}

// TestBuildDefaults — умолчания вида и типа работы.
func TestBuildDefaults(t *testing.T) {
	m := build(t)
	f := m.Function("Изготовление юбки")
	if f == nil {
		t.Fatal("корневая работа не нашлась")
	}
	if got := f.KindName(); got != ir.KindIDEF0 {
		t.Errorf("kind = %q, ожидался %q", got, ir.KindIDEF0)
	}
	if got := f.TypeName(); got != ir.TypeProcess {
		t.Errorf("type = %q, ожидался %q", got, ir.TypeProcess)
	}
	if f.Of.Set() {
		t.Errorf("у корневой работы не должно быть of: %+v", f.Of)
	}
}

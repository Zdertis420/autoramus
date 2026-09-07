package decompile_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/syntax"
)

// icom собирает стороны работ так, как их видит IR: по связям.
func icom(m *ir.Model) map[string]map[string][]string {
	out := make(map[string]map[string][]string)
	side := func(name, key, flow string) {
		if out[name] == nil {
			out[name] = make(map[string][]string)
		}
		out[name][key] = append(out[name][key], flow)
	}
	for _, l := range m.Links {
		switch {
		case l.From.Set() && !l.To.Set():
			side(l.From.Name, "out", l.Flow.Name)
		case l.To.Set() && !l.From.Set():
			side(l.To.Name, l.SideName(), l.Flow.Name)
		}
	}
	return out
}

// reparse прогоняет напечатанный документ обратно через разбор и понижение.
func reparse(t *testing.T, text string) *ir.Model {
	t.Helper()
	root, diags := syntax.Load([]byte(text))
	if diags.HasErrors() {
		t.Fatalf("напечатанный документ не разбирается: %+v\n%s", diags, text)
	}
	return ir.Build(root)
}

// TestRoundTripThroughLanguage — главный тест: модель, проведённая через
// печать и обратный разбор, должна остаться той же. Он доказывает, что
// писатель и фронтенд обратны друг другу, а значит канонический вывод
// действительно можно скармливать компилятору.
func TestRoundTripThroughLanguage(t *testing.T) {
	source := decompiled(t)

	for _, format := range []struct {
		name  string
		write func(*bytes.Buffer) error
	}{
		{"YAML", func(b *bytes.Buffer) error { return decompile.WriteYAML(b, source, decompile.Options{}) }},
		{"JSON", func(b *bytes.Buffer) error { return decompile.WriteJSON(b, source) }},
	} {
		t.Run(format.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := format.write(&buf); err != nil {
				t.Fatal(err)
			}
			got := reparse(t, buf.String())

			if got.Name.Name != source.Name.Name || got.Author != source.Author || got.Page != source.Page {
				t.Errorf("шапка разошлась: %q %q %q", got.Name.Name, got.Author, got.Page)
			}
			if len(got.Flows) != len(source.Flows) {
				t.Fatalf("потоков %d, было %d", len(got.Flows), len(source.Flows))
			}
			for i := range source.Flows {
				if got.Flows[i].Name != source.Flows[i].Name {
					t.Errorf("поток %d = %q, был %q", i, got.Flows[i].Name, source.Flows[i].Name)
				}
			}

			if len(got.Functions) != len(source.Functions) {
				t.Fatalf("работ %d, было %d", len(got.Functions), len(source.Functions))
			}
			for i, f := range source.Functions {
				if got.Functions[i].Name.Name != f.Name.Name {
					t.Errorf("работа %d = %q, была %q", i, got.Functions[i].Name.Name, f.Name.Name)
				}
				if got.Functions[i].Of.Name != f.Of.Name {
					t.Errorf("%s: of = %q, был %q", f.Name.Name, got.Functions[i].Of.Name, f.Of.Name)
				}
			}

			wantICOM, gotICOM := icom(source), icom(got)
			for name, sides := range wantICOM {
				for key, flows := range sides {
					if strings.Join(gotICOM[name][key], "|") != strings.Join(flows, "|") {
						t.Errorf("%s.%s = %v, было %v", name, key, gotICOM[name][key], flows)
					}
				}
			}

			if got.Layout == nil {
				t.Fatal("раскладка потерялась")
			}
			if len(got.Layout.Functions) != len(source.Layout.Functions) ||
				len(got.Layout.Arrows) != len(source.Layout.Arrows) {
				t.Fatalf("раскладка: блоков %d, ломаных %d",
					len(got.Layout.Functions), len(got.Layout.Arrows))
			}
			for i, a := range source.Layout.Arrows {
				gotArrow := got.Layout.Arrows[i]
				if gotArrow.Flow.Name != a.Flow.Name {
					t.Errorf("стрелка %d: поток %q, был %q", i, gotArrow.Flow.Name, a.Flow.Name)
				}
				if len(gotArrow.Segments) != len(a.Segments) {
					t.Fatalf("стрелка «%s»: сегментов %d, было %d",
						a.Flow.Name, len(gotArrow.Segments), len(a.Segments))
				}
				for j, s := range a.Segments {
					gotSeg := gotArrow.Segments[j]
					if gotSeg.Context != s.Context || gotSeg.On.Name != s.On.Name {
						t.Errorf("«%s» сегмент %d: диаграмма %q/%v, была %q/%v",
							a.Flow.Name, j, gotSeg.On.Name, gotSeg.Context, s.On.Name, s.Context)
					}
					if len(gotSeg.Points) != len(s.Points) {
						t.Errorf("«%s» сегмент %d: точек %d, было %d",
							a.Flow.Name, j, len(gotSeg.Points), len(s.Points))
					}
					// Концы — самое хрупкое место: узлы сшивают ветвление,
					// и потеря вида конца рвёт стрелку.
					for _, end := range []struct {
						key       string
						got, want *ir.Endpoint
					}{{"from", gotSeg.From, s.From}, {"to", gotSeg.To, s.To}} {
						if end.got.Kind() != end.want.Kind() {
							t.Errorf("«%s» сегмент %d, конец %s: вид %q, был %q",
								a.Flow.Name, j, end.key, end.got.Kind(), end.want.Kind())
							continue
						}
						if end.want == nil {
							continue
						}
						if end.got.Function.Name != end.want.Function.Name ||
							end.got.Side != end.want.Side ||
							end.got.Border != end.want.Border ||
							end.got.Node.Name != end.want.Node.Name ||
							end.got.Tunnel != end.want.Tunnel {
							t.Errorf("«%s» сегмент %d, конец %s: %+v, был %+v",
								a.Flow.Name, j, end.key, end.got, end.want)
						}
					}
				}
			}
			for i, f := range source.Layout.Functions {
				if got.Layout.Functions[i].X.Val != f.X.Val || got.Layout.Functions[i].Width.Val != f.Width.Val {
					t.Errorf("блок %d: %+v, был %+v", i, got.Layout.Functions[i], f)
				}
			}
		})
	}
}

// TestYAMLQuoting — имена, которые YAML прочитал бы не как строки, обязаны
// уйти в кавычки. Иначе поток «123» станет числом, а «Да» рискует стать
// логическим значением у чужого разборщика.
func TestYAMLQuoting(t *testing.T) {
	m := &ir.Model{Name: ir.Ref{Name: "Модель", Raw: "Модель", Path: "/model"}}
	for i, name := range []string{"Да", "123", "- дефис", "с: двоеточием", "true", "null", " краевой пробел "} {
		m.Flows = append(m.Flows, ir.Ref{Name: name, Raw: name, Path: "/flows/" + string(rune('0'+i))})
	}
	m.Functions = []*ir.Function{{
		Name: ir.Ref{Name: "Модель", Raw: "Модель", Path: "/functions/0/name"},
		Path: "/functions/0",
	}}
	m.Index()

	var buf bytes.Buffer
	if err := decompile.WriteYAML(&buf, m, decompile.Options{}); err != nil {
		t.Fatal(err)
	}
	got := reparse(t, buf.String())

	if len(got.Flows) != len(m.Flows) {
		t.Fatalf("потоков %d, было %d\n%s", len(got.Flows), len(m.Flows), buf.String())
	}
	for i, want := range m.Flows {
		if got.Flows[i].Raw != want.Raw {
			t.Errorf("поток %d = %q, был %q", i, got.Flows[i].Raw, want.Raw)
		}
	}
}

// TestUnprintableSectionsAreRefused — классификаторы и raw писатель пока не
// умеет, и молчать об этом нельзя: потерянные данные хуже отказа.
func TestUnprintableSectionsAreRefused(t *testing.T) {
	m := &ir.Model{
		Name:        ir.Ref{Name: "Модель", Raw: "Модель", Path: "/model"},
		Classifiers: []*ir.Classifier{{Name: ir.Ref{Name: "Документы", Raw: "Документы"}}},
	}
	if err := decompile.WriteYAML(&bytes.Buffer{}, m, decompile.Options{}); err == nil {
		t.Error("классификаторы должны приводить к ошибке, а не теряться")
	}
}

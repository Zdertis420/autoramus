package decompile_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Работы адресуются по идентичности, а не по имени. Уникальность имён —
// свойство входного документа (Р4), а не файла Ramus: там имя может
// повторяться и отсутствовать. Раньше отображение строилось по имени, все
// безымянные работы сходились в одну ячейку, и связи четырёх разных работ
// накапливались на последней.
//
// Проверяется ниже уровня команды: testModel.rsf получает отказ по
// непереносимому содержимому, и документа для него не печатается.

// sourceOf открывает модель из перечня по имени.
func sourceOf(t *testing.T, name string) *rsf.Model {
	t.Helper()

	var path string
	for _, m := range fixtures.All() {
		if m.Name == name {
			path = m.Path
		}
	}
	if path == "" {
		t.Fatalf("в перечне нет модели %s", name)
	}

	f, err := rsf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	source, err := rsf.NewModel(f)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func decompiledByName(t *testing.T, name string) *ir.Model {
	t.Helper()
	model, err := decompile.Model(sourceOf(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return model
}

// TestLinksStayWithTheirFunction — связи безымянных работ не сходятся на одной.
func TestLinksStayWithTheirFunction(t *testing.T) {
	m := decompiledByName(t, "testModel")

	var unnamed int
	for _, f := range m.Functions {
		if f.Name.Name == "" {
			unnamed++
		}
	}
	if unnamed < 2 {
		t.Skipf("в модели %d безымянных работ, проверять нечего", unnamed)
	}

	var buf bytes.Buffer
	if err := decompile.WriteYAML(&buf, m, decompile.Options{}); err != nil {
		t.Fatal(err)
	}

	// Работа, собравшая связи всех остальных, узнаётся по раздутым спискам:
	// одна работа не может законно иметь вход, повторённый несколько раз.
	for _, side := range []string{"in:", "control:", "mechanism:", "out:"} {
		for _, line := range strings.Split(buf.String(), "\n") {
			if !strings.Contains(line, side) {
				continue
			}
			if flows := parseFlowList(line); hasDuplicate(flows) {
				t.Errorf("связи разных работ сошлись на одной: %s", strings.TrimSpace(line))
			}
		}
	}
}

// TestNoDuplicateICOM — один поток не называется дважды на одной стороне одной
// работы. Несколько секторов на один поток — это ветвление, а не несколько
// стрелок.
func TestNoDuplicateICOM(t *testing.T) {
	for _, f := range fixtures.All() {
		t.Run(f.Name, func(t *testing.T) {
			m := decompiledByName(t, f.Name)

			seen := make(map[string]map[string]bool)
			for _, l := range m.Links {
				owner, side := l.To, l.Side
				if !l.To.Set() {
					owner, side = l.From, "out"
				}
				if !owner.Set() {
					continue
				}
				key := owner.Path + "|" + side
				if seen[key] == nil {
					seen[key] = make(map[string]bool)
				}
				if seen[key][l.Flow.Name] {
					t.Errorf("работа %q, сторона %s: поток «%s» назван дважды",
						owner.Name, side, l.Flow.Name)
				}
				seen[key][l.Flow.Name] = true
			}
		})
	}
}

func parseFlowList(line string) []string {
	open := strings.Index(line, "[")
	close := strings.LastIndex(line, "]")
	if open < 0 || close < open {
		return nil
	}
	var out []string
	for _, part := range strings.Split(line[open+1:close], ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func hasDuplicate(items []string) bool {
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		if seen[it] {
			return true
		}
		seen[it] = true
	}
	return false
}

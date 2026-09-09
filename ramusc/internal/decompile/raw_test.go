package decompile_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// TestRawCarriesLosses — то, ради чего люк и задействован: цвета, статус и
// свойства подписи доезжают до документа, а не пропадают молча.
func TestRawCarriesLosses(t *testing.T) {
	m := decompiledByName(t, "ФормированиеТП")

	var buf bytes.Buffer
	if err := decompile.WriteYAML(&buf, m, decompile.Options{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"F_BACKGROUND", "F_STATUS", "F_FONT", "F_DECOMPOSITION_TYPE",
		"F_SECTOR_PROPERTIES", "F_PROJECT_PREFERENCES",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("атрибут %s не доехал через люк", want)
		}
	}
}

// TestRawOutputValidates — документ с люком принимается языком. Иначе люк
// превратил бы полезный вывод в непринимаемый.
func TestRawOutputValidates(t *testing.T) {
	for _, f := range fixtures.All() {
		if f.ExpectRefusal {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := decompile.WriteYAML(&buf, decompiledByName(t, f.Name), decompile.Options{}); err != nil {
				t.Fatal(err)
			}
			diags, err := validate.Source(buf.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if errors, _ := diags.Count(); errors != 0 {
				var text bytes.Buffer
				_ = diags.WriteText(&text, "вывод")
				t.Errorf("ошибок %d:\n%s", errors, text.String())
			}
		})
	}
}

// TestRawIsLastResort — атрибут, для которого у языка есть своё поле, в люк не
// попадает. Иначе одно и то же окажется записано дважды и однажды разойдётся.
func TestRawIsLastResort(t *testing.T) {
	m := decompiledByName(t, "ФормированиеТП")

	own := map[string]bool{
		"Название": true, "HierarchicalAttribute": true,
		"F_BOUNDS": true, "F_SECTOR_POINTS": true,
		"F_SECTOR_BORDER_START": true, "F_SECTOR_BORDER_END": true,
		"F_STREAM_NAME": true, "F_TYPE": true,
	}
	check := func(where string, raw []ir.RawAttr) {
		for _, a := range raw {
			if own[a.Attribute.Raw] {
				t.Errorf("%s: атрибут %s есть в полях языка, но попал в люк", where, a.Attribute.Raw)
			}
		}
	}

	check("модель", m.Raw)
	for _, f := range m.Functions {
		check("работа "+f.Name.Name, f.Raw)
	}
	if m.Layout != nil {
		for _, a := range m.Layout.Arrows {
			for _, s := range a.Segments {
				check("сегмент "+a.Flow.Name, s.Raw)
			}
		}
	}
}

// TestEmptyRawIsNotPrinted — люк есть у каждого элемента (Р17), и у
// большинства ему нечего нести. Печатать пустую секцию значило бы утопить
// документ в шуме.
func TestEmptyRawIsNotPrinted(t *testing.T) {
	m := &ir.Model{
		Name:      ir.Ref{Name: "Модель", Raw: "Модель", Path: "/model"},
		Functions: []*ir.Function{{Name: ir.Ref{Name: "Работа", Raw: "Работа"}}},
	}
	var buf bytes.Buffer
	if err := decompile.WriteYAML(&buf, m, decompile.Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "raw:") {
		t.Errorf("пустой люк напечатан:\n%s", buf.String())
	}
}

// TestRawSurvivesReparse — значение люка переживает обратное чтение, включая
// многоколоночное. Генератору предстоит разложить его по колонкам, и потеря
// имён колонок сделала бы это невозможным.
func TestRawSurvivesReparse(t *testing.T) {
	m := &ir.Model{
		Name: ir.Ref{Name: "Модель", Raw: "Модель", Path: "/model"},
		Raw: []ir.RawAttr{{
			Attribute: ir.Ref{Name: "F_PROJECT_PREFERENCES", Raw: "F_PROJECT_PREFERENCES"},
			Value:     map[string]any{"DEFINITION": "A4", "USED_AT": json.Number("3")},
		}},
		Functions: []*ir.Function{{
			Name: ir.Ref{Name: "Работа", Raw: "Работа"},
			Raw: []ir.RawAttr{{
				Attribute: ir.Ref{Name: "F_STATUS", Raw: "F_STATUS"},
				Value:     json.Number("0"),
			}},
		}},
	}

	var buf bytes.Buffer
	if err := decompile.WriteYAML(&buf, m, decompile.Options{}); err != nil {
		t.Fatal(err)
	}
	got := reparse(t, buf.String())

	if len(got.Raw) != 1 || got.Raw[0].Attribute.Raw != "F_PROJECT_PREFERENCES" {
		t.Fatalf("люк модели после разбора: %+v\n%s", got.Raw, buf.String())
	}
	value, ok := got.Raw[0].Value.(map[string]any)
	if !ok {
		t.Fatalf("многоколоночное значение стало %T, ожидалось отображение", got.Raw[0].Value)
	}
	if value["DEFINITION"] != "A4" {
		t.Errorf("колонка DEFINITION = %v, было A4", value["DEFINITION"])
	}
	if len(got.Functions) != 1 || len(got.Functions[0].Raw) != 1 {
		t.Fatalf("люк работы потерялся: %+v", got.Functions)
	}
}

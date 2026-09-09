package decompile_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// TestColumnTargetIsPrinted — у ссылочной колонки цель напечатана именем
// справочника, у обычной поля цели нет вовсе: валидатор считает лишнее поле
// ошибкой (unexpected_column_target).
func TestColumnTargetIsPrinted(t *testing.T) {
	m := &ir.Model{
		Name: ir.Ref{Name: "Модель", Raw: "Модель", Path: "/model"},
		Flows: []ir.Ref{
			{Name: "Ткань", Raw: "Ткань"}, {Name: "Правила", Raw: "Правила"},
			{Name: "Швея", Raw: "Швея"}, {Name: "Юбка", Raw: "Юбка"},
		},
		Functions: []*ir.Function{{
			Name: ir.Ref{Name: "Модель", Raw: "Модель", Path: "/functions/0/name"},
			Path: "/functions/0",
		}},
		Classifiers: []*ir.Classifier{
			{
				Name: ir.Ref{Name: "Материалы", Raw: "Материалы"},
				Columns: []*ir.Column{{
					Name: ir.Ref{Name: "Артикул", Raw: "Артикул"},
					Type: ir.Ref{Name: "text", Raw: "text"},
				}},
			},
			{
				Name: ir.Ref{Name: "Изделия", Raw: "Изделия"},
				Columns: []*ir.Column{
					{
						Name: ir.Ref{Name: "Материал", Raw: "Материал"},
						Type: ir.Ref{Name: "ref", Raw: "ref"},
						Of:   ir.Ref{Name: "Материалы", Raw: "Материалы"},
					},
					{
						Name: ir.Ref{Name: "Состав", Raw: "Состав"},
						Type: ir.Ref{Name: "list", Raw: "list"},
						Of:   ir.Ref{Name: "Материалы", Raw: "Материалы"},
					},
					{
						Name: ir.Ref{Name: "Название", Raw: "Название"},
						Type: ir.Ref{Name: "text", Raw: "text"},
					},
				},
			},
		},
	}

	// Работа должна быть полной, иначе валидатор ругается на неё, а не на
	// колонки, и проверка перестаёт проверять то, ради чего написана.
	name := m.Functions[0].Name
	for _, side := range []struct {
		flow, key string
	}{{"Ткань", ir.SideIn}, {"Правила", ir.SideControl}, {"Швея", ir.SideMechanism}} {
		m.Links = append(m.Links, &ir.Link{
			Flow: ir.Ref{Name: side.flow, Raw: side.flow}, To: name, Side: side.key, Sugar: true,
		})
	}
	m.Links = append(m.Links, &ir.Link{
		Flow: ir.Ref{Name: "Юбка", Raw: "Юбка"}, From: name, Sugar: true,
	})
	m.Index()

	var buf bytes.Buffer
	if err := decompile.WriteYAML(&buf, m, decompile.Options{}); err != nil {
		t.Fatal(err)
	}

	if n := strings.Count(buf.String(), "of: Материалы"); n != 2 {
		t.Errorf("полей цели %d, ожидалось два (ref и list):\n%s", n, buf.String())
	}

	// У обычной колонки цели быть не должно. Считаем поля of во всём документе:
	// их ровно столько, сколько ссылочных колонок.
	if n := strings.Count(buf.String(), "of:"); n != 2 {
		t.Errorf("полей of всего %d, а ссылочных колонок две", n)
	}

	diags, err := validate.Source(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Severity == "error" {
			t.Errorf("вывод не принимается языком: %+v", d)
		}
	}
}

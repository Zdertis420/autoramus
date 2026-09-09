package layout_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
)

// Секция layout — оверрайд (Р2): поправка автора поверх раскладки, а не
// соперник ей. Заданное автором побеждает целиком.

// pin закрепляет за работой геометрию, как это делает автор в документе.
func pin(m *ir.Model, name string, x, y, w, h float64) {
	if m.Layout == nil {
		m.Layout = &ir.Layout{Path: "/layout"}
	}
	m.Layout.Functions = append(m.Layout.Functions, &ir.FunctionLayout{
		Function: ir.Ref{Name: name},
		X:        ir.Num{Val: x, Set: true},
		Y:        ir.Num{Val: y, Set: true},
		Width:    ir.Num{Val: w, Set: true},
		Height:   ir.Num{Val: h, Set: true},
	})
}

// TestFullOverrideChangesNothing — документ с полной геометрией проходит
// раскладку без единого изменения.
//
// Из этого следует побайтовость: если бы раскладка переставляла записи или
// правила значения, файл после неё получился бы другим, и настоящие модели
// начали бы «дрейфовать» при каждой сборке.
func TestFullOverrideChangesNothing(t *testing.T) {
	m := model("корень", "первая", "вторая")
	pin(m, "корень", 1, 2, 3, 4)
	pin(m, "первая", 5, 6, 7, 8)
	pin(m, "вторая", 9, 10, 11, 12)

	before := make([]ir.FunctionLayout, len(m.Layout.Functions))
	for i, box := range m.Layout.Functions {
		before[i] = *box
	}

	layout.Apply(m)

	if len(m.Layout.Functions) != len(before) {
		t.Fatalf("записей стало %d, было %d: раскладка дописала лишнее",
			len(m.Layout.Functions), len(before))
	}
	for i, box := range m.Layout.Functions {
		if box.Function.Name != before[i].Function.Name {
			t.Errorf("запись %d: работа «%s», была «%s» — порядок переставлен",
				i, box.Function.Name, before[i].Function.Name)
		}
		if box.X.Val != before[i].X.Val || box.Y.Val != before[i].Y.Val ||
			box.Width.Val != before[i].Width.Val || box.Height.Val != before[i].Height.Val {
			t.Errorf("«%s»: (%v, %v, %v, %v), было (%v, %v, %v, %v)",
				box.Function.Name, box.X.Val, box.Y.Val, box.Width.Val, box.Height.Val,
				before[i].X.Val, before[i].Y.Val, before[i].Width.Val, before[i].Height.Val)
		}
	}
}

// TestPartialOverride — закреплённая работа остаётся на месте, остальные
// получают вычисленное.
//
// И главное: вычисленные координаты не зависят от того, что соседа закрепили.
// Иначе одна поправка сдвигала бы всю диаграмму, и автор терял бы возможность
// править по одному блоку.
func TestPartialOverride(t *testing.T) {
	free := model("корень", "первая", "вторая", "третья")
	layout.Apply(free)
	want := boxes(free)

	m := model("корень", "первая", "вторая", "третья")
	pin(m, "вторая", 400, 400, 40, 40)
	layout.Apply(m)
	got := boxes(m)

	fixed := got["вторая"]
	if fixed.X.Val != 400 || fixed.Y.Val != 400 || fixed.Width.Val != 40 || fixed.Height.Val != 40 {
		t.Errorf("закреплённая работа сдвинулась: (%v, %v, %v, %v)",
			fixed.X.Val, fixed.Y.Val, fixed.Width.Val, fixed.Height.Val)
	}

	for _, name := range []string{"первая", "третья"} {
		if got[name].X.Val != want[name].X.Val || got[name].Y.Val != want[name].Y.Val {
			t.Errorf("«%s» сдвинулась из-за закрепления соседа: (%v, %v), ожидалось (%v, %v)",
				name, got[name].X.Val, got[name].Y.Val, want[name].X.Val, want[name].Y.Val)
		}
	}
}

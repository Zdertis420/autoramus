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

// pinArrow закрепляет за потоком геометрию стрелки, как это делает автор.
func pinArrow(m *ir.Model, flow string, points ...float64) {
	if m.Layout == nil {
		m.Layout = &ir.Layout{Path: "/layout"}
	}
	seg := &ir.Segment{
		On:   ir.Ref{Name: "корень", Path: "/layout/arrows/0/segments/0/on"},
		From: &ir.Endpoint{Border: ir.BorderLeft},
		To:   &ir.Endpoint{Function: ir.Ref{Name: "первая"}, Side: ir.SideIn},
	}
	for i := 0; i+1 < len(points); i += 2 {
		seg.Points = append(seg.Points, ir.Point{
			X: ir.Num{Val: points[i], Set: true},
			Y: ir.Num{Val: points[i+1], Set: true},
		})
	}
	m.Layout.Arrows = append(m.Layout.Arrows, &ir.ArrowLayout{
		Flow:     ir.Ref{Name: flow},
		Segments: []*ir.Segment{seg},
	})
}

// TestArrowOverrideWins — геометрия стрелки, заданная автором, не трогается.
//
// Это то же правило Р2, что и у блоков, но для стрелок оно до сих пор ни на
// чём не проверялось: в языке геометрия стрелок была, а компилятор её не
// использовал.
func TestArrowOverrideWins(t *testing.T) {
	m := modelOf(t, example("skirt.yaml"))
	pinArrow(m, "Ткань", 7, 100, 200, 100)

	layout.Apply(m)

	arrow := arrowOf(t, m, "Ткань")
	if len(arrow.Segments) != 1 {
		t.Fatalf("сегментов %d, автор задал один: раскладка дописала своё", len(arrow.Segments))
	}
	points := arrow.Segments[0].Points
	if len(points) != 2 || points[0].X.Val != 7 || points[1].X.Val != 200 {
		t.Errorf("ломаная автора изменилась: %s", format(arrow.Segments[0]))
	}

	// Соседний поток при этом обязан быть разложен как обычно.
	if len(arrowOf(t, m, "Фурнитура").Segments) == 0 {
		t.Error("соседняя стрелка осталась без геометрии")
	}
}

// TestArrowOverrideKeepsOthers — закрепление одной стрелки не двигает
// остальные (SC-008).
func TestArrowOverrideKeepsOthers(t *testing.T) {
	free := modelOf(t, example("skirt.yaml"))
	layout.Apply(free)

	m := modelOf(t, example("skirt.yaml"))
	pinArrow(m, "Ткань", 7, 100, 200, 100)
	layout.Apply(m)

	for _, a := range free.Layout.Arrows {
		if a.Flow.Name == "Ткань" {
			continue
		}
		other := arrowOf(t, m, a.Flow.Name)
		if len(other.Segments) != len(a.Segments) {
			t.Errorf("«%s»: сегментов %d, было %d", a.Flow.Name, len(other.Segments), len(a.Segments))
			continue
		}
		for i := range a.Segments {
			if format(other.Segments[i]) != format(a.Segments[i]) {
				t.Errorf("«%s», сегмент %d: %s, было %s", a.Flow.Name, i,
					format(other.Segments[i]), format(a.Segments[i]))
			}
		}
	}
}

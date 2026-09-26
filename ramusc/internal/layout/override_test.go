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

// pinArrow закрепляет за потоком геометрию одного сегмента на названной
// диаграмме, как это делает автор. Пустое имя диаграммы — контекстная A-0.
func pinArrow(m *ir.Model, flow, diagram, to string, points ...float64) {
	if m.Layout == nil {
		m.Layout = &ir.Layout{Path: "/layout"}
	}
	seg := &ir.Segment{
		On:      ir.Ref{Name: diagram, Path: "/layout/arrows/0/segments/0/on"},
		Context: diagram == "",
		From:    &ir.Endpoint{Border: ir.BorderLeft},
		To:      &ir.Endpoint{Function: ir.Ref{Name: to}, Side: ir.SideIn},
	}
	if seg.Context {
		seg.On = ir.Ref{Name: m.Name.Name, Path: seg.On.Path}
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
	pinArrow(m, "Ткань", "Изготовление юбки", "Раскрой материала", 7, 100, 200, 100)

	layout.Apply(m)

	arrow := arrowOf(t, m, "Ткань")
	authored := segmentOn(t, arrow, "Изготовление юбки")
	points := authored.Points
	if len(points) != 2 || points[0].X.Val != 7 || points[1].X.Val != 200 {
		t.Errorf("ломаная автора изменилась: %s", format(authored))
	}

	// Соседний поток при этом обязан быть разложен как обычно.
	if len(arrowOf(t, m, "Фурнитура").Segments) == 0 {
		t.Error("соседняя стрелка осталась без геометрии")
	}
}

// TestArrowOverrideKeepsOtherDiagrams — стрелка, описанная автором на одной
// диаграмме, остаётся разложенной на всех остальных (SC-002).
//
// Прежде гранулярностью оверрайда был поток целиком по всей модели, и три
// сегмента, описанные для A0, стирали сегмент на A-0. Стрелка пропадала из
// файла молча, а Ramus рисовал на её месте туннельные скобки: у граничного
// конца не оставалось пары на родительской диаграмме.
func TestArrowOverrideKeepsOtherDiagrams(t *testing.T) {
	free := modelOf(t, example("skirt.yaml"))
	layout.Apply(free)
	want := segmentOn(t, arrowOf(t, free, "Ткань"), "")

	m := modelOf(t, example("skirt.yaml"))
	pinArrow(m, "Ткань", "Изготовление юбки", "Раскрой материала", 7, 100, 200, 100)
	layout.Apply(m)

	got := segmentOn(t, arrowOf(t, m, "Ткань"), "")
	if format(got) != format(want) {
		t.Errorf("сегмент на A-0: %s, без оверрайда был %s", format(got), format(want))
	}

	// И он именно дописан, а не подменил авторский: сегментов стало два.
	if n := len(arrowOf(t, m, "Ткань").Segments); n != 2 {
		t.Errorf("сегментов %d, ожидалось два — авторский на A0 и дописанный на A-0", n)
	}
}

// TestArrowOverrideKeepsOthers — закрепление одной стрелки не двигает
// остальные (SC-008).
func TestArrowOverrideKeepsOthers(t *testing.T) {
	free := modelOf(t, example("skirt.yaml"))
	layout.Apply(free)

	m := modelOf(t, example("skirt.yaml"))
	pinArrow(m, "Ткань", "Изготовление юбки", "Раскрой материала", 7, 100, 200, 100)
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

// TestAuthoredFlowFullyDrawn — у потока, часть которого автор нарисовал сам,
// в файл едут все его связи, а не только нарисованные (SC-002).
//
// `skirt-full.yaml` описывает «Правила изготовления» только на A0: вход с края
// и две ветки. Связей же у потока пять — три управления на A0, вход на A0 и
// управление корневой работы на A-0. Прежде в файл ехали три сегмента из пяти.
func TestAuthoredFlowFullyDrawn(t *testing.T) {
	m := modelOf(t, example("skirt-full.yaml"))
	layout.Apply(m)

	arrow := arrowOf(t, m, "Правила изготовления")

	// Считаем не сегменты, а точки крепления к блокам: сегменты дерева между
	// узлами связей не несут, и сравнивать их с числом связей нечего.
	attached := make(map[string]bool)
	for _, s := range arrow.Segments {
		diagram := s.On.Name
		if s.Context {
			diagram = "" // контекстная A-0
		}
		for _, e := range []*ir.Endpoint{s.From, s.To} {
			if e != nil && e.Function.Set() {
				attached[diagram+"/"+e.Function.Name+"/"+e.Side] = true
			}
		}
	}

	want := []string{
		"/Изготовление юбки/control", // A-0: то, что пропадало
		"Изготовление юбки/Раскрой материала/control",
		"Изготовление юбки/Сшивание деталей/control",
		"Изготовление юбки/Добавление фурнитуры/control",
	}
	for _, w := range want {
		if !attached[w] {
			t.Errorf("нет крепления «%s»: стрелка не доехала", w)
		}
	}
	if len(attached) != len(want) {
		t.Errorf("креплений %d, ожидалось %d: %v", len(attached), len(want), attached)
	}
}

// TestPartialOverrideFillsSize — что автор не задал, раскладка дописывает
// (research Р-8).
//
// Схема разрешает положение без размера. Прежде такой блок пропускался
// целиком, как авторский, и уходил в файл шириной и высотой 0 — не решение
// автора, а дефект. Заданное же автором не трогается, даже если стрелкам на
// нём тесно.
func TestPartialOverrideFillsSize(t *testing.T) {
	m := model("корень", "первая", "вторая", "третья")
	crowd(m, "первая", ir.SideIn, 3)
	crowd(m, "вторая", ir.SideIn, 3)
	if m.Layout == nil {
		m.Layout = &ir.Layout{Path: "/layout"}
	}
	m.Layout.Functions = append(m.Layout.Functions, &ir.FunctionLayout{
		Function: ir.Ref{Name: "первая"},
		X:        ir.Num{Val: 100, Set: true},
		Y:        ir.Num{Val: 120, Set: true},
	})
	pin(m, "вторая", 400, 300, 40, 40)
	layout.Apply(m)
	got := boxes(m)

	first := got["первая"]
	if first.X.Val != 100 || first.Y.Val != 120 {
		t.Errorf("положение, заданное автором, сдвинулось: (%v, %v)", first.X.Val, first.Y.Val)
	}
	if first.Width.Val != 72 || first.Height.Val != 60 {
		t.Errorf("недостающий размер %v × %v, ожидался 72 × 60 — по трём входам",
			first.Width.Val, first.Height.Val)
	}

	second := got["вторая"]
	if second.Width.Val != 40 || second.Height.Val != 40 {
		t.Errorf("размер, заданный автором, изменился: %v × %v", second.Width.Val, second.Height.Val)
	}
}

// TestAutoBlocksAvoidAuthored — блок, который ставит раскладка, не ложится на
// блок, поставленный автором (specs/014-arrows-avoid-blocks, FR-010).
//
// Прежде лестница расставлялась так, будто авторских блоков нет, и в
// skirt-full «Добавление фурнитуры» легло на «Сшивание деталей»: стрелкам
// между ними пройти было негде. Авторский блок не двигается; авто-блок, на
// который ничто не налезает, остаётся на своей ступени.
func TestAutoBlocksAvoidAuthored(t *testing.T) {
	check := func(t *testing.T, m *ir.Model) {
		t.Helper()
		pinned := make(map[string]ir.FunctionLayout)
		for _, f := range m.Layout.Functions {
			pinned[f.Function.Name] = *f
		}
		layout.Apply(m)
		for owner, blocks := range blocksByDiagram(m) {
			for _, a := range blocks {
				if _, authored := pinned[a.Function.Name]; authored {
					continue
				}
				for _, b := range blocks {
					if _, authored := pinned[b.Function.Name]; !authored {
						continue
					}
					if a.X.Val < b.X.Val+b.Width.Val && b.X.Val < a.X.Val+a.Width.Val &&
						a.Y.Val < b.Y.Val+b.Height.Val && b.Y.Val < a.Y.Val+a.Height.Val {
						t.Errorf("диаграмма «%s»: «%s» лежит на авторском «%s»",
							owner, a.Function.Name, b.Function.Name)
					}
				}
			}
		}
		for name, was := range pinned {
			now := boxes(m)[name]
			if now.X.Val != was.X.Val || now.Y.Val != was.Y.Val {
				t.Errorf("авторский «%s» сдвинулся: (%v, %v) → (%v, %v)",
					name, was.X.Val, was.Y.Val, now.X.Val, now.Y.Val)
			}
		}
	}

	t.Run("skirt-full.yaml", func(t *testing.T) {
		check(t, modelOf(t, example("skirt-full.yaml")))
	})

	t.Run("авторский блок на чужой ступени", func(t *testing.T) {
		free := model("корень", "первая", "вторая", "третья")
		layout.Apply(free)
		want := boxes(free)

		// «Первая» поставлена автором туда, где лестница ставит «вторую».
		m := model("корень", "первая", "вторая", "третья")
		pin(m, "первая", want["вторая"].X.Val, want["вторая"].Y.Val, 72, 50.4)
		check(t, m)

		// «Третья» не задета — стоит на своей ступени.
		if got := boxes(m)["третья"]; got.X.Val != want["третья"].X.Val || got.Y.Val != want["третья"].Y.Val {
			t.Errorf("«третья» сдвинулась, хотя ей ничто не мешало: (%v, %v) вместо (%v, %v)",
				got.X.Val, got.Y.Val, want["третья"].X.Val, want["третья"].Y.Val)
		}
	})
}

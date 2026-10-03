package layout_test

import (
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
)

// Ветвление. Рисунок сверяется с `examples/тест.rsf`, где «контроль» приходит
// сверху к четырём блокам: один вход с края, цепочка из n−1 узлов вдоль общей
// горизонтали, отвод из каждого узла в свой блок и дотяжка последнего.
// Сегментов выходит 2n−1.

// branchSegments отдаёт сегменты стрелки на диаграмме-декомпозиции: контекстная
// A-0 зовётся так же, и отличить её можно только пометкой Context.
func branchSegments(a *ir.ArrowLayout, diagram string) []*ir.Segment {
	var out []*ir.Segment
	for _, s := range a.Segments {
		if s.On.Name == diagram && !s.Context {
			out = append(out, s)
		}
	}
	return out
}

// TestTreeShape — рисунок дерева совпадает с «тестом».
//
// `staircase.yaml` — четыре подработы цепочкой, то есть структура `тест.rsf`, и
// поток там зовётся так же. «Контроль» приходит управлением ко всем четырём,
// и дерево обязано выйти тем же: семь сегментов, один вход, три узла.
func TestTreeShape(t *testing.T) {
	m := modelOf(t, document("staircase.yaml"))
	layout.Apply(m)

	segments := branchSegments(arrowOf(t, m, "контроль"), "Работа")
	const consumers = 4
	if got, want := len(segments), 2*consumers-1; got != want {
		t.Fatalf("сегментов %d, ожидалось %d — столько же, сколько у «контроля» в тест.rsf", got, want)
	}

	var entries, nodes, drops int
	seen := make(map[string]bool)
	for _, s := range segments {
		if s.From.Kind() == ir.EndpointBorder {
			entries++
		}
		for _, e := range []*ir.Endpoint{s.From, s.To} {
			if e.Kind() == ir.EndpointNode {
				seen[e.Node.Name] = true
			}
		}
		if s.To.Kind() == ir.EndpointFunction {
			drops++
		}
	}
	nodes = len(seen)

	if entries != 1 {
		t.Errorf("край листа пересекают %d сегмента, а должен один (FR-002)", entries)
	}
	if nodes != consumers-1 {
		t.Errorf("узлов %d, ожидалось %d", nodes, consumers-1)
	}
	if drops != consumers {
		t.Errorf("отводов в блоки %d, потребителей %d — до кого-то не дошли", drops, consumers)
	}
}

// TestTreeNodesAreOnePoint — сегменты, делящие узел, сходятся в одной точке.
//
// Это тот же инвариант, что проверяется на готовом файле (generate), но здесь
// он снимается с раскладки: если узел разъехался, видно сразу, в каком месте, а
// не после сборки .rsf.
func TestTreeNodesAreOnePoint(t *testing.T) {
	for _, path := range documents() {
		t.Run(path, func(t *testing.T) {
			m := modelOf(t, path)
			written := authored(m)
			layout.Apply(m)

			for _, a := range m.Layout.Arrows {
				// Ключ — диаграмма и имя узла: одно имя на разных диаграммах
				// означает сшивку уровней, а не одну точку.
				at := make(map[string][2]float64)
				for _, s := range a.Segments {
					if written[authoredKey(a.Flow.Name, s)] {
						continue // геометрию писал автор, форма не наша
					}
					ends := []struct {
						e *ir.Endpoint
						p ir.Point
					}{
						{s.From, s.Points[0]},
						{s.To, s.Points[len(s.Points)-1]},
					}
					for _, end := range ends {
						if end.e.Kind() != ir.EndpointNode {
							continue
						}
						key := s.On.Name + "\x00" + end.e.Node.Name
						got := [2]float64{end.p.X.Val, end.p.Y.Val}
						if was, seen := at[key]; seen && was != got {
							t.Errorf("«%s», узел %s: точки %v и %v — узел обязан быть одной точкой (FR-004)",
								a.Flow.Name, end.e.Node.Name, was, got)
						}
						at[key] = got
					}
				}
			}
		})
	}
}

// TestNoNodesWithoutBranching — ветвить нечего, узлов не появляется (FR-012).
//
// «Ткань» в «Юбке» приходит к одной работе на каждой диаграмме. Дерево из
// одной ветви — это просто линия, и узел на ней был бы лишней точкой, которой
// нет и у Ramus.
func TestNoNodesWithoutBranching(t *testing.T) {
	m := modelOf(t, example("skirt.yaml"))
	layout.Apply(m)

	for _, s := range arrowOf(t, m, "Ткань").Segments {
		for _, e := range []*ir.Endpoint{s.From, s.To} {
			if e.Kind() == ir.EndpointNode {
				t.Errorf("«Ткань» не ветвится, а узел %s ей поставлен", e.Node.Name)
			}
		}
	}
}

// TestTreeIsDeterministic — две раскладки одного документа дают одно дерево.
//
// Узлы раздаются обходом групп, а обход отображения в Go случаен: если бы
// порядок протёк наружу, две сборки дали бы разные файлы (принцип III).
func TestTreeIsDeterministic(t *testing.T) {
	first := modelOf(t, example("skirt.yaml"))
	layout.Apply(first)
	second := modelOf(t, example("skirt.yaml"))
	layout.Apply(second)

	if len(first.Layout.Arrows) != len(second.Layout.Arrows) {
		t.Fatalf("стрелок %d и %d", len(first.Layout.Arrows), len(second.Layout.Arrows))
	}
	for i, a := range first.Layout.Arrows {
		b := second.Layout.Arrows[i]
		if a.Flow.Name != b.Flow.Name || len(a.Segments) != len(b.Segments) {
			t.Fatalf("стрелка %d разошлась: «%s»/%d против «%s»/%d",
				i, a.Flow.Name, len(a.Segments), b.Flow.Name, len(b.Segments))
		}
		for j, s := range a.Segments {
			o := b.Segments[j]
			if s.From.Node.Name != o.From.Node.Name || s.To.Node.Name != o.To.Node.Name {
				t.Errorf("«%s», сегмент %d: узлы разошлись", a.Flow.Name, j)
			}
			for k, p := range s.Points {
				q := o.Points[k]
				if !near(p.X.Val, q.X.Val) || !near(p.Y.Val, q.Y.Val) {
					t.Errorf("«%s», сегмент %d, точка %d: (%g,%g) против (%g,%g)",
						a.Flow.Name, j, k, p.X.Val, p.Y.Val, q.X.Val, q.Y.Val)
				}
			}
		}
	}
}

// turns считает повороты внутри одной ломаной.
//
// Поворот внутри ломаной Ramus скругляет, поворот на стыке двух секторов — нет.
// Считается ради меры, а не ради требования: доля зависит от того, сколько в
// модели ветвлений против простых связей, и сравнивать её между моделями нельзя.
func turns(s *ir.Segment) int {
	n := 0
	for i := 1; i+1 < len(s.Points); i++ {
		a, b, c := s.Points[i-1], s.Points[i], s.Points[i+1]
		before := near(a.X.Val, b.X.Val) // отрезок вертикален
		after := near(b.X.Val, c.X.Val)
		if before != after {
			n++
		}
	}
	return n
}

// pointsOf печатает ломаную для сообщения об ошибке.
func pointsOf(s *ir.Segment) [][2]float64 {
	out := make([][2]float64, 0, len(s.Points))
	for _, p := range s.Points {
		out = append(out, [2]float64{p.X.Val, p.Y.Val})
	}
	return out
}

// TestTurnsAreInsidePolylines — счёт поворотов внутри ломаных и на стыках.
//
// Считает то же, что `evidence/turns.py` снаружи компилятора, и служит для
// сверки с ним: расхождение значит, что раскладка думает одно, а в файл
// доезжает другое.
//
// Числа не закрепляются порогом. Доля зависит от того, сколько в модели
// ветвлений против простых связей, и сравнивать её между моделями нельзя;
// требование фичи — структурное, оно в TestBranchCarriesItsOwnCorner.
func TestTurnsAreInsidePolylines(t *testing.T) {
	m := modelOf(t, example("chakhokhbili.yaml"))
	layout.Apply(m)

	inside, joints := 0, 0
	for _, a := range m.Layout.Arrows {
		// Направление каждого конца сегмента, сложенное по точкам: если в
		// одной точке сходятся и горизонталь, и вертикаль, это поворот на
		// стыке.
		dirs := make(map[[3]interface{}]map[bool]bool)
		for _, s := range a.Segments {
			inside += turns(s)
			if len(s.Points) < 2 {
				continue
			}
			for _, end := range [2]struct{ at, next ir.Point }{
				{s.Points[0], s.Points[1]},
				{s.Points[len(s.Points)-1], s.Points[len(s.Points)-2]},
			} {
				key := [3]interface{}{diagramOf(s), end.at.X.Val, end.at.Y.Val}
				if dirs[key] == nil {
					dirs[key] = make(map[bool]bool)
				}
				dirs[key][near(end.at.X.Val, end.next.X.Val)] = true
			}
		}
		for _, d := range dirs {
			if len(d) > 1 {
				joints++
			}
		}
	}
	t.Logf("поворотов внутри ломаных %d, на стыках %d", inside, joints)
}

// TestSimpleArrowsKeepTurnsInside — стрелка без ветвления поворачивает внутри
// себя.
//
// Переделывать здесь было нечего: измерение до фичи показало, что прямые связи,
// коридоры обратной связи и граничные стрелки без ветвления уже дают все свои
// повороты внутри ломаных и ни одного на стыке. Тест закрепляет это, чтобы
// перестройка дерева не испортила соседей, которых не трогала.
//
// Стрелка без ветвления — та, у которой нет ни одного конца-узла: её сегменты
// друг с другом не стыкуются, и стыковому повороту взяться неоткуда.
func TestSimpleArrowsKeepTurnsInside(t *testing.T) {
	for _, path := range documents() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m := modelOf(t, path)
			written := authored(m)
			layout.Apply(m)

			for _, a := range m.Layout.Arrows {
				branching := false
				for _, s := range a.Segments {
					for _, e := range []*ir.Endpoint{s.From, s.To} {
						if e.Kind() == ir.EndpointNode {
							branching = true
						}
					}
				}
				if branching {
					continue
				}
				for _, s := range a.Segments {
					if written[authoredKey(a.Flow.Name, s)] {
						continue
					}
					if len(s.Points) < 2 {
						t.Errorf("«%s»: сегмент из %d точек", a.Flow.Name, len(s.Points))
					}
					// Ломаная есть ломаная: повороты внутри неё законны, а
					// стыковаться ей не с чем.
					if turns(s) != len(s.Points)-2 && len(s.Points) > 2 {
						t.Logf("«%s»: %d поворотов на %d точек — есть коллинеарные звенья",
							a.Flow.Name, turns(s), len(s.Points))
					}
				}
			}
		})
	}
}

// TestStraightBorderArrowStaysOneSegment — прямая граничная стрелка осталась
// одним отрезком (FR-005).
//
// Поток, идущий от края листа прямо в работу, поворачивать не должен, и лишних
// точек у него быть не может: два конца, и всё. Проверяется на
// `two-sides.yaml`, где такие стрелки есть на каждой стороне.
func TestStraightBorderArrowStaysOneSegment(t *testing.T) {
	m := modelOf(t, document("two-sides.yaml"))
	layout.Apply(m)

	straight := 0
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			if s.From.Kind() != ir.EndpointBorder || s.To.Kind() != ir.EndpointFunction {
				continue
			}
			if turns(s) > 0 {
				continue // стрелка обходит блок — это другой случай
			}
			straight++
			if len(s.Points) != 2 {
				t.Errorf("«%s»: прямая граничная стрелка из %d точек, ожидалось 2: %v",
					a.Flow.Name, len(s.Points), pointsOf(s))
			}
		}
	}
	if straight == 0 {
		t.Fatal("в документе нет прямых граничных стрелок — проверять нечего")
	}
}

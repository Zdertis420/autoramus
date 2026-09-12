package layout_test

import (
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
				if written[a.Flow.Name] {
					continue // геометрию писал автор, форма не наша
				}
				// Ключ — диаграмма и имя узла: одно имя на разных диаграммах
				// означает сшивку уровней, а не одну точку.
				at := make(map[string][2]float64)
				for _, s := range a.Segments {
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

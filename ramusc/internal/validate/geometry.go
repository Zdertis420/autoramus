package validate

import (
	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Geometry проверяет разложенную модель: не идёт ли стрелка, нарисованная
// автором, сквозь блок своей диаграммы (specs/014-arrows-avoid-blocks).
//
// Проверяются только авторские сегменты: то, что проложила раскладка, сквозь
// блоки не идёт по построению, и сторожит это тест, а не диагностика. Авторское
// раскладка не трогает (Р2), и сказать автору можно только предупреждением —
// одним на сегмент, в позиции точки, с которой начинается первый отрезок,
// идущий сквозь блок.
func Geometry(m *ir.Model, authored map[*ir.Segment]bool) diag.List {
	if m == nil || m.Layout == nil || len(authored) == 0 {
		return nil
	}

	// Блоки по диаграммам. Диаграмма зовётся работой-владельцем; у
	// контекстной A-0 владельца-работы нет, и она зовётся пустой строкой —
	// так же, как в раскладке.
	owner := make(map[string]string, len(m.Functions))
	for _, f := range m.Functions {
		owner[f.Name.Name] = f.Of.Name
	}
	blocks := make(map[string][]*ir.FunctionLayout)
	for _, f := range m.Layout.Functions {
		if d, ok := owner[f.Function.Name]; ok {
			blocks[d] = append(blocks[d], f)
		}
	}

	var out diag.List
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			if !authored[s] {
				continue
			}
			diagram, title := s.On.Name, s.On.Name
			if s.Context {
				diagram = ""
			}
			if d := throughBlock(s, blocks[diagram]); d != nil {
				out = append(out, diag.NewWarning(diag.CodeArrowThroughBlock, d.point.Pos, d.point.Path,
					"стрелка «%s» на диаграмме «%s» проходит сквозь работу «%s»: авторские точки не правятся, но линия читается как вход или выход этой работы",
					a.Flow.Name, title, d.block))
			}
		}
	}
	return out
}

// crossing — первый отрезок сегмента, идущий сквозь блок.
type crossing struct {
	point ir.Point // точка, с которой начинается отрезок
	block string
}

// throughBlock ищет первый отрезок сегмента, проходящий внутри блока.
// Касание стороны — не пересечение: точка крепления лежит ровно на ней.
func throughBlock(s *ir.Segment, blocks []*ir.FunctionLayout) *crossing {
	const eps = 1e-6
	for i := 1; i < len(s.Points); i++ {
		a, b := s.Points[i-1], s.Points[i]
		x1, x2 := min(a.X.Val, b.X.Val), max(a.X.Val, b.X.Val)
		y1, y2 := min(a.Y.Val, b.Y.Val), max(a.Y.Val, b.Y.Val)
		for _, f := range blocks {
			if x2 > f.X.Val+eps && x1 < f.X.Val+f.Width.Val-eps &&
				y2 > f.Y.Val+eps && y1 < f.Y.Val+f.Height.Val-eps {
				return &crossing{point: a, block: f.Function.Name}
			}
		}
	}
	return nil
}

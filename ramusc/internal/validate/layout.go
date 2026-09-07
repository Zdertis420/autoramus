package validate

import (
	"math"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// checkLayout проверяет ручной оверрайд раскладки (Р2). Сама геометрия — дело
// автораскладки, здесь проверяется только то, что записи ссылаются на
// существующее и что числа — числа.
func (c *checker) checkLayout() {
	l := c.m.Layout
	if l == nil {
		return
	}

	seen := make(map[string]ir.Ref, len(l.Functions))
	for _, fl := range l.Functions {
		if f := c.function(fl.Function, "имя работы"); f != nil {
			if first, dup := seen[fl.Function.Name]; dup {
				c.add(diag.New(diag.CodeDuplicateLayout, fl.Function.Pos, fl.Function.Path,
					"раскладка работы «%s» уже задана в строке %d",
					fl.Function.Name, first.Pos.Line))
			} else {
				seen[fl.Function.Name] = fl.Function
			}
		}
		c.checkCoordinate(fl.X, "x")
		c.checkCoordinate(fl.Y, "y")
		c.checkSize(fl.Width, "width")
		c.checkSize(fl.Height, "height")
	}

	for _, a := range l.Arrows {
		c.flow(a.Flow, "имя потока")

		// Диаграмма зовётся по своей работе, а контекстная — по имени модели.
		if c.name(a.On, "имя диаграммы") && a.On.Name != c.m.Name.Name {
			c.function(a.On, "имя диаграммы")
		}
		if a.From.Set() {
			c.function(a.From, "имя работы-источника")
		}
		if a.To.Set() {
			c.function(a.To, "имя работы-приёмника")
		}

		for _, p := range a.Points {
			c.checkCoordinate(p.X, "x")
			c.checkCoordinate(p.Y, "y")
		}
	}
}

func (c *checker) checkCoordinate(n ir.Num, what string) {
	if !n.Set || finite(n.Val) {
		return
	}
	c.add(diag.New(diag.CodeBadGeometry, n.Pos, n.Path,
		"координата %s должна быть обычным числом", what))
}

func (c *checker) checkSize(n ir.Num, what string) {
	if !n.Set {
		return
	}
	if !finite(n.Val) || n.Val <= 0 {
		c.add(diag.New(diag.CodeBadGeometry, n.Pos, n.Path,
			"%s должен быть положительным числом", what))
	}
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

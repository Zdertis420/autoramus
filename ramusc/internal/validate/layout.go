package validate

import (
	"math"
	"strings"

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
		c.checkSegments(a)
		c.checkCoverage(a)
	}
}

// checkCoverage требует, чтобы нарисованное автором покрывало объявленное им же.
//
// Единица оверрайда — пара «поток + диаграмма»: раскладка не трогает поток на
// той диаграмме, где автор описал хоть один его сегмент. Значит на такой
// диаграмме за все связи потока отвечает автор, и связь, которую он не нарисовал,
// просто не попадёт в файл. Молчать об этом нельзя — стрелка пропадала бы ровно
// так же, как пропадала прежде из-за оверрайда на весь поток.
//
// Мера покрытия — точка крепления, а не связь: у потребителя должен быть
// сегмент, приходящий к нему нужной стороной, у производителя — сегмент,
// выходящий из него. Так проверка не пересказывает правила сборки стрелок из
// раскладки и не расходится с ними при следующей правке.
func (c *checker) checkCoverage(a *ir.ArrowLayout) {
	flow := a.Flow.Name
	if flow == "" {
		return
	}

	// Диаграммы, на которых автор взялся рисовать этот поток. Контекстная
	// зовётся пустой строкой — так же, как её называет c.diagram.
	authored := make(map[string]bool)
	for _, s := range a.Segments {
		if s.Context {
			authored[""] = true
			continue
		}
		authored[s.On.Name] = true
	}

	// Что автор нарисовал: концы, прицепленные к блокам.
	type attachment struct{ diagram, function, side string }
	covered := make(map[attachment]bool)
	for _, s := range a.Segments {
		diagram := s.On.Name
		if s.Context {
			diagram = ""
		}
		for _, e := range []*ir.Endpoint{s.From, s.To} {
			if e == nil || e.Kind() != ir.EndpointFunction {
				continue
			}
			covered[attachment{diagram, e.Function.Name, e.Side}] = true
		}
	}

	for _, f := range c.m.Functions {
		if !c.canonical(f) {
			continue
		}
		diagram, ok := c.diagram(f)
		if !ok || !authored[diagram] {
			continue
		}
		name := f.Name.Name

		// Работа, которая сама производит и сама потребляет поток, стрелки не
		// даёт вовсе: рисовать петлю из блока в него же раскладка не берётся.
		if c.outputs[name][flow] && len(c.inputs[name][flow]) > 0 {
			continue
		}

		for side := range c.inputs[name][flow] {
			if covered[attachment{diagram, name, side}] {
				continue
			}
			c.add(diag.New(diag.CodeIncompleteArrowLayout, a.Flow.Pos, a.Flow.Path,
				"на %s сегменты потока «%s» не доходят до работы «%s» стороной %s: "+
					"взявшись рисовать поток на диаграмме, опишите все его связи там",
				diagramIn(diagram), flow, name, side))
		}

		if c.outputs[name][flow] && !covered[attachment{diagram, name, ir.SideOut}] {
			c.add(diag.New(diag.CodeIncompleteArrowLayout, a.Flow.Pos, a.Flow.Path,
				"на %s сегменты потока «%s» не выходят из работы «%s»: "+
					"взявшись рисовать поток на диаграмме, опишите все его связи там",
				diagramIn(diagram), flow, name))
		}
	}
}

// checkSegments проверяет геометрию одной стрелки. Узлы именуются внутри
// стрелки, поэтому и считаются здесь же: одноимённые узлы разных потоков
// друг о друге ничего не знают.
func (c *checker) checkSegments(a *ir.ArrowLayout) {
	nodes := make(map[string]int)

	for _, s := range a.Segments {
		// Диаграмма зовётся по своей работе, а контекстная — по имени модели.
		if c.name(s.On, "имя диаграммы") && s.On.Name != c.m.Name.Name {
			c.function(s.On, "имя диаграммы")
		}
		if s.Context && s.On.Name != c.m.Name.Name {
			c.add(diag.New(diag.CodeBadGeometry, s.On.Pos, s.On.Path,
				"контекстная диаграмма зовётся именем модели «%s», а не «%s»",
				c.m.Name.Name, s.On.Name))
		}

		c.checkEndpoint(s.From, "from", nodes)
		c.checkEndpoint(s.To, "to", nodes)

		for _, p := range s.Points {
			c.checkCoordinate(p.X, "x")
			c.checkCoordinate(p.Y, "y")
		}
	}

	// Узел, встретившийся один раз, ничего не сшивает: это либо опечатка
	// в имени, либо оборванная ветка.
	for _, s := range a.Segments {
		for _, e := range []*ir.Endpoint{s.From, s.To} {
			if e.Kind() == ir.EndpointNode && nodes[e.Node.Name] == 1 {
				c.add(diag.New(diag.CodeLonelyNode, e.Node.Pos, e.Node.Path,
					"узел «%s» у потока «%s» встречается один раз: сшивать нечего",
					e.Node.Name, a.Flow.Name))
			}
		}
	}
}

func (c *checker) checkEndpoint(e *ir.Endpoint, what string, nodes map[string]int) {
	if e == nil {
		return // висящий конец: в моделях Ramus такие есть
	}

	// Ровно одно из трёх: блок, край листа или узел.
	var kinds []string
	if e.Function.Set() {
		kinds = append(kinds, "function")
	}
	if e.Border != "" {
		kinds = append(kinds, "border")
	}
	if e.Node.Set() {
		kinds = append(kinds, "node")
	}
	switch len(kinds) {
	case 0:
		c.add(diag.New(diag.CodeBadEndpoint, e.Pos, e.Path,
			"конец %s ничего не называет: нужен function, border или node", what))
		return
	case 1:
	default:
		c.add(diag.New(diag.CodeBadEndpoint, e.Pos, e.Path,
			"конец %s называет сразу %s, а должен что-то одно",
			what, strings.Join(kinds, " и ")))
		return
	}

	switch e.Kind() {
	case ir.EndpointFunction:
		c.function(e.Function, "имя работы")
		if e.Side == "" {
			c.add(diag.New(diag.CodeBadEndpoint, e.Pos, e.Path,
				"у конца, прицепленного к работе «%s», не указана сторона (side)",
				e.Function.Name))
		}
	case ir.EndpointBorder:
		if e.Side != "" {
			c.add(diag.New(diag.CodeBadEndpoint, e.Pos, e.Path,
				"side имеет смысл только у конца, прицепленного к работе; у края листа сторона — это border"))
		}
	case ir.EndpointNode:
		if c.name(e.Node, "имя узла") {
			nodes[e.Node.Name]++
		}
		if e.Side != "" {
			c.add(diag.New(diag.CodeBadEndpoint, e.Pos, e.Path,
				"side имеет смысл только у конца, прицепленного к работе, а этот конец — узел"))
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

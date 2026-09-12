package layout

import (
	"fmt"
	"sort"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Ветвление: поток, приходящий на диаграмму к нескольким работам, рисуется
// одним деревом, а не отдельной линией на каждого потребителя.
//
// Рисунок не придуман, а снят с `examples/тест.rsf` посегментно. «Контроль»
// приходит там сверху к четырём блокам и разложен так:
//
//	край сверху ──▼── узел ──▶ узел ──▶ узел      магистраль, общая горизонталь
//	                  │         │        │   ╲
//	                  ▼         ▼        ▼    ▼
//	                блок1     блок2    блок3 блок4
//
// то есть: один вход с края листа, спуск до общей магистрали, цепочка из n−1
// узлов вдоль неё и отвод из каждого узла в свой блок; последний узел
// обслуживает двоих. Сегментов выходит 2n−1 — семь на четырёх потребителях,
// ровно столько их в «тесте». «Вход2» там же приходит слева к двум блокам и
// даёт три сегмента при одном узле: тот же рисунок, повёрнутый на 90°.
//
// Узлы заводятся только там, где есть что ветвить: один потребитель — одна
// линия и ни одного узла (FR-012).

// branch перестраивает граничные стрелки, у которых потребителей больше одного,
// в деревья. Остальные возвращаются как были.
//
// Порядок сохраняется: группа замещается своим деревом на месте первого её
// сегмента. Обход отображений дал бы разные файлы на одном входе.
func branch(all []*arrow, boxes map[string]box, blocks map[string][]box) []*arrow {
	// Ключ источника: у края листа он один на диаграмму и сторону, и все
	// сегменты под ним — ветви одного дерева.
	type key struct{ diagram, flow, side string }

	groups := make(map[key][]*arrow)
	var order []key
	for _, a := range all {
		if !a.from.onBorder() || !a.to.onFunction() {
			continue
		}
		k := key{a.diagram, a.flow, a.from.side}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], a)
	}

	built := make(map[key][]*arrow, len(order))
	for _, k := range order {
		group := groups[k]
		if len(group) < 2 {
			continue // ветвить нечего
		}
		built[k] = tree(group, boxes, blocks[k.diagram])
	}

	done := make(map[key]bool, len(built))
	out := make([]*arrow, 0, len(all))
	for _, a := range all {
		k := key{a.diagram, a.flow, a.from.side}
		segments, branched := built[k]
		if !branched {
			out = append(out, a)
			continue
		}
		if done[k] {
			continue // уже вставлено деревом целиком
		}
		done[k] = true
		out = append(out, segments...)
	}
	return out
}

// tree собирает дерево одной граничной стрелки.
func tree(group []*arrow, boxes map[string]box, blocks []box) []*arrow {
	side := group[0].from.side
	sample := group[0]

	// Потребители выстраиваются вдоль магистрали по фактической координате
	// точки крепления, а не по порядку в документе: иначе линия возвращалась
	// бы назад на работах, стоящих на диаграмме не подряд.
	type target struct {
		arrow *arrow
		at    point
	}
	targets := make([]target, 0, len(group))
	for _, a := range group {
		targets = append(targets, target{arrow: a, at: attach(boxes[a.to.function], a.to)})
	}
	along := alongTrunk(side)
	sort.SliceStable(targets, func(i, j int) bool {
		return along(targets[i].at) < along(targets[j].at)
	})

	trunk := trunkAt(side, blocks)
	// Узел сидит на магистрали напротив своего потребителя: у него координата
	// потребителя вдоль магистрали и координата самой магистрали поперёк.
	at := func(t target) point { return onTrunk(side, trunk, t.at) }

	name := func(i int) string {
		// Имя узла живёт в пределах одной стрелки, но одна стрелка ветвится и
		// на родителе, и на ребёнке, поэтому диаграмма входит в имя.
		return fmt.Sprintf("%s/%s/%d", sample.diagram, side, i)
	}

	seg := func(from, to end, points ...point) *arrow {
		return &arrow{flow: sample.flow, diagram: sample.diagram,
			from: from, to: to, points: points}
	}

	nodes := len(targets) - 1
	out := make([]*arrow, 0, 2*len(targets)-1)

	// Вход: с края листа до первого узла. Пересекает край ровно один раз —
	// это и есть FR-002.
	head := at(targets[0])
	out = append(out, seg(
		end{side: side}, end{node: name(0)},
		entryPoint(side, head), head))

	// Магистраль: цепочка узлов вдоль общей линии.
	for i := 0; i+1 < nodes; i++ {
		out = append(out, seg(
			end{node: name(i)}, end{node: name(i + 1)},
			at(targets[i]), at(targets[i+1])))
	}

	// Отводы: из каждого узла в свой блок. Последний узел ведёт двоих —
	// своего потребителя и последнего, до которого магистраль дотягивается
	// уже без узла.
	for i, t := range targets {
		node := i
		if node >= nodes {
			node = nodes - 1
		}
		line := []point{at(targets[node])}
		if node != i {
			// Дотяжка вдоль магистрали до последнего потребителя, а уже
			// оттуда поворот в блок.
			line = append(line, onTrunk(side, trunk, t.at))
		}
		out = append(out, seg(end{node: name(node)}, t.arrow.to, append(line, t.at)...))
	}
	return out
}

// alongTrunk отдаёт координату вдоль магистрали: для управления и механизма
// магистраль горизонтальна, для входа — вертикальна.
func alongTrunk(side string) func(point) float64 {
	if side == ir.SideIn {
		return func(p point) float64 { return p.y }
	}
	return func(p point) float64 { return p.x }
}

// onTrunk кладёт точку на магистраль напротив заданной.
func onTrunk(side string, trunk float64, at point) point {
	if side == ir.SideIn {
		return point{x: trunk, y: at.y}
	}
	return point{x: at.x, y: trunk}
}

// entryPoint — место, где стрелка пересекает край листа: напротив первого узла.
func entryPoint(side string, head point) point {
	switch side {
	case ir.SideIn:
		return point{x: sheetLeft, y: head.y}
	case ir.SideControl:
		return point{x: head.x, y: sheetTop}
	default: // ir.SideMechanism
		return point{x: head.x, y: sheetBottom}
	}
}

// trunkAt выбирает линию магистрали: посередине между краем листа и ближайшим
// к нему краем блоков.
//
// Это решение, а не мера, и обозначено так намеренно. В «тесте» магистраль
// «контроля» стоит на y=45.365 при крае 7 и верхе блоков 80, механизма — на
// y=396 при крае 437 и низе блоков 372.4; ни то ни другое не описывается ни
// серединой, ни постоянным отступом, а других моделей с ветвлением у нас нет.
// Середина промежутка — то же правило, по которому уже стоит вертикаль прямой
// связи, и тем она здесь и выбрана.
func trunkAt(side string, blocks []box) float64 {
	switch side {
	case ir.SideIn:
		edge := sheetRight
		for _, b := range blocks {
			if b.x < edge {
				edge = b.x
			}
		}
		return between(sheetLeft, edge)
	case ir.SideControl:
		edge := sheetBottom
		for _, b := range blocks {
			if b.y < edge {
				edge = b.y
			}
		}
		return between(sheetTop, edge)
	default: // ir.SideMechanism
		edge := sheetTop
		for _, b := range blocks {
			if b.y+b.height > edge {
				edge = b.y + b.height
			}
		}
		return between(sheetBottom, edge)
	}
}

// between — середина промежутка, но не ближе stub к краю листа: магистраль,
// прижатая вплотную, читается как часть рамки.
func between(sheet, edge float64) float64 {
	mid := (sheet + edge) / 2
	if edge > sheet && mid < sheet+stub {
		return sheet + stub
	}
	if edge < sheet && mid > sheet-stub {
		return sheet - stub
	}
	return mid
}

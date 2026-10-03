package layout

import (
	"fmt"
	"math"
	"sort"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Ветвление: поток, у которого на диаграмме несколько получателей, рисуется
// одним деревом, а не отдельной линией на каждого. Корень дерева — либо край
// листа (поток приходит на диаграмму снаружи), либо порт работы (поток выходит
// из блока). Рисунок у них общий, и различаются они только корнем.
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

// branch перестраивает стрелки, у которых получателей больше одного, в деревья.
// Остальные возвращаются как были.
//
// Источником дерева бывает и край листа, и порт работы. Разница между ними —
// только в корне, поэтому ключ у них общий, а работа в нём пуста у края листа:
// прежнее граничное ветвление осталось частным случаем, а не отдельной веткой
// кода.
//
// Порядок сохраняется: группа замещается своим деревом на месте первого её
// сегмента. Обход отображений дал бы разные файлы на одном входе.
func branch(all []*arrow, boxes map[string]box, blocks map[string][]box) []*arrow {
	type key struct{ diagram, flow, function, side string }

	groups := make(map[key][]*arrow)
	var order []key
	for _, a := range all {
		if !branchable(a) {
			continue
		}
		k := key{a.diagram, a.flow, a.from.function, a.from.side}
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
		if k.function == "" {
			built[k] = tree(group, boxes, blocks[k.diagram])
			continue
		}
		built[k] = portTree(group, boxes, blocks[k.diagram])
	}

	done := make(map[key]bool, len(built))
	out := make([]*arrow, 0, len(all))
	for _, a := range all {
		k := key{a.diagram, a.flow, a.from.function, a.from.side}
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

// branchable сообщает, что сегмент вправе войти в дерево.
//
// С края листа — только в блок: края, соединённого с краем, не бывает. Из порта
// работы — куда угодно, в том числе за край листа: поток, объявленный выходом
// диаграммы, уходит наружу такой же веткой, как всякая другая.
func branchable(a *arrow) bool {
	if a.from.onBorder() {
		return a.to.onFunction()
	}
	return a.from.onFunction()
}

// portTree собирает дерево, растущее из порта работы.
//
// Рисунок снят с `examples/ФормированиеТП.rsf`: поток «Структура программа»
// выходит из блока 51 в точке (345.3, 185.9), идёт горизонталью до узла
// (400.6, 185.9), и уже из узла расходится к двум блокам. Узел там один на всё
// дерево и стоит на высоте порта: цепочки узлов вдоль магистрали, какую Ramus
// строит у граничной стрелки, здесь нет.
//
// Сортировать получателей не нужно и незачем: из одного узла ветки расходятся
// в любом порядке, а порядок записи берётся из документа и потому устойчив.
func portTree(group []*arrow, boxes map[string]box, blocks []box) []*arrow {
	sample := group[0]
	port := attach(boxes[sample.from.function], sample.from)
	node := point{x: portLane(group, port, boxes, blocks), y: port.y}

	// Имя узла живёт в пределах одной стрелки, но один поток вправе ветвиться
	// на диаграмме и от края листа, и от порта, и от двух портов сразу —
	// поэтому в имя входит источник, а не одна лишь сторона.
	name := fmt.Sprintf("%s/%s/%s", sample.diagram, sample.from.function, sample.from.side)

	out := make([]*arrow, 0, len(group)+1)

	// Голова: от порта до узла. Она одна — ради неё всё и затевалось.
	out = append(out, &arrow{flow: sample.flow, diagram: sample.diagram,
		from: sample.from, to: end{node: name},
		points: clean([]point{port, node})})

	for _, a := range group {
		_, to := ends(a, boxes)
		// Маршрут ветки строит route() по исходной стрелке: и направление, и
		// обход блоков он считает по блоку производителя, а у конца-узла блока
		// нет — backward() на нём всегда сказал бы «вперёд» и увёл обратную
		// ветку сквозь лестницу. Меняется только начальная точка: узел вместо
		// порта.
		out = append(out, &arrow{flow: a.flow, diagram: a.diagram,
			from: end{node: name}, to: a.to,
			points: route(a, node, to, blocks)})
	}
	return out
}

// portLane — вертикаль, на которой стоит узел дерева от порта.
//
// Это решение, а не мера, и обозначено так намеренно. В «ФормированииТП» узлы
// двух таких деревьев стоят на 400.625 и 249.375 при промежутках 345.3…450 и
// 153…273.3. Ни серединой промежутка (397.7 и 213.2), ни постоянным отступом
// от блока (55.3 против 96.4) эти числа не описываются, а файл правлен руками:
// выдавать их за меру нельзя.
//
// Берётся то же правило, по которому уже стоит вертикаль прямой связи, —
// середина промежутка до ближайшего получателя (middle в route.go). Правило в
// пакете уже есть, снято с настоящих файлов для соседнего случая, и заводить
// второе того же смысла незачем. Заодно ветка к ближайшему получателю выходит
// ровно такой, какой была бы одиночная связь.
func portLane(group []*arrow, port point, boxes map[string]box, blocks []box) float64 {
	lane := math.Inf(1)
	for _, a := range group {
		if !a.to.onFunction() {
			continue
		}
		at := middle(a, port, attach(boxes[a.to.function], a.to), blocks)
		if at > port.x && at < lane {
			lane = at
		}
	}
	if math.IsInf(lane, 1) {
		// Все ветки уходят назад или за край листа: промежутка, по которому
		// считать середину, нет. Отступ от порта — тот же stub, на котором
		// разворачивается обратная связь.
		return port.x + stub
	}
	return lane
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
		out = append(out, seg(end{node: name(node)}, t.arrow.to, branchLine(side, append(line, t.at), blocks)...))
	}
	return out
}

// branchLine — отвод из узла в блок, не задевающий чужих блоков
// (specs/014-arrows-avoid-blocks).
//
// Сверху и снизу магистраль лежит над или под всеми блоками, и отвод
// спускается к порту вертикалью: в колонке порта над приёмником (и под ним)
// блоков лестницы нет, отвод чист всегда. Слева магистраль вертикальна, и
// отвод идёт к порту горизонталью на его высоте, — а выросшие блоки
// перекрываются по высоте (place), и горизонталь может задеть блок, стоящий
// левее приёмника. Тогда отвод уходит от узла вдоль магистрали в коридор над
// блоками или под ними и подходит к порту слева, как вход у around. Узел не
// двигается: он остаётся одной точкой на магистрали.
func branchLine(side string, line []point, blocks []box) []point {
	if side != ir.SideIn {
		return line
	}
	node, port := line[0], line[len(line)-1]
	back := port.x - stub
	candidates := [][]point{line}
	for _, corridor := range []float64{corridorAbove(blocks), corridorBelow(blocks)} {
		candidates = append(candidates, []point{node, {x: node.x, y: corridor},
			{x: back, y: corridor}, {x: back, y: port.y}, port})
	}
	return dodge(blocks, candidates...)
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

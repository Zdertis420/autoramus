package layout

import (
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Стрелки: что нарисовано на каждой диаграмме и куда прицеплено.
//
// Связи в документе лежат половинками (ir.Build): у записи из in, control и
// mechanism заполнено только To, у записи из out — только From. Пара собирается
// на диаграмме: поток, который тут кто-то производит, даёт стрелку между
// блоками; поток, которого не производит никто, приходит с края листа.
//
// Опираться на это можно потому, что конвейер идёт лесенкой: валидатор уже
// проверил баланс (checkBalance, checkDecomposition) и гарантировал, что
// потребляемый поток либо производится на этой же диаграмме, либо приходит к
// родителю той же стороной. Перепроверять здесь нечего.

// arrow — один нарисованный сегмент на одной диаграмме.
type arrow struct {
	flow string
	// diagram — владелец диаграммы; пусто означает контекстную A-0.
	diagram string
	from    end
	to      end
	// points — готовая ломаная. Непуста у сегментов дерева: их маршруты
	// считаются все вместе (tree.go), а не поодиночке, потому что магистраль
	// и узлы у них общие. У остальных пуста, и маршрут строит route().
	points []point
}

// end — конец стрелки: сторона блока, край листа или узел ветвления.
type end struct {
	// function — работа, к которой прицеплен конец; пусто — край листа
	// либо узел.
	function string
	// side — сторона ICOM. У края листа она же говорит, какой это край:
	// вход приходит слева, управление сверху, механизм снизу, выход уходит
	// вправо.
	side string
	// place — место на стороне блока, k-я стрелка из n. Раздаётся отдельным
	// проходом, когда все стрелки диаграммы уже известны.
	place slot
	// node — имя узла ветвления; непустое означает, что конец сходится с
	// другими сегментами этой же стрелки. Область видимости имени — одна
	// стрелка, так его понимает и генератор.
	node string
}

// onBorder сообщает, что конец лежит на краю листа.
func (e end) onBorder() bool { return e.function == "" && e.node == "" }

// onFunction сообщает, что конец прицеплен к блоку.
func (e end) onFunction() bool { return e.function != "" }

// slot — место стрелки на стороне блока: k-я из n.
type slot struct{ k, n int }

// arrows выводит стрелки всех диаграмм модели и раздаёт им места на сторонах.
//
// Порядок — порядок диаграмм, внутри диаграммы порядок связей в документе.
// Обход отображений дал бы разные файлы на одном и том же входе (принцип III
// конституции).
func arrows(m *ir.Model) []*arrow {
	var out []*arrow
	for _, d := range diagrams(m) {
		out = append(out, diagramArrows(m, d)...)
	}
	assignSlots(out)
	return out
}

// diagramArrows выводит стрелки одной диаграммы.
func diagramArrows(m *ir.Model, d diagram) []*arrow {
	produced := make(map[string][]string)
	consumed := make(map[string]bool)
	for _, l := range m.Links {
		if l.From.Name != "" && d.siblings[l.From.Name] {
			produced[l.Flow.Name] = appendOnce(produced[l.Flow.Name], l.From.Name)
		}
		if l.To.Name != "" && d.siblings[l.To.Name] {
			consumed[l.Flow.Name] = true
		}
	}

	// Одна и та же связь выразима дважды: списками ICOM у обеих работ и записью
	// в links (Р7). Половинки ICOM и явная запись приходят сюда по разным
	// веткам, и без ключа стрелка рисовалась бы двумя параллельными линиями с
	// одной подписью, да ещё и с общими кросспоинтами — Ramus считал бы их
	// одним узлом с двумя секторами. Об этом же скажет валидатор
	// (duplicate_link), но сказать мало: файл должен быть верным.
	type linkKey struct{ flow, from, fromSide, to, toSide string }

	var out []*arrow
	seen := make(map[linkKey]bool)
	add := func(flow string, from, to end) {
		k := linkKey{flow, from.function, from.side, to.function, to.side}
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, &arrow{flow: flow, diagram: d.owner, from: from, to: to})
	}

	for _, l := range m.Links {
		flow := l.Flow.Name
		switch {
		case l.To.Name != "" && d.siblings[l.To.Name]:
			to := end{function: l.To.Name, side: l.SideName()}

			// Явная связь секции links: оба конца названы автором, и искать
			// производителя по имени потока не нужно (Р7).
			if l.From.Name != "" && d.siblings[l.From.Name] {
				add(flow, end{function: l.From.Name, side: ir.SideOut}, to)
				continue
			}

			producers := produced[flow]
			if len(producers) == 0 {
				// Производителя на диаграмме нет — поток приходит снаружи.
				// Край выбирается по стороне: у граничной стрелки сторона
				// ICOM не меняется, это и проверяет валидатор.
				add(flow, end{side: to.side}, to)
				continue
			}
			for _, from := range producers {
				if from == to.function {
					continue // работа сама себе источник: рисовать нечего
				}
				add(flow, end{function: from, side: ir.SideOut}, to)
			}

		case l.From.Name != "" && d.siblings[l.From.Name] && l.To.Name == "":
			if consumed[flow] {
				// Уже нарисовано стрелкой к потребителю: связь одна, и
				// рисовать её дважды значило бы удвоить стрелки.
				continue
			}
			add(flow, end{function: l.From.Name, side: ir.SideOut}, end{side: ir.SideOut})
		}
	}
	return out
}

// appendOnce добавляет имя, если его ещё нет: работа, объявившая один и тот же
// выход дважды, не должна давать двух стрелок.
func appendOnce(list []string, name string) []string {
	for _, existing := range list {
		if existing == name {
			return list
		}
	}
	return append(list, name)
}

// assignSlots раздаёт стрелкам места на сторонах блоков.
//
// Сторона делится на n+1 равных частей, k-я стрелка садится в k/(n+1). Мера
// снята с контекстной диаграммы `examples/тест.rsf`: два входа стоят ровно в
// 1/3 и 2/3 высоты блока, а одиночные управление, механизм и выход — в
// середине стороны.
//
// Ключ включает диаграмму: один и тот же блок на диаграмме родителя и на своей
// собственной несёт разные стрелки, и места считаются отдельно.
func assignSlots(all []*arrow) {
	type key struct{ diagram, function, side string }

	total := make(map[key]int)
	for _, a := range all {
		for _, e := range []end{a.from, a.to} {
			if !e.onFunction() {
				continue
			}
			total[key{a.diagram, e.function, e.side}]++
		}
	}

	taken := make(map[key]int, len(total))
	for _, a := range all {
		for _, e := range []*end{&a.from, &a.to} {
			if !e.onFunction() {
				continue
			}
			k := key{a.diagram, e.function, e.side}
			taken[k]++
			e.place = slot{k: taken[k], n: total[k]}
		}
	}
}

// attach отдаёт точку крепления к стороне блока.
func attach(b box, e end) point {
	f := float64(e.place.k) / float64(e.place.n+1)
	switch e.side {
	case ir.SideIn:
		return point{x: b.x, y: b.y + b.height*f}
	case ir.SideControl:
		return point{x: b.x + b.width*f, y: b.y}
	case ir.SideMechanism:
		return point{x: b.x + b.width*f, y: b.y + b.height}
	default: // ir.SideOut
		return point{x: b.x + b.width, y: b.y + b.height*f}
	}
}

// borderOf переводит сторону ICOM в край листа. Названия у края геометрические:
// роли ICOM у него нет, он просто сторона диаграммы.
func borderOf(side string) string {
	switch side {
	case ir.SideIn:
		return ir.BorderLeft
	case ir.SideControl:
		return ir.BorderTop
	case ir.SideMechanism:
		return ir.BorderBottom
	default: // ir.SideOut
		return ir.BorderRight
	}
}

// boxIndex сводит раскладку блоков в отображение «работа → прямоугольник».
func boxIndex(m *ir.Model) map[string]box {
	out := make(map[string]box, len(m.Layout.Functions))
	for _, f := range m.Layout.Functions {
		out[f.Function.Name] = box{
			x:      f.X.Val,
			y:      f.Y.Val,
			width:  f.Width.Val,
			height: f.Height.Val,
			owner:  f.Function.Name,
		}
	}
	return out
}

// arrowKey — пара «поток + диаграмма»: единица, которой меряется оверрайд.
//
// Не поток целиком: описав стрелку на одной диаграмме, автор не отказывается
// от неё на всех остальных. Прежде отказывался, и сегмент на A-0 пропадал из
// файла молча.
//
// И не отдельный сегмент: стрелка на диаграмме — дерево с общей магистралью и
// общими узлами, строится целиком или не строится. Авторское дерево и
// дописанная раскладкой ветка дали бы на одной диаграмме два входа с края —
// ровно тот рисунок, который убирало граничное ветвление.
type arrowKey struct{ flow, diagram string }

// placeArrows дописывает геометрию стрелкам, которых не задал автор.
//
// Стрелка в языке — это поток со всеми своими сегментами, поэтому сегменты
// группируются по имени потока, а порядок записей берётся из секции flows:
// обход отображения дал бы разные файлы на одном входе.
func placeArrows(m *ir.Model) {
	known := authored(m)

	boxes := boxIndex(m)
	blocks := make(map[string][]box)
	for _, d := range diagrams(m) {
		blocks[d.owner] = diagramBoxes(d, boxes)
	}

	var free []*arrow
	for _, a := range arrows(m) {
		if known[arrowKey{flow: a.flow, diagram: a.diagram}] {
			// Автор нарисовал этот поток на этой диаграмме сам — не трогаем
			// ни одного его сегмента здесь (Р2), и ветвление к нему тоже не
			// применяется. На прочих диаграммах поток раскладывается как
			// обычно.
			continue
		}
		free = append(free, a)
	}

	// Ломаные строятся все разом и до записи в IR: разведение по каналам
	// (channel.go) смотрит на готовые координаты всех стрелок диаграммы сразу.
	// Пока маршрут считался лениво, в segmentOf, увидеть их вместе было негде.
	routed := branch(free, boxes, blocks)
	for _, a := range routed {
		if a.points != nil {
			continue // сегмент дерева: его ломаная посчитана целиком в tree.go
		}
		from, to := ends(a, boxes)
		a.points = route(a, from, to, blocks[a.diagram])
	}

	channels(routed, blocks, occupied(m))

	drawn := make(map[string][]*arrow)
	for _, a := range routed {
		drawn[a.flow] = append(drawn[a.flow], a)
	}

	// Запись потока, которую автор уже завёл: дописывать будем в неё, а не
	// рядом. Две записи об одном потоке не запрещены, но читать такой документ
	// незачем — сегменты одной стрелки должны лежать вместе.
	existing := make(map[string]*ir.ArrowLayout, len(m.Layout.Arrows))
	for _, a := range m.Layout.Arrows {
		if _, ok := existing[a.Flow.Name]; !ok {
			existing[a.Flow.Name] = a
		}
	}

	for _, flow := range m.Flows {
		segments := drawn[flow.Name]
		if len(segments) == 0 {
			continue
		}

		out, ok := existing[flow.Name]
		if !ok {
			path := fmt.Sprintf("/layout/arrows/%d", len(m.Layout.Arrows))
			out = &ir.ArrowLayout{
				Flow: ir.Ref{Name: flow.Name, Path: path + "/flow"},
				Path: path,
			}
			m.Layout.Arrows = append(m.Layout.Arrows, out)
		}

		// Дописанное идёт после авторского: индексы и координаты того, что
		// автор задал сам, не двигаются (Р2).
		for _, a := range segments {
			path := fmt.Sprintf("%s/segments/%d", out.Path, len(out.Segments))
			out.Segments = append(out.Segments, segmentOf(m, a, path))
		}
	}
}

// authored отмечает пары «поток + диаграмма», которые автор нарисовал сам.
//
// Диаграмма зовётся по своей работе, а контекстная A-0 — по имени модели, и
// отличить её от декомпозиции корневой работы можно только по пометке Context.
// Внутри раскладки контекстная диаграмма зовётся пустой строкой — так же, как
// в arrow.diagram.
func authored(m *ir.Model) map[arrowKey]bool {
	out := make(map[arrowKey]bool)
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			diagram := s.On.Name
			if s.Context {
				diagram = ""
			}
			out[arrowKey{flow: a.Flow.Name, diagram: diagram}] = true
		}
	}
	return out
}

// diagramBoxes отдаёт блоки одной диаграммы: то, сквозь что стрелке идти
// нельзя.
func diagramBoxes(d diagram, boxes map[string]box) []box {
	out := make([]box, 0, len(d.children))
	for _, name := range d.children {
		if b, ok := boxes[name]; ok {
			out = append(out, b)
		}
	}
	return out
}

// segmentOf собирает сегмент: диаграмму, концы и ломаную.
//
// Ломаная приходит готовой: её построил и развёл по каналам placeArrows. Здесь
// остаётся перенос, и это намеренно — считать маршрут в тот момент, когда
// сегмент уже пишется в IR, значило бы считать его после разведения.
func segmentOf(m *ir.Model, a *arrow, path string) *ir.Segment {
	seg := &ir.Segment{Path: path}
	if a.diagram == "" {
		// Контекстная диаграмма зовётся так же, как корневая работа, и
		// отличается от её декомпозиции только этой пометкой.
		seg.Context = true
		seg.On = ir.Ref{Name: m.Name.Name, Path: path + "/on"}
	} else {
		seg.On = ir.Ref{Name: a.diagram, Path: path + "/on"}
	}
	seg.From = endpointOf(a.from, path+"/from")
	seg.To = endpointOf(a.to, path+"/to")

	for i, p := range a.points {
		point := fmt.Sprintf("%s/points/%d", path, i)
		seg.Points = append(seg.Points, ir.Point{
			X:    ir.Num{Val: p.x, Set: true, Path: point + "/0"},
			Y:    ir.Num{Val: p.y, Set: true, Path: point + "/1"},
			Path: point,
		})
	}
	return seg
}

// ends считает точки обоих концов. Конец на краю листа выравнивается по
// противоположному: граничная стрелка идёт прямой, пока ей не мешают.
func ends(a *arrow, boxes map[string]box) (from, to point) {
	if !a.from.onBorder() {
		from = attach(boxes[a.from.function], a.from)
	}
	if !a.to.onBorder() {
		to = attach(boxes[a.to.function], a.to)
	}
	if a.from.onBorder() {
		from = borderPoint(a.from.side, to)
	}
	if a.to.onBorder() {
		to = borderPoint(a.to.side, from)
	}
	return from, to
}

// borderPoint кладёт конец на край листа напротив уже известного конца.
func borderPoint(side string, opposite point) point {
	switch borderOf(side) {
	case ir.BorderLeft:
		return point{x: sheetLeft, y: opposite.y}
	case ir.BorderRight:
		return point{x: sheetRight, y: opposite.y}
	case ir.BorderTop:
		return point{x: opposite.x, y: sheetTop}
	default: // ir.BorderBottom
		return point{x: opposite.x, y: sheetBottom}
	}
}

// endpointOf переводит конец стрелки в язык IR.
func endpointOf(e end, path string) *ir.Endpoint {
	if e.node != "" {
		return &ir.Endpoint{Node: ir.Ref{Name: e.node, Path: path + "/node"}, Path: path}
	}
	if e.onBorder() {
		return &ir.Endpoint{Border: borderOf(e.side), Path: path}
	}
	return &ir.Endpoint{
		Function: ir.Ref{Name: e.function, Path: path + "/function"},
		Side:     e.side,
		Path:     path,
	}
}

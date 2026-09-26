package layout

import (
	"sort"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Каналы: две стрелки не должны лечь на одну линию так, чтобы слиться в одну.
//
// Маршруты считаются поодиночке и про соседей не знают: вертикаль прямой связи
// — середина промежутка между блоками, коридор обратной связи — одна
// горизонталь на диаграмму, магистраль ветвления — одна линия на сторону. Для
// одной стрелки это верно, для второй — то же самое число.
//
// Поэтому разведение живёт отдельным проходом по уже готовым ломаным, а не
// правкой четырёх формул. Причина не в аккуратности, а в том, что наложение
// бывает межсемейным: feedback() не может знать, что middle() выбрала ту же
// вертикаль. Увидеть это можно только там, где видны все координаты разом.
//
// Пересечение в точке допустимо и никого не смущает; разводится только
// наложение отрезком.

// ref — точка ломаной: чья и какая по счёту.
type ref struct {
	arrow *arrow
	index int
}

// claim — заявка стрелки на линию. Единица, которая двигается.
//
// Заявка, а не сегмент, потому что сегменты одной стрелки на одной линии
// обязаны ехать вместе: магистраль ветвления — это три и более сегмента на
// общей горизонтали, а узел, где они сходятся, у Ramus одна точка. Сдвинув
// сегмент порознь, мы развели бы узел на две точки и вернули бы ошибку, которую
// закрыло граничное ветвление.
type claim struct {
	diagram  string
	flow     string
	vertical bool    // линия вертикальна: общая координата — x
	coord    float64 // каноническая координата, та самая, что даёт формула
	lo, hi   float64 // протяжённость вдоль линии
	refs     []ref   // точки, которые едут вместе с заявкой
	pinned   bool    // двигать нельзя: сдвиг отцепит стрелку
	key      [2]int  // порядок в документе: номер стрелки, затем первой точки
}

// channels разводит заявки разных стрелок, попавшие на одну линию.
//
// occupied — линии, занятые не нами: геометрия, написанная автором. Её не
// двигают (Р2), но и не игнорируют — иначе автоматическая стрелка легла бы
// поверх авторской и не заметила бы (FR-009).
func channels(all []*arrow, blocks map[string][]box, occupied []claim) {
	byDiagram := make(map[string][]claim)
	for _, c := range append(claims(all), occupied...) {
		byDiagram[c.diagram] = append(byDiagram[c.diagram], c)
	}

	// Порядок диаграмм берётся из порядка стрелок, а не из обхода отображения:
	// иначе два запуска дали бы разные файлы (принцип III конституции).
	seen := make(map[string]bool, len(byDiagram))
	for _, a := range all {
		if seen[a.diagram] {
			continue
		}
		seen[a.diagram] = true
		// Стрелки диаграммы — чтобы, выбирая сторону сдвига, видеть, кого
		// заявка пересечёт. Порядок — порядок all, без обхода отображений.
		var arrows []*arrow
		for _, b := range all {
			if b.diagram == a.diagram {
				arrows = append(arrows, b)
			}
		}
		spread(byDiagram[a.diagram], blocks[a.diagram], arrows)
	}
}

// claims собирает заявки всех стрелок.
func claims(all []*arrow) []claim {
	// Ключ линии. Координата входит в него значением: совпадение рождается из
	// одной и той же формулы, применённой дважды, и совпадает побитово.
	type key struct {
		diagram, flow string
		vertical      bool
		coord         float64
	}

	index := make(map[key]int)
	var out []claim

	for n, a := range all {
		for i := 1; i < len(a.points); i++ {
			p, q := a.points[i-1], a.points[i]

			var k key
			var lo, hi float64
			switch {
			case p.x == q.x && p.y == q.y:
				continue // вырожденный отрезок: его убирает clean()
			case p.x == q.x:
				k = key{a.diagram, a.flow, true, p.x}
				lo, hi = minMax(p.y, q.y)
			case p.y == q.y:
				k = key{a.diagram, a.flow, false, p.y}
				lo, hi = minMax(p.x, q.x)
			default:
				// Наискось маршруты не ходят. Если такой отрезок появится,
				// разводить его нечем: у него нет одной координаты.
				continue
			}

			at, ok := index[k]
			if !ok {
				at = len(out)
				index[k] = at
				out = append(out, claim{
					diagram: k.diagram, flow: k.flow, vertical: k.vertical,
					coord: k.coord, lo: lo, hi: hi, key: [2]int{n, i - 1},
				})
			}

			c := &out[at]
			if lo < c.lo {
				c.lo = lo
			}
			if hi > c.hi {
				c.hi = hi
			}
			if terminal(a, i-1) || terminal(a, i) {
				c.pinned = true
			}
		}
	}

	// Вторым проходом — все точки стрелки, лежащие на линии заявки, а не
	// только концы её отрезков.
	//
	// Без этого узел ветвления разъезжается на две точки: магистраль идёт по
	// горизонтали, отвод из узла — по вертикали, и точка узла попадает в
	// горизонтальную заявку только как конец отрезка магистрали. Отвод хранит
	// свою копию той же точки, она остаётся на прежней линии, и узел, который у
	// Ramus обязан быть одной точкой, становится двумя.
	for _, a := range all {
		for i, p := range a.points {
			for _, k := range [2]key{
				{a.diagram, a.flow, true, p.x},
				{a.diagram, a.flow, false, p.y},
			} {
				if at, ok := index[k]; ok {
					out[at].refs = appendRef(out[at].refs, ref{a, i})
					if terminal(a, i) {
						out[at].pinned = true
					}
				}
			}
		}
	}
	return out
}

// terminal сообщает, что точка — конец стрелки, прицепленный к блоку или
// лежащий на краю листа.
//
// Такую точку двигать нельзя: место на стороне блока выдано ей раскладкой
// (assignSlots), а сдвиг увёл бы стрелку со стороны. Сегмент, у которого такой
// конец есть, закреплён целиком — он и станет препятствием для соседей вместо
// того, чтобы уступать им место.
//
// Правило намеренно строже необходимого: точка на краю листа могла бы скользить
// вдоль края, но выигрыш от этого мал, а способов отцепить стрелку — много.
func terminal(a *arrow, i int) bool {
	if i == 0 {
		return a.from.node == ""
	}
	if i == len(a.points)-1 {
		return a.to.node == ""
	}
	return false
}

// appendRef добавляет точку, если её ещё нет: соседние сегменты одной линии
// делят точку, и двигать её дважды незачем.
func appendRef(list []ref, r ref) []ref {
	for _, existing := range list {
		if existing == r {
			return list
		}
	}
	return append(list, r)
}

// spread раздаёт линии заявкам одной диаграммы.
//
// Закреплённые встают первыми и своих координат не меняют: подвинуться обязан
// тот, кто может. Остальные идут в порядке документа, и первая по нему остаётся
// на канонической линии — за счёт этого модель, где разводить нечего, не
// сдвигается ни на йоту (FR-008).
func spread(all []claim, blocks []box, arrows []*arrow) {
	var fixed, movable []claim
	for _, c := range all {
		if c.pinned {
			fixed = append(fixed, c)
		} else {
			movable = append(movable, c)
		}
	}

	sort.SliceStable(movable, func(i, j int) bool {
		a, b := movable[i].key, movable[j].key
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		return a[1] < b[1]
	})

	// Когда блоки налезают друг на друга, чистой линии не существует в
	// принципе, и запрет заезжать в блок запретил бы всякое движение вообще:
	// заявка осталась бы на канонической линии вместе со своим наложением.
	// Такое бывает только от ручной геометрии — своя раскладка разносит блоки по
	// диагонали, — и требовать в этом случае чистого пути значит требовать
	// невозможного. Ровно так же рассуждает проверка маршрутов.
	clean := !crowded(blocks)

	placed := fixed
	for _, c := range movable {
		move(&c, pickLane(c, placed, blocks, clean, arrows))
		placed = append(placed, c)
	}
}

// crowded сообщает, что блоки диаграммы налезают друг на друга.
func crowded(blocks []box) bool {
	for i, a := range blocks {
		for _, b := range blocks[i+1:] {
			if a.x < b.x+b.width && b.x < a.x+a.width &&
				a.y < b.y+b.height && b.y < a.y+a.height {
				return true
			}
		}
	}
	return false
}

// pickLane выбирает линию для заявки: каноническую, если на ней свободно, иначе
// ближайшую свободную по обе стороны от неё.
//
// Из двух свободных полос одного шага берётся та, где стрелка заявки меньше
// пересекает остальные (specs/015-remove-double-crossings). Прежде бралась
// первая по списку, «+шаг», — и у входа в «Тушение» на «чахохбили» «Томатная
// масса» ушла на «+6» и дважды пересекла «Курицу с луком», хотя с «−6» не
// пересекала бы её вовсе. При равенстве — по-прежнему «+шаг»: где выбирать не
// из чего, рисунок не меняется. Шаг от канона важнее пересечений: дальняя
// полоса без пересечений не берётся вместо ближней с одним — разведение не
// вправе двигать рисунок шире необходимого.
//
// Перебор конечный и упорядоченный: шаг постоянен, стороны чередуются, за края
// листа выходить нельзя. Это не поиск пути и не решатель ограничений — от
// запуска к запуску результат один и тот же.
//
// Свободной может не оказаться вовсе: между блоками бывает слишком тесно. Тогда
// берётся каноническая линия, и наложение остаётся. Это названная граница
// (FR-010), а не тихая неудача, и ровно так же поступает pick() при обходе
// блоков.
func pickLane(c claim, placed []claim, blocks []box, clean bool, arrows []*arrow) float64 {
	if free(c, c.coord, placed, blocks, false, clean) {
		return c.coord
	}

	low, high := sheetTop, sheetBottom
	if c.vertical {
		low, high = sheetLeft, sheetRight
	}

	for step := 1; step <= maxChannels; step++ {
		shift := float64(step) * channelStep
		best, fewest := 0.0, -1
		for _, lane := range [2]float64{c.coord + shift, c.coord - shift} {
			if lane < low || lane > high {
				continue
			}
			if !free(c, lane, placed, blocks, true, clean) {
				continue
			}
			if n := crossingsAt(c, lane, arrows); fewest < 0 || n < fewest {
				best, fewest = lane, n
			}
		}
		if fewest >= 0 {
			return best
		}
	}
	return c.coord
}

// free сообщает, что на линии заявке есть место.
//
// Занято, если там уже лежит заявка, перекрывающаяся по протяжённости. Общая
// линия сама по себе помехой не считается: две стрелки, идущие по одной
// вертикали на разной высоте, различимы прекрасно, и разводить их значило бы
// двигать рисунок без нужды. Мера этому есть — три такие пары на диаграмме A0
// «чахохбили».
//
// moved говорит, что заявка съезжает с канонической линии: только тогда
// проверяется, во что она переезжает. На канонической линии проверять нечего —
// маршрут её уже выбрал, и если он и задевает блок, то это дефект маршрута, а
// не разведения, и лечится он не здесь.
func free(c claim, lane float64, placed []claim, blocks []box, moved, clean bool) bool {
	for _, other := range placed {
		if other.vertical != c.vertical || other.coord != lane {
			continue
		}
		if other.flow == c.flow {
			continue // своя же стрелка: общая магистраль намеренна (FR-003)
		}
		if minf(c.hi, other.hi)-maxf(c.lo, other.lo) > 0 {
			return false
		}
	}
	return !moved || !damages(c, lane, blocks, clean)
}

// damages сообщает, что переезд на линию портит рисунок.
//
// Портит тремя способами, и все три увидены на настоящих моделях:
//
//   - отрезок схлопывается в ноль, когда линия совпадает с координатой соседней
//     точки. Ramus рисует такой отрезок точкой на стрелке, и выглядит это как
//     грязь — ровно то, что убирает clean() при построении маршрута;
//   - отрезок ложится вдоль стороны блока и сливается с рамкой. Это то же
//     наложение, только вторая линия принадлежит не стрелке, а работе (FR-004);
//   - отрезок заезжает внутрь блока.
//
// Первое проверяется всегда: схлопнутый отрезок — это грязь и на диаграмме, где
// блоки налезают друг на друга. Второе и третье — только когда чистый путь
// вообще существует (clean).
//
// Проверяются все отрезки затронутых стрелок, а не только сегменты самой
// заявки: заявка везёт свои точки, и соседние отрезки ломаной при этом
// удлиняются. Удлинившийся отрезок способен задеть блок ничуть не хуже.
func damages(c claim, lane float64, blocks []box, clean bool) bool {
	was := make([]point, 0, len(c.refs))
	for _, r := range c.refs {
		was = append(was, r.arrow.points[r.index])
	}
	shift(c, lane)
	defer func() {
		for i, r := range c.refs {
			r.arrow.points[r.index] = was[i]
		}
	}()

	seen := make(map[*arrow]bool, len(c.refs))
	for _, r := range c.refs {
		if seen[r.arrow] {
			continue
		}
		seen[r.arrow] = true

		points := r.arrow.points
		for i := 1; i < len(points); i++ {
			if points[i-1] == points[i] {
				return true
			}
		}
		if clean && (crossesRoute(points, blocks) || alongSide(points, blocks)) {
			return true
		}
	}
	return false
}

// crossingsAt считает, сколько раз стрелка заявки пересечёт остальные стрелки
// диаграммы, если заявку сдвинуть на линию. Сдвиг пробный — точки
// возвращаются на место, как в damages.
//
// Своя стрелка себе не помеха: отвод, пересекающий собственную магистраль,
// пересечением не считается. Пересечение — горизонталь строго внутри
// вертикали; касание концами не в счёт.
func crossingsAt(c claim, lane float64, arrows []*arrow) int {
	was := make([]point, 0, len(c.refs))
	for _, r := range c.refs {
		was = append(was, r.arrow.points[r.index])
	}
	shift(c, lane)
	defer func() {
		for i, r := range c.refs {
			r.arrow.points[r.index] = was[i]
		}
	}()

	n := 0
	for _, own := range arrows {
		if own.flow != c.flow {
			continue
		}
		for _, other := range arrows {
			if other.flow == c.flow {
				continue
			}
			for i := 1; i < len(own.points); i++ {
				for j := 1; j < len(other.points); j++ {
					if crossing(own.points[i-1], own.points[i], other.points[j-1], other.points[j]) {
						n++
					}
				}
			}
		}
	}
	return n
}

// crossing сообщает, что горизонталь одного отрезка проходит строго внутри
// вертикали другого.
func crossing(a, b, c, d point) bool {
	ah, ch := a.y == b.y, c.y == d.y
	if ah == ch {
		return false
	}
	if !ah {
		a, b, c, d = c, d, a, b
	}
	x1, x2 := minMax(a.x, b.x)
	y1, y2 := minMax(c.y, d.y)
	return x1 < c.x && c.x < x2 && y1 < a.y && a.y < y2
}

// alongSide сообщает, что отрезок лёг вдоль стороны блока.
//
// Совпасть координатой мало — нужно ещё идти вдоль стороны отрезком ненулевой
// длины. Точка крепления лежит ровно на стороне блока, и отрезок, который в неё
// упирается, касается стороны в одной точке; это норма, а не слияние.
func alongSide(points []point, blocks []box) bool {
	for i := 1; i < len(points); i++ {
		p, q := points[i-1], points[i]
		for _, b := range blocks {
			if p.x == q.x && (p.x == b.x || p.x == b.x+b.width) {
				lo, hi := minMax(p.y, q.y)
				if minf(hi, b.y+b.height)-maxf(lo, b.y) > 0 {
					return true
				}
			}
			if p.y == q.y && (p.y == b.y || p.y == b.y+b.height) {
				lo, hi := minMax(p.x, q.x)
				if minf(hi, b.x+b.width)-maxf(lo, b.x) > 0 {
					return true
				}
			}
		}
	}
	return false
}

// move переносит заявку на линию и запоминает новую координату.
func move(c *claim, lane float64) {
	shift(*c, lane)
	c.coord = lane
}

// shift двигает точки заявки: у вертикальной линии меняется x, у
// горизонтальной — y. Соседние отрезки ломаной при этом удлиняются или
// укорачиваются, а узловые точки едут вместе со своей магистралью, поэтому узел
// остаётся одной точкой.
func shift(c claim, lane float64) {
	for _, r := range c.refs {
		if c.vertical {
			r.arrow.points[r.index].x = lane
		} else {
			r.arrow.points[r.index].y = lane
		}
	}
}

// occupied собирает заявки по геометрии, которую автор написал сам.
//
// Двигать их нельзя (Р2), но и не видеть их нельзя: автоматическая стрелка
// легла бы поверх авторской и не заметила бы этого. Поэтому они приходят в
// раздачу закреплёнными — как препятствие, а не как участник.
func occupied(m *ir.Model) []claim {
	if m.Layout == nil {
		return nil
	}

	var out []claim
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			diagram := s.On.Name
			if s.Context {
				diagram = ""
			}
			for i := 1; i < len(s.Points); i++ {
				p, q := s.Points[i-1], s.Points[i]
				c := claim{diagram: diagram, flow: a.Flow.Name, pinned: true}
				switch {
				case p.X.Val == q.X.Val && p.Y.Val == q.Y.Val:
					continue
				case p.X.Val == q.X.Val:
					c.vertical, c.coord = true, p.X.Val
					c.lo, c.hi = minMax(p.Y.Val, q.Y.Val)
				case p.Y.Val == q.Y.Val:
					c.coord = p.Y.Val
					c.lo, c.hi = minMax(p.X.Val, q.X.Val)
				default:
					continue
				}
				out = append(out, c)
			}
		}
	}
	return out
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

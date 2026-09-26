package layout

import (
	"math"
	"sort"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Маршруты стрелок. У каждой категории свой шаблон из двух-шести точек:
// лестница гарантирует, что у прямой связи источник левее и выше приёмника, и
// путь строится по образцу, а не поиском.
//
// Числа сняты с `examples/тест.rsf` — модели, которую Ramus разложил сам:
// вертикаль прямой связи стоит ровно посередине промежутка между блоками
// (212⅔, 406, 599⅓ на трёх связях подряд), а граничная стрелка идёт прямой от
// края листа. Обратной связи там нет ни одной, и её коридор — наше решение, а
// не измерение.

// point — точка ломаной.
type point struct{ x, y float64 }

// stub — отступ от блока, на котором стрелка разворачивается. Взят равным
// просвету между блоками: своей меры у него нет, а плодить константы, когда
// подходит имеющаяся, незачем.
const stub = gap

// route ведёт стрелку от одного конца к другому.
func route(a *arrow, from, to point, blocks []box) []point {
	switch {
	case a.from.onBorder():
		return clean(fromBorder(a, from, to, blocks))
	case a.to.onBorder():
		// Выход наружу: горизонталь до правого края листа.
		return clean([]point{from, to})
	case backward(a, blocks):
		return clean(feedback(a, from, to, blocks))
	default:
		return clean(forward(a, from, to, blocks))
	}
}

// backward сообщает, что стрелка идёт назад: приёмник стоит не правее
// источника, и прямой маршрут пошёл бы справа налево сквозь лестницу.
//
// Смотрим на фактические прямоугольники, а не на порядок работ в документе:
// координаты мог задать автор, и маршрут обязан считаться по тому, что есть.
func backward(a *arrow, blocks []box) bool {
	source, ok := blockOf(a.from, blocks)
	target, found := blockOf(a.to, blocks)
	if !ok || !found {
		return false
	}
	return source.x+source.width >= target.x
}

// forward — прямая связь: источник левее приёмника.
func forward(a *arrow, from, to point, blocks []box) []point {
	switch a.to.side {
	case ir.SideControl:
		// Вправо до вертикали порта, вниз в верх блока — пока приёмник ниже
		// точки выхода.
		//
		// Выросшие блоки перекрываются по высоте (place), и приёмник может
		// подняться выше выхода источника. Тогда горизонталь прошла бы сквозь
		// него, и стрелка обходит сверху: вправо до середины промежутка,
		// вверх над приёмником, вправо до порта, вниз. Обход — только по
		// нужде: где хватает прежнего шаблона, рисунок не меняется.
		above := to.y - stub
		if from.y <= above {
			return []point{from, {x: to.x, y: from.y}, to}
		}
		mid := middle(a, from, to, blocks)
		return []point{from, {x: mid, y: from.y}, {x: mid, y: above}, {x: to.x, y: above}, to}

	case ir.SideMechanism:
		// Механизм приходит снизу, поэтому стрелка обязана обогнуть блок и
		// подойти из-под него: вход сверху прошёл бы сквозь работу.
		below := corridorBelow(blocks)
		return []point{from, {x: middle(a, from, to, blocks), y: from.y},
			{x: middle(a, from, to, blocks), y: below}, {x: to.x, y: below}, to}

	default: // ir.SideIn
		// Вправо, вниз, вправо. Вертикаль — ровно посередине промежутка.
		mid := middle(a, from, to, blocks)
		return []point{from, {x: mid, y: from.y}, {x: mid, y: to.y}, to}
	}
}

// middle отдаёт вертикаль между блоками: середину промежутка от правого края
// источника до левого края приёмника.
//
// Мера точная: на трёх связях `тест.rsf` середина совпадает до последнего
// знака. Если блоки стоят вплотную или внахлёст (ручная геометрия), берётся
// середина между самими точками — хуже, но определённо.
func middle(a *arrow, from, to point, blocks []box) float64 {
	source, ok := blockOf(a.from, blocks)
	target, found := blockOf(a.to, blocks)
	if ok && found && source.x+source.width < target.x {
		return (source.x + source.width + target.x) / 2
	}
	return (from.x + to.x) / 2
}

// feedback — обратная связь: обход снаружи лестницы.
//
// Меры нет: ни в одной модели проверочного набора обратной связи не
// нарисовано. Правило собственное — коридор под блоками для прихода на вход и
// над блоками для прихода на управление, то есть там, где заведомо пусто.
func feedback(a *arrow, from, to point, blocks []box) []point {
	turn := from.x + stub

	switch a.to.side {
	case ir.SideControl:
		above := corridorAbove(blocks)
		return []point{from, {x: turn, y: from.y}, {x: turn, y: above},
			{x: to.x, y: above}, to}

	case ir.SideMechanism:
		below := corridorBelow(blocks)
		return []point{from, {x: turn, y: from.y}, {x: turn, y: below},
			{x: to.x, y: below}, to}

	default: // ir.SideIn
		// Вход приходит слева, поэтому мало обойти блоки — надо ещё зайти за
		// приёмник и вернуться вправо.
		below := corridorBelow(blocks)
		return pick(blocks, lanes(to.x-stub, blocks), func(back float64) []point {
			return []point{from, {x: turn, y: from.y}, {x: turn, y: below},
				{x: back, y: below}, {x: back, y: to.y}, to}
		})
	}
}

// lanes отдаёт вертикальные полосы-кандидаты, начиная с желаемой.
//
// Желаемая — та, что даёт канонический рисунок; остальные берутся из свободных
// промежутков между блоками и сортируются по близости к ней. Это не поиск пути,
// а выбор полосы из готового списка: перебор конечен, порядок определён, и от
// запуска к запуску результат не меняется.
func lanes(want float64, blocks []box) []float64 {
	out := []float64{want}

	// Занятые полосы по горизонтали, слитые в непересекающиеся отрезки.
	type span struct{ from, to float64 }
	var busy []span
	for _, b := range blocks {
		busy = append(busy, span{b.x, b.x + b.width})
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].from < busy[j].from })

	edge := sheetLeft
	for _, s := range busy {
		if s.from-edge > 2*stub {
			out = append(out, (edge+s.from)/2)
		}
		if s.to > edge {
			edge = s.to
		}
	}
	if sheetRight-edge > 2*stub {
		out = append(out, (edge+sheetRight)/2)
	}

	// Ближние полосы — первыми: чем ближе к желаемой, тем меньше рисунок
	// отличается от канонического.
	rest := out[1:]
	sort.SliceStable(rest, func(i, j int) bool {
		return math.Abs(rest[i]-want) < math.Abs(rest[j]-want)
	})
	return out
}

// pick строит маршрут по каждой полосе и берёт первый, который никого не
// задевает.
//
// Чистого может не быть вовсе: автор вправе расставить блоки так, что обойти
// их нечем. Тогда берётся первый — канонический, — и это названное ограничение
// (FR-005), а не тихая неудача.
func pick(blocks []box, candidates []float64, build func(lane float64) []point) []point {
	for _, lane := range candidates {
		points := build(lane)
		if !crossesRoute(points, blocks) {
			return points
		}
	}
	return build(candidates[0])
}

// crossesRoute сообщает, задевает ли ломаная хоть один блок.
func crossesRoute(points []point, blocks []box) bool {
	for i := 1; i < len(points); i++ {
		if crossesAny(points[i-1], points[i], blocks) {
			return true
		}
	}
	return false
}

// fromBorder — стрелка приходит с края листа.
//
// Прямая, пока она никого не задевает. Задевает — обход: короткий отступ от
// края, вертикаль в свободном коридоре перед блоком и горизонталь в его
// сторону. Тот же рисунок Ramus использует в `тест.rsf` (сектор 97).
func fromBorder(a *arrow, from, to point, blocks []box) []point {
	straight := []point{from, to}
	if !crossesAny(from, to, blocks) {
		return straight
	}

	target, ok := blockOf(a.to, blocks)
	if !ok {
		return straight
	}

	switch a.from.side {
	case ir.SideIn:
		// С края листа — в свободную полосу перед блоком и оттуда в его
		// левую сторону. Тот же рисунок, что у Ramus в секторе 97 «теста».
		edge := corridorAbove(blocks)
		return pick(blocks, lanes(target.x-stub, blocks), func(lane float64) []point {
			return []point{{x: sheetLeft, y: edge}, {x: lane, y: edge}, {x: lane, y: to.y}, to}
		})
	case ir.SideControl:
		lane := corridorAbove(blocks)
		return []point{{x: from.x, y: sheetTop}, {x: from.x, y: lane}, {x: to.x, y: lane}, to}
	case ir.SideMechanism:
		lane := corridorBelow(blocks)
		return []point{{x: from.x, y: sheetBottom}, {x: from.x, y: lane}, {x: to.x, y: lane}, to}
	default:
		return straight
	}
}

// corridorBelow — свободная горизонталь под всеми блоками диаграммы.
func corridorBelow(blocks []box) float64 {
	bottom := sheetTop
	for _, b := range blocks {
		if b.y+b.height > bottom {
			bottom = b.y + b.height
		}
	}
	lane := bottom + stub
	if lane > sheetBottom-stub {
		lane = sheetBottom - stub
	}
	return lane
}

// corridorAbove — свободная горизонталь над всеми блоками диаграммы.
func corridorAbove(blocks []box) float64 {
	top := sheetBottom
	for _, b := range blocks {
		if b.y < top {
			top = b.y
		}
	}
	lane := top - stub
	if lane < sheetTop+stub {
		lane = sheetTop + stub
	}
	return lane
}

// blockOf находит прямоугольник блока, к которому прицеплен конец.
//
// Ищется среди блоков этой же диаграммы: чужие сюда не попадают, а имя работы
// у конца может совпасть с именем работы на другой диаграмме только в
// невалидном документе, которого до раскладки не доходит.
func blockOf(e end, blocks []box) (box, bool) {
	if e.onBorder() {
		return box{}, false
	}
	for _, b := range blocks {
		if b.owner == e.function {
			return b, true
		}
	}
	return box{}, false
}

// crossesAny сообщает, задевает ли отрезок хоть один блок.
//
// Касание не считается: точка крепления лежит ровно на стороне блока, и
// строгие неравенства отделяют её от настоящего пересечения.
func crossesAny(a, b point, blocks []box) bool {
	for _, block := range blocks {
		if crosses(a, b, block) {
			return true
		}
	}
	return false
}

func crosses(a, b point, block box) bool {
	left, right := minMax(a.x, b.x)
	top, bottom := minMax(a.y, b.y)
	return right > block.x && left < block.x+block.width &&
		bottom > block.y && top < block.y+block.height
}

func minMax(a, b float64) (float64, float64) {
	if a > b {
		return b, a
	}
	return a, b
}

// clean убирает повторы: вырожденный шаблон не должен давать отрезков нулевой
// длины. Их Ramus рисует как точки на стрелке, и выглядит это как грязь.
func clean(points []point) []point {
	out := points[:0:0]
	for _, p := range points {
		if n := len(out); n > 0 && out[n-1] == p {
			continue
		}
		out = append(out, p)
	}
	return out
}

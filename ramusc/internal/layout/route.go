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
		// Выход наружу: горизонталь до правого края листа. Правее и ниже может
		// стоять выросший блок, перекрывающийся с источником по высоте
		// (place), — тогда горизонталь уходит в коридор.
		return dodge(blocks, append([][]point{{from, to}}, around(from, to, toBorder, blocks)...)...)
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
		//
		// Оба шаблона — первыми кандидатами, в прежнем порядке; за ними те же
		// на соседних полосах и коридор над блоками: горизонталь на высоте
		// выхода может задеть блок между источником и приёмником.
		above := to.y - stub
		mid := middle(a, from, to, blocks)
		climb := func(lane float64) []point {
			return []point{from, {x: lane, y: from.y}, {x: lane, y: above}, {x: to.x, y: above}, to}
		}
		var candidates [][]point
		if from.y <= above {
			candidates = append(candidates, []point{from, {x: to.x, y: from.y}, to})
		}
		candidates = append(candidates, climb(mid))
		candidates = append(candidates, laned(mid, blocks, climb)...)
		return dodge(blocks, append(candidates, around(from, to, ir.SideControl, blocks)...)...)

	case ir.SideMechanism:
		// Механизм приходит снизу, поэтому стрелка обязана обогнуть блок и
		// подойти из-под него: вход сверху прошёл бы сквозь работу.
		// Вертикаль посередине промежутка может задеть блок между источником
		// и приёмником — тогда соседние полосы и коридор.
		below := corridorBelow(blocks)
		build := func(lane float64) []point {
			return []point{from, {x: lane, y: from.y}, {x: lane, y: below}, {x: to.x, y: below}, to}
		}
		mid := middle(a, from, to, blocks)
		candidates := append([][]point{build(mid)}, laned(mid, blocks, build)...)
		return dodge(blocks, append(candidates, around(from, to, ir.SideMechanism, blocks)...)...)

	default: // ir.SideIn
		// Вправо, вниз, вправо. Вертикаль — ровно посередине промежутка.
		//
		// Через одну ступень лестницы середина промежутка — центр пропущенного
		// блока, и шаблон ведёт стрелку сквозь него. Шаблон остаётся первым
		// кандидатом — чистые связи не меняются, и `тест.rsf` по-прежнему
		// совпадает до знака, — а за ним идут соседние полосы и коридоры.
		build := func(lane float64) []point {
			return []point{from, {x: lane, y: from.y}, {x: lane, y: to.y}, to}
		}
		mid := middle(a, from, to, blocks)
		candidates := append([][]point{build(mid)}, laned(mid, blocks, build)...)
		return dodge(blocks, append(candidates, around(from, to, ir.SideIn, blocks)...)...)
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
		//
		// Коридор под блоками — первым, над ними — запасным: полосы, закрытые
		// снизу, бывают открыты сверху.
		var candidates [][]point
		for _, corridor := range []float64{corridorBelow(blocks), corridorAbove(blocks)} {
			for _, back := range lanes(to.x-stub, blocks) {
				candidates = append(candidates, []point{from, {x: turn, y: from.y}, {x: turn, y: corridor},
					{x: back, y: corridor}, {x: back, y: to.y}, to})
			}
		}
		return dodge(blocks, candidates...)
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

// Обход блоков (specs/014-arrows-avoid-blocks).
//
// Шаблоны маршрутов сняты с `тест.rsf`, но строятся без оглядки на блоки: у
// прямой связи через одну ступень лестницы вертикаль середины промежутка
// попадает ровно в центр пропущенного блока — при постоянном шаге так при
// любых шаге и ширине. Лечится не новым шаблоном, а перебором готового
// списка, как pick() у обратной связи: сначала прежний шаблон, затем тот же на
// соседних полосах, затем путь через коридор над блоками и под ними. Первый,
// кто никого не задевает, и берётся. Поиска пути здесь нет — список конечный,
// порядок определён, и от запуска к запуску выбор один.

// dodge отдаёт первый кандидат, который не задевает ни одного блока.
//
// Чистого нет — первый, то есть прежний шаблон: так бывает, только когда
// автор сам положил свои блоки внахлёст (FR-005), и обойти их нечем. В
// собственной лестнице раскладки последний кандидат — коридор — чист всегда
// (см. around), и сюда дело не доходит.
func dodge(blocks []box, candidates ...[]point) []point {
	for _, c := range candidates {
		if c = clean(c); !crossesRoute(c, blocks) {
			return c
		}
	}
	return clean(candidates[0])
}

// laned строит тот же шаблон на полосах из lanes(), ближайших к прежней, —
// кроме самой прежней: она уже первый кандидат.
func laned(want float64, blocks []box, build func(lane float64) []point) [][]point {
	var out [][]point
	for _, lane := range lanes(want, blocks)[1:] {
		out = append(out, build(lane))
	}
	return out
}

// toBorder — конец стрелки на правом краю листа; для around он подход, как и
// стороны блока.
const toBorder = "border"

// around — обходы через коридоры: вбок от своей стороны на stub, коридор над
// всеми блоками или под ними, подход к концу с его стороны.
//
// Почему это чисто всегда в лестнице, которую расставила раскладка. Коридоры
// лежат над самым высоким и под самым низким блоком — их не пересекает никто.
// Вертикаль в stub за правым краем источника свободна: блоки левее кончаются
// раньше, блоки правее начинаются не ближе просвета gap, а stub = gap, и
// вертикаль в худшем случае касается соседа, не входя в него (страховка
// промежутка в place). Вертикаль в stub перед левым краем приёмника — то же в
// зеркале. Вход сверху спускается из коридора прямо в порт: в колонке
// приёмника над ним блоков нет, левее стоящие кончаются раньше.
func around(from, to point, approach string, blocks []box) [][]point {
	exit := from.x + stub
	var corridors []float64
	switch approach {
	case ir.SideControl:
		corridors = []float64{corridorAbove(blocks)}
	case ir.SideMechanism:
		corridors = []float64{corridorBelow(blocks)}
	default:
		corridors = []float64{corridorAbove(blocks), corridorBelow(blocks)}
	}

	var out [][]point
	for _, y := range corridors {
		path := []point{from, {x: exit, y: from.y}, {x: exit, y: y}}
		switch approach {
		case ir.SideIn:
			back := to.x - stub
			path = append(path, point{x: back, y: y}, point{x: back, y: to.y}, to)
		case toBorder:
			// Конец на краю листа вправе сменить высоту: уровни сшиты номером
			// узла, а не координатой.
			path = append(path, point{x: sheetRight, y: y})
		default: // управление, механизм: вертикалью в порт
			path = append(path, point{x: to.x, y: y}, to)
		}
		out = append(out, path)
	}
	return out
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

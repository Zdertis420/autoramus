package generate

import (
	"math"
	"sort"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Подписи стрелок: сколько места занимает имя и куда его поставить
// (specs/013-arrow-label-overlap).

// textWidth — ширина строки подписи: сумма ширин её знаков.
//
// Метрика, а не оценка. Прежде здесь стояло «4.3 на букву» — среднее по
// нескольким подписям Ramus, — и оно ошибалось в обе стороны: «Помидоры»
// выходили в 34.4 против настоящих 39. Ramus при загрузке переносит текст по
// ширине рамки, и слово, не влезшее в строку, режет по буквам — «Помидор|ы».
// Шрифт подписи — Dialog 8, ширины у него целые и складываются без
// кернинга, поэтому ширина строки — сумма ширин знаков из таблицы
// (glyphs.go): на тринадцати однострочных подписях двух моделей Ramus это
// совпадает с рамкой в файле до единицы.
func textWidth(s string) float64 {
	var width float64
	for _, r := range s {
		if w, ok := glyphWidth[r]; ok {
			width += w
			continue
		}
		width += glyphFallback
	}
	return width
}

// textLayout — одно разбиение имени на строки.
type textLayout struct {
	lines         []string
	width, height float64
}

// wordsOf режет имя на слова так, как переносит строки Ramus
// (PStringBounder, BreakIterator): по пробелам и после дефиса. Дефис остаётся
// с предшествующей частью: «Хмели-» / «сунели».
func wordsOf(name string) []string {
	var out []string
	for _, field := range strings.Fields(name) {
		for {
			i := strings.Index(field, "-")
			if i < 0 || i == len(field)-1 {
				break
			}
			out = append(out, field[:i+1])
			field = field[i+1:]
		}
		out = append(out, field)
	}
	return out
}

// joinWords склеивает слова строки: через пробел, а после дефиса — вплотную,
// как их склеил бы Ramus.
func joinWords(words []string) string {
	var b strings.Builder
	for i, w := range words {
		if i > 0 && !strings.HasSuffix(words[i-1], "-") {
			b.WriteByte(' ')
		}
		b.WriteString(w)
	}
	return b.String()
}

// layouts отдаёт разбиения имени на строки: сначала одна строка, затем две,
// три — по числу слов. У каждого числа строк берётся разбиение с самой узкой
// самой широкой строкой.
//
// Ширина раскладки не бывает меньше самого длинного слова — слово не режется, —
// и Ramus, получив рамку такой ширины, не перенесёт внутри слова. Переносит он
// жадно (LineBreakMeasurer), а не по нашей разбивке, но при той же ширине
// жадный перенос даёт строк не больше, чем любой другой: высота его рамки не
// превысит нашей, и рамка ляжет внутрь.
//
// Перебор точек разреза полный: слов в имени потока единицы.
func layouts(name string) []textLayout {
	words := wordsOf(name)
	if len(words) == 0 {
		return []textLayout{{lines: []string{name}, width: textWidth(name), height: sectorLineHeight}}
	}
	var out []textLayout
	for k := 1; k <= len(words); k++ {
		var best textLayout
		found := false
		eachSplit(len(words), k, func(cuts []int) {
			lines := make([]string, 0, k)
			width := 0.0
			from := 0
			for _, to := range append(cuts, len(words)) {
				line := joinWords(words[from:to])
				lines = append(lines, line)
				width = max(width, textWidth(line))
				from = to
			}
			// Строго уже — иначе первое найденное: разбиения перебираются в
			// одном и том же порядке, и выбор от запуска к запуску один.
			if !found || width < best.width {
				best = textLayout{lines: lines, width: width, height: float64(k) * sectorLineHeight}
				found = true
			}
		})
		out = append(out, best)
	}
	return out
}

// eachSplit перебирает разрезы n слов на k непустых строк по возрастанию:
// cuts — индексы слов, с которых начинаются строки со второй по k-ю.
func eachSplit(n, k int, visit func(cuts []int)) {
	cuts := make([]int, k-1)
	var walk func(i, from int)
	walk = func(i, from int) {
		if i == len(cuts) {
			visit(append([]int(nil), cuts...))
			return
		}
		// Каждой из оставшихся строк нужно хотя бы по слову.
		for c := from; c <= n-(len(cuts)-i); c++ {
			cuts[i] = c
			walk(i+1, c+1)
		}
	}
	walk(0, 1)
}

// Расстановка подписей.
//
// Подпись ставится у своей стрелки так, чтобы не лечь ни на другую подпись,
// ни на блок, ни за лист, и по возможности не на чужую линию. Перебор
// конечный и упорядоченный — как выбор полосы в раскладке (pick, lanes):
// никакого поиска пути, никакого солвера, и от запуска к запуску один
// результат.
const (
	// labelGap — отступ подписи от своей линии. Мера: у Ramus подписи стоят
	// со всех четырёх сторон линии в 0…6 единицах, обычно около трёх.
	labelGap = 3.0

	// labelReach — дальше этого от своей линии подпись не уносится. Мера:
	// самая далёкая подпись в моделях Ramus набора — в 34 единицах; дальше
	// читатель перестаёт понимать, чья она.
	labelReach = 34.0

	// labelStep — шаг, с которым подпись отходит от середины отрезка вдоль
	// него и от линии прочь. Решение, а не мера: взят просвет раскладки, 6.
	labelStep = 6.0

	// labelShifts — сколько шагов вдоль отрезка пробовать в каждую сторону.
	// Решение: 25 шагов по 6 — 150 единиц, как у разведения по каналам; на
	// длинной магистрали дальше подпись уходит от места, где её ищут глазами.
	labelShifts = 25

	// Лист: подпись за его край не заходит. Те же числа, что у раскладки.
	sheetLeft, sheetRight = 7.0, 793.0
	sheetTop, sheetBottom = 7.0, 437.0
)

type point struct{ x, y float64 }

// piece — прямой отрезок ломаной.
type piece struct{ a, b point }

func (p piece) horizontal() bool { return p.a.y == p.b.y }
func (p piece) length() float64  { return math.Abs(p.b.x-p.a.x) + math.Abs(p.b.y-p.a.y) }

// crosses — отрезок проходит внутри рамки. Касание не считается.
func (p piece) crosses(r label) bool {
	left, right := math.Min(p.a.x, p.b.x), math.Max(p.a.x, p.b.x)
	top, bottom := math.Min(p.a.y, p.b.y), math.Max(p.a.y, p.b.y)
	return right > r.x && left < r.x+r.width && bottom > r.y && top < r.y+r.height
}

// overlap — площадь пересечения двух прямоугольников.
func overlap(a, b label) float64 {
	w := math.Min(a.x+a.width, b.x+b.width) - math.Max(a.x, b.x)
	h := math.Min(a.y+a.height, b.y+b.height) - math.Max(a.y, b.y)
	if w <= 1e-6 || h <= 1e-6 {
		return 0
	}
	return w * h
}

func insideSheet(r label) bool {
	return r.x >= sheetLeft && r.y >= sheetTop &&
		r.x+r.width <= sheetRight && r.y+r.height <= sheetBottom
}

// wire — отрезок стрелки на диаграмме с именем её потока.
type wire struct {
	flow string
	piece
}

// board — всё, что подпись на одной диаграмме обязана или желает обойти.
type board struct {
	blocks []label
	wires  []wire
	placed []label
}

// pending — подпись, которую ещё предстоит поставить.
type pending struct {
	segment *ir.Segment
	flow    string
}

// placeLabels расставляет подписи всех диаграмм модели.
//
// Кто подписан — решает labeled (правило PR #4), и здесь это не меняется:
// меняются только место и рамка. Геометрия стрелок и блоков готова и не
// трогается.
func placeLabels(source *ir.Model) map[*ir.Segment]label {
	out := make(map[*ir.Segment]label)
	if source.Layout == nil {
		return out
	}

	// Диаграмма зовётся своим владельцем; у контекстной A-0 владельца-работы
	// нет, и она зовётся пустой строкой — так же, как в раскладке.
	diagramOf := func(seg *ir.Segment) string {
		if seg.Context {
			return ""
		}
		return seg.On.Name
	}
	owner := make(map[string]string, len(source.Functions))
	for _, f := range source.Functions {
		owner[f.Name.Name] = f.Of.Name
	}

	boards := make(map[string]*board)
	var diagrams []string
	boardOf := func(d string) *board {
		b, ok := boards[d]
		if !ok {
			b = &board{}
			boards[d] = b
			diagrams = append(diagrams, d)
		}
		return b
	}
	for _, f := range source.Layout.Functions {
		d, known := owner[f.Function.Name]
		if !known {
			continue
		}
		boardOf(d).blocks = append(boardOf(d).blocks,
			label{x: f.X.Val, y: f.Y.Val, width: f.Width.Val, height: f.Height.Val})
	}

	queues := make(map[string][]pending)
	for _, a := range source.Layout.Arrows {
		for _, seg := range a.Segments {
			d := diagramOf(seg)
			b := boardOf(d)
			for _, p := range pieces(seg) {
				b.wires = append(b.wires, wire{flow: a.Flow.Name, piece: p})
			}
			if labeled(seg) {
				queues[d] = append(queues[d], pending{segment: seg, flow: a.Flow.Name})
			}
		}
	}

	for _, d := range diagrams {
		b := boards[d]
		queue := queues[d]
		// Порядок — порядок стрелок в документе. Пробовали «самые стеснённые
		// первыми»: на всех документах набора итог тот же до единицы, и
		// простой порядок оставлен (specs/013-arrow-label-overlap, research Р-5).
		for _, p := range queue {
			r := b.choose(p)
			out[p.segment] = r
			b.placed = append(b.placed, r)
		}
	}
	return out
}

// pieces — прямые отрезки ломаной, вырожденные отброшены.
func pieces(seg *ir.Segment) []piece {
	var out []piece
	for i := 1; i < len(seg.Points); i++ {
		a := point{seg.Points[i-1].X.Val, seg.Points[i-1].Y.Val}
		b := point{seg.Points[i].X.Val, seg.Points[i].Y.Val}
		if a != b {
			out = append(out, piece{a, b})
		}
	}
	return out
}

// candidates перебирает места подписи в порядке предпочтения: отрезки своей
// линии от длинного к короткому, отступ от линии от ближнего к дальнему, от
// середины отрезка к краям, стороны — над, под, слева, справа. Первое место и
// есть прежнее — середина самого длинного отрезка, — только отодвинутое от
// линии, чтобы её не перечёркивала своя же стрелка.
//
// visit возвращает false, чтобы прекратить перебор.
func candidates(seg *ir.Segment, layout textLayout, visit func(label) bool) {
	own := pieces(seg)
	sort.SliceStable(own, func(i, j int) bool { return own[i].length() > own[j].length() })

	w, h := layout.width, layout.height
	for _, p := range own {
		lo := point{math.Min(p.a.x, p.b.x), math.Min(p.a.y, p.b.y)}
		hi := point{math.Max(p.a.x, p.b.x), math.Max(p.a.y, p.b.y)}
		for off := labelGap; off <= labelReach; off += labelStep {
			for n := 0; n <= 2*labelShifts; n++ {
				// 0, +1, −1, +2, −2 … шага от середины.
				shift := float64((n+1)/2) * labelStep
				if n%2 == 0 {
					shift = -shift
				}
				if p.horizontal() {
					x := (lo.x+hi.x)/2 - w/2 + shift
					if x+w <= lo.x || x >= hi.x {
						continue // рамка ушла с отрезка вбок
					}
					if !visit(label{x: x, y: lo.y - off - h, width: w, height: h}) ||
						!visit(label{x: x, y: lo.y + off, width: w, height: h}) {
						return
					}
					continue
				}
				y := (lo.y+hi.y)/2 - h/2 + shift
				if y+h <= lo.y || y >= hi.y {
					continue
				}
				if !visit(label{x: lo.x - off - w, y: y, width: w, height: h}) ||
					!visit(label{x: lo.x + off, y: y, width: w, height: h}) {
					return
				}
			}
		}
	}
}

// blocked — рамке мешает жёсткое препятствие: блок, край листа, уже
// поставленная подпись.
func (b *board) blocked(r label, labels bool) bool {
	if !insideSheet(r) {
		return true
	}
	for _, x := range b.blocks {
		if overlap(r, x) > 0 {
			return true
		}
	}
	if labels {
		for _, x := range b.placed {
			if overlap(r, x) > 0 {
				return true
			}
		}
	}
	return false
}

// soft — сколько линий рамка пересекает: чужих и, отдельно, своих. Чужие —
// мягкое препятствие (FR-007): их обходят, если можно. Свои — ещё мягче:
// подпись по определению стоит рядом с ними, но перечёркнутая своей же
// стрелкой читается хуже.
func (b *board) soft(r label, flow string) (foreign, own int) {
	for _, w := range b.wires {
		if !w.crosses(r) {
			continue
		}
		if w.flow == flow {
			own++
		} else {
			foreign++
		}
	}
	return foreign, own
}

// choose выбирает место подписи.
//
// Сначала — места без жёстких препятствий, с меньшим числом строк: две строки
// берутся, только если в одну места нет. Среди них — меньше всего чужих линий,
// затем своих; при равенстве — первое по порядку candidates.
//
// Места без жёстких препятствий нет вовсе — названная граница (FR-012):
// подпись остаётся у своей линии, в пределах labelReach, там, где наложение
// меньше по площади, а затем по числу чужих линий. Дальше она не уносится и
// тильдой со стрелкой не связывается: так решил автор, и так поступает и
// раскладка, когда чистой полосы нет.
func (b *board) choose(p pending) label {
	type score struct {
		area         float64
		foreign, own int
	}
	better := func(a, c score) bool {
		if a.area != c.area {
			return a.area < c.area
		}
		if a.foreign != c.foreign {
			return a.foreign < c.foreign
		}
		return a.own < c.own
	}

	all := layouts(p.flow)
	for _, layout := range all {
		var best label
		var bestScore score
		found := false
		candidates(p.segment, layout, func(r label) bool {
			if b.blocked(r, true) {
				return true
			}
			foreign, own := b.soft(r, p.flow)
			s := score{foreign: foreign, own: own}
			if !found || better(s, bestScore) {
				best, bestScore, found = r, s, true
			}
			// Лучше, чем ни одной линии, не бывает: первое такое место и есть
			// ответ.
			return foreign+own > 0
		})
		if found {
			return best
		}
	}

	// Названная граница.
	var best label
	var bestScore score
	found := false
	for _, layout := range all {
		candidates(p.segment, layout, func(r label) bool {
			if !insideSheet(r) {
				return true
			}
			var area float64
			for _, x := range b.blocks {
				area += overlap(r, x)
			}
			for _, x := range b.placed {
				area += overlap(r, x)
			}
			foreign, own := b.soft(r, p.flow)
			s := score{area: area, foreign: foreign, own: own}
			if !found || better(s, bestScore) {
				best, bestScore, found = r, s, true
			}
			return true
		})
	}
	if !found {
		// Даже внутри листа места нет — рамка шире листа. Ставим первое место
		// одной строкой: сказать больше нечего.
		candidates(p.segment, all[0], func(r label) bool { best = r; return false })
	}
	return best
}

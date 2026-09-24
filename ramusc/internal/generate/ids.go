package generate

import (
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Номера, которых в IR нет и быть не должно: они про устройство файла, а не
// про геометрию. Раздаёт их генератор, потому что он один знает, что уже
// лежит в таблицах.
//
// Ординаты и кросспоинты берутся из последовательностей в data/sequences.xml.
// Ramus эти две сам не перематывает (RSF-FORMAT.md §1), поэтому после записи
// их приходится увеличивать вручную — этим и кончается работа счётчика.

// counters раздаёт номера ординат и кросспоинтов.
type counters struct {
	file       *rsf.File
	ordinate   int64
	crosspoint int64

	// junctions — узлы, сшивающие уровни. Ключ — поток, работа и сторона:
	// конец на блоке родителя и конец на краю листа дочерней диаграммы это
	// один и тот же узел, и номер у них общий.
	//
	// Мера снята с настоящих файлов: в `тест.rsf` конец «контроля» на блоке
	// работы1 и начало того же «контроля» на краю её диаграммы несут cp=2.
	// Во входном языке этой сшивки нет — выражать её нечем, — поэтому
	// вычисляет её генератор.
	junctions map[junction]int64
}

// junction — узел между уровнями.
type junction struct {
	flow     string
	function string
	side     string
}

// newCounters читает последовательности файла.
func newCounters(file *rsf.File) (*counters, error) {
	seq, err := file.Sequences()
	if err != nil {
		return nil, err
	}
	ordinate, ok := seq[rsf.SequenceOrdinates]
	if !ok {
		return nil, fmt.Errorf("в заготовке нет последовательности %s", rsf.SequenceOrdinates)
	}
	crosspoint, ok := seq[rsf.SequenceCrosspoints]
	if !ok {
		return nil, fmt.Errorf("в заготовке нет последовательности %s", rsf.SequenceCrosspoints)
	}
	return &counters{
		file:       file,
		ordinate:   ordinate,
		crosspoint: crosspoint,
		junctions:  make(map[junction]int64),
	}, nil
}

// nextOrdinate выдаёт номер координатной линии.
func (c *counters) nextOrdinate() int64 {
	n := c.ordinate
	c.ordinate++
	return n
}

// nextCrosspoint выдаёт номер узла.
func (c *counters) nextCrosspoint() int64 {
	n := c.crosspoint
	c.crosspoint++
	return n
}

// junctionOf выдаёт номер узла для конца сегмента.
//
// Концы, которые сшивают уровни, получают общий номер; всем прочим достаётся
// свой. Узел опознаётся по потоку, работе и стороне ICOM: конец на стороне S
// блока F и конец на краю S диаграммы самой F — это одна точка перехода.
func (c *counters) junctionOf(flow string, seg *ir.Segment, e *ir.Endpoint) int64 {
	key, ok := junctionKeyOf(flow, seg, e)
	if !ok {
		return c.nextCrosspoint()
	}
	if n, found := c.junctions[key]; found {
		return n
	}
	n := c.nextCrosspoint()
	c.junctions[key] = n
	return n
}

// junctionKeyOf опознаёт конец, который может оказаться половиной перехода
// между уровнями.
//
// Конец на краю контекстной диаграммы половиной не бывает: выше A-0 уровня
// нет, сшивать не с чем.
func junctionKeyOf(flow string, seg *ir.Segment, e *ir.Endpoint) (junction, bool) {
	switch e.Kind() {
	case ir.EndpointFunction:
		return junction{flow: flow, function: e.Function.Name, side: e.Side}, true
	case ir.EndpointBorder:
		if seg.Context {
			return junction{}, false
		}
		return junction{flow: flow, function: seg.On.Name, side: icomOf(e.Border)}, true
	default:
		return junction{}, false
	}
}

// flush записывает выросшие последовательности обратно в файл.
func (c *counters) flush() error {
	if err := c.file.SetSequence(rsf.SequenceOrdinates, c.ordinate); err != nil {
		return err
	}
	return c.file.SetSequence(rsf.SequenceCrosspoints, c.crosspoint)
}

// lines раздаёт номера координатных линий одного потока на одной диаграмме.
//
// Точки с одинаковой координатой лежат на одной прямой и делят номер: так это
// устроено в настоящих файлах (сектор 36 «Изготовления юбки» — две пары точек
// на двух линиях), причём не только внутри сектора, но и между секторами одной
// стрелки — иначе узел ветвления не был бы для Ramus одной точкой. Между
// разными потоками линии не общие: у Ramus так не бывает ни разу, а общая
// направляющая потащила бы за собой чужую стрелку при правке.
type lines struct {
	counters *counters
	x, y     map[float64]int64
}

func newLines(c *counters) *lines {
	return &lines{counters: c, x: make(map[float64]int64), y: make(map[float64]int64)}
}

func (l *lines) ordinateX(v float64) int64 { return lineOf(l.x, v, l.counters) }
func (l *lines) ordinateY(v float64) int64 { return lineOf(l.y, v, l.counters) }

func lineOf(known map[float64]int64, v float64, c *counters) int64 {
	if n, ok := known[v]; ok {
		return n
	}
	n := c.nextOrdinate()
	known[v] = n
	return n
}

// sideOf переводит сторону ICOM в код Ramus.
func sideOf(side string) rsf.Side {
	switch side {
	case ir.SideIn:
		return rsf.SideLeft
	case ir.SideControl:
		return rsf.SideTop
	case ir.SideMechanism:
		return rsf.SideBottom
	default: // ir.SideOut
		return rsf.SideRight
	}
}

// borderCodeOf переводит край листа в код Ramus. Коды те же, что у сторон
// блока: у края нет роли ICOM, но словарь один.
func borderCodeOf(border string) rsf.Side {
	switch border {
	case ir.BorderLeft:
		return rsf.SideLeft
	case ir.BorderTop:
		return rsf.SideTop
	case ir.BorderBottom:
		return rsf.SideBottom
	default: // ir.BorderRight
		return rsf.SideRight
	}
}

// icomOf переводит край листа обратно в сторону ICOM: у граничной стрелки
// сторона при переходе на другой уровень не меняется, и это то, чем узел
// опознаётся.
func icomOf(border string) string {
	switch border {
	case ir.BorderLeft:
		return ir.SideIn
	case ir.BorderTop:
		return ir.SideControl
	case ir.BorderBottom:
		return ir.SideMechanism
	default: // ir.BorderRight
		return ir.SideOut
	}
}

// nodeIDs раздаёт номера узлам ветвления одной стрелки.
//
// Сегменты, сходящиеся в узле, зовут его одним именем, и номер у них общий:
// этим ветвление в файле и держится.
type nodeIDs struct {
	counters *counters
	known    map[string]int64
}

func newNodeIDs(c *counters) *nodeIDs {
	return &nodeIDs{counters: c, known: make(map[string]int64)}
}

func (n *nodeIDs) id(name string) int64 {
	if id, ok := n.known[name]; ok {
		return id
	}
	id := n.counters.nextCrosspoint()
	n.known[name] = id
	return id
}

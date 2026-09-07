package ir

import (
	"math"
	"strconv"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/syntax"
)

// Build опускает дерево разбора в IR. Функция тотальная и без диагностик:
// её дело — понижение, а не проверка. Коллекции сохраняются в порядке
// документа вместе с дублями, потому что дубль — это ошибка, о которой должен
// рассказать валидатор, а не молча съесть Build.
//
// Вызывать имеет смысл только после успешной проверки по схеме: тогда типы
// полей заведомо верные, и отсутствующее поле означает «не указано», а не
// «указано неправильно».
func Build(root *syntax.Node) *Model {
	c := cursor{node: root}
	m := &Model{Pos: c.pos()}

	m.Name = c.refField("model")
	m.Author = c.strField("author")
	m.Page = c.strField("page")

	for _, it := range c.list("flows") {
		m.Flows = append(m.Flows, it.ref())
	}
	for _, it := range c.list("streams") {
		m.Streams = append(m.Streams, buildStream(it))
	}
	for _, it := range c.list("functions") {
		f, sugar := buildFunction(it)
		m.Functions = append(m.Functions, f)
		m.Links = append(m.Links, sugar...)
	}
	for _, it := range c.list("links") {
		m.Links = append(m.Links, buildLink(it))
	}
	for _, it := range c.list("classifiers") {
		m.Classifiers = append(m.Classifiers, buildClassifier(it))
	}
	if l, ok := c.field("layout"); ok {
		m.Layout = buildLayout(l)
	}

	m.index()
	return m
}

func buildStream(c cursor) *Stream {
	return &Stream{
		Name: c.refField("name"),
		Note: c.strField("note"),
		Raw:  buildRaw(c),
		Path: c.path,
		Pos:  c.pos(),
	}
}

// buildFunction возвращает работу и связи, которые дал её ICOM-сахар: списки
// in/control/out/mechanism — это те же Link, что и в явной секции links (Р7).
func buildFunction(c cursor) (*Function, []*Link) {
	f := &Function{
		Name: c.refField("name"),
		Of:   c.refField("of"),
		Kind: c.refField("kind"),
		Type: c.refField("type"),
		Note: c.strField("note"),
		Raw:  buildRaw(c),
		Path: c.path,
		Pos:  c.pos(),
	}

	var links []*Link
	for _, side := range []string{SideIn, SideControl, SideMechanism} {
		for _, it := range c.list(side) {
			links = append(links, &Link{
				Flow:  it.ref(),
				To:    f.Name,
				Side:  side,
				Sugar: true,
				Path:  it.path,
				Pos:   it.pos(),
			})
		}
	}
	// У выхода стороны нет: сторона в Link описывает конец `to`.
	for _, it := range c.list("out") {
		links = append(links, &Link{
			Flow:  it.ref(),
			From:  f.Name,
			Sugar: true,
			Path:  it.path,
			Pos:   it.pos(),
		})
	}
	return f, links
}

func buildLink(c cursor) *Link {
	return &Link{
		Flow: c.refField("flow"),
		From: c.refField("from"),
		To:   c.refField("to"),
		Side: Normalize(c.strField("side")),
		Note: c.strField("note"),
		Path: c.path,
		Pos:  c.pos(),
	}
}

func buildClassifier(c cursor) *Classifier {
	cl := &Classifier{
		Name: c.refField("name"),
		Note: c.strField("note"),
		Path: c.path,
		Pos:  c.pos(),
	}
	for _, it := range c.list("columns") {
		cl.Columns = append(cl.Columns, &Column{
			Name: it.refField("name"),
			Type: it.refField("type"),
			Of:   it.refField("of"),
			Note: it.strField("note"),
			Path: it.path,
			Pos:  it.pos(),
		})
	}
	for _, it := range c.list("rows") {
		row := &Row{
			Note: it.strField("note"),
			Raw:  buildRaw(it),
			Path: it.path,
			Pos:  it.pos(),
		}
		for _, cellNode := range it.list("cells") {
			row.Cells = append(row.Cells, buildCell(cellNode))
		}
		cl.Rows = append(cl.Rows, row)
	}
	return cl
}

// buildCell держит позицию значения, а не ключа: сообщение о ячейке говорит
// о значении, туда же должен встать маркер.
func buildCell(c cursor) *Cell {
	cell := &Cell{
		Column: c.refField("column"),
		Path:   c.path,
		Pos:    c.pos(),
	}
	if v, ok := c.field("value"); ok {
		cell.Value = v.value()
		cell.Path = v.path
		cell.Pos = v.pos()
	}
	return cell
}

func buildLayout(c cursor) *Layout {
	l := &Layout{Path: c.path, Pos: c.pos()}
	for _, it := range c.list("functions") {
		l.Functions = append(l.Functions, &FunctionLayout{
			Function: it.refField("function"),
			X:        it.numField("x"),
			Y:        it.numField("y"),
			Width:    it.numField("width"),
			Height:   it.numField("height"),
			Path:     it.path,
			Pos:      it.pos(),
		})
	}
	for _, it := range c.list("arrows") {
		a := &ArrowLayout{
			Flow:    it.refField("flow"),
			On:      it.refField("on"),
			Context: it.boolField("context"),
			From:    it.refField("from"),
			To:      it.refField("to"),
			Path:    it.path,
			Pos:     it.pos(),
		}
		for _, p := range it.list("points") {
			coords := p.items()
			pt := Point{Path: p.path, Pos: p.pos()}
			if len(coords) > 0 {
				pt.X = coords[0].num()
			}
			if len(coords) > 1 {
				pt.Y = coords[1].num()
			}
			a.Points = append(a.Points, pt)
		}
		l.Arrows = append(l.Arrows, a)
	}
	return l
}

func buildRaw(c cursor) []RawAttr {
	var out []RawAttr
	for _, it := range c.list("raw") {
		attr := RawAttr{
			Attribute: it.refField("attribute"),
			Path:      it.path,
			Pos:       it.pos(),
		}
		if v, ok := it.field("value"); ok {
			attr.Value = v.value()
		}
		out = append(out, attr)
	}
	return out
}

// cursor — узел документа вместе с путём до него. Путь нужен диагностике:
// GUI ветвится и по нему тоже.
type cursor struct {
	node *syntax.Node
	path string
}

func (c cursor) field(key string) (cursor, bool) {
	if c.node == nil || c.node.Kind != syntax.Object {
		return cursor{}, false
	}
	v := c.node.Field(key)
	if v == nil {
		return cursor{}, false
	}
	return cursor{node: v, path: c.path + "/" + syntax.EscapeToken(key)}, true
}

// items разворачивает массив. Не массив и отсутствующий узел дают пустой
// список: разбирать такое — дело схемы, до сюда оно не доходит.
func (c cursor) items() []cursor {
	if c.node == nil || c.node.Kind != syntax.Array {
		return nil
	}
	out := make([]cursor, len(c.node.Items))
	for i, it := range c.node.Items {
		out[i] = cursor{node: it, path: c.path + "/" + strconv.Itoa(i)}
	}
	return out
}

func (c cursor) list(key string) []cursor {
	f, ok := c.field(key)
	if !ok {
		return nil
	}
	return f.items()
}

func (c cursor) str() string {
	if c.node == nil || c.node.Kind != syntax.String {
		return ""
	}
	return c.node.Str
}

func (c cursor) strField(key string) string {
	f, ok := c.field(key)
	if !ok {
		return ""
	}
	return f.str()
}

func (c cursor) ref() Ref {
	raw := c.str()
	return Ref{Name: Normalize(raw), Raw: raw, Path: c.path, Pos: c.pos()}
}

func (c cursor) refField(key string) Ref {
	f, ok := c.field(key)
	if !ok {
		return Ref{}
	}
	return f.ref()
}

// num переводит число документа в float64. Значение, не влезающее в double
// (например 1e999), помечается как NaN — о такой геометрии валидатор скажет
// отдельно.
func (c cursor) num() Num {
	if c.node == nil || c.node.Kind != syntax.Number {
		return Num{Path: c.path, Pos: c.pos()}
	}
	v, err := c.node.Num.Float64()
	if err != nil {
		v = math.NaN()
	}
	return Num{Val: v, Set: true, Path: c.path, Pos: c.pos()}
}

func (c cursor) numField(key string) Num {
	f, ok := c.field(key)
	if !ok {
		return Num{}
	}
	return f.num()
}

func (c cursor) boolField(key string) bool {
	f, ok := c.field(key)
	if !ok || f.node == nil || f.node.Kind != syntax.Bool {
		return false
	}
	return f.node.Bool
}

func (c cursor) value() any {
	if c.node == nil {
		return nil
	}
	return c.node.Value()
}

func (c cursor) pos() diag.Pos {
	if c.node == nil {
		return diag.Pos{}
	}
	return c.node.Pos
}

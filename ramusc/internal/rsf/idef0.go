package rsf

import (
	"fmt"
	"sort"
)

// Ветки версионирования строк (RSF-FORMAT.md §2): удаление объекта — это
// пометка, а не стирание, поэтому живые строки приходится отбирать явно.
const (
	AliveBranch = "2147483647"
	DeadBranch  = "0"
)

// Типы функциональных блоков (F_TYPE, com.ramussoft.pb.Function).
const (
	TypeProcessComplex    = 0
	TypeProcess           = 1
	TypeProcessPart       = 2
	TypeOperation         = 3
	TypeAction            = 4
	TypeExternalReference = 1001
	TypeDataStore         = 1002
	TypeDFDSRole          = 1003
)

// Side — сторона блока (FUNCTION_TYPE, MovingPanel). В IDEF0 стороны и есть ICOM.
type Side int

const (
	SideRight  Side = 0 // выход
	SideBottom Side = 1 // механизм
	SideLeft   Side = 2 // вход
	SideTop    Side = 3 // управление
)

// ICOM отдаёт имя стороны так, как оно называется во входном языке.
func (s Side) ICOM() string {
	switch s {
	case SideLeft:
		return "in"
	case SideTop:
		return "control"
	case SideBottom:
		return "mechanism"
	case SideRight:
		return "out"
	default:
		return "?"
	}
}

func (s Side) String() string {
	switch s {
	case SideLeft:
		return "вход(L)"
	case SideTop:
		return "управление(T)"
	case SideBottom:
		return "механизм(B)"
	case SideRight:
		return "выход(R)"
	default:
		return "?"
	}
}

// Rect — F_BOUNDS работы.
type Rect struct{ X, Y, Width, Height float64 }

// Function — работа модели.
type Function struct {
	ID       int64
	Name     string
	Parent   int64 // -1, если не задан
	Previous int64 // предыдущий брат, -1 у первого
	Type     int
	Bounds   *Rect
}

// Stream — поток, то есть стрелка-сущность.
type Stream struct {
	ID   int64
	Name string
}

// Border — конец сегмента стрелки.
type Border struct {
	Function     int64 // ≥ 0 — конец прицеплен к блоку
	FunctionType Side
	BorderType   int   // ≥ 0 — конец на краю листа
	Crosspoint   int64 // узел ветвления или слияния
	TunnelSoft   bool  // туннель: стрелка намеренно не переходит на другой уровень
}

// OnFunction сообщает, что конец прицеплен к блоку.
func (b *Border) OnFunction() bool { return b != nil && b.Function >= 0 }

// OnBorder сообщает, что конец лежит на краю листа.
func (b *Border) OnBorder() bool { return b != nil && b.Function < 0 && b.BorderType >= 0 }

// Point — точка ломаной сегмента.
type Point struct {
	Position  int64
	Type      int64
	X, Y      float64
	XOrdinate int64 // общие «направляющие», по которым выравниваются соседние стрелки
	YOrdinate int64
}

// Sector — сегмент стрелки на одной диаграмме.
type Sector struct {
	ID      int64
	Diagram int64 // на чьей диаграмме нарисован: F_FUNCTION_SECTOR
	Stream  int64 // какой поток несёт: F_SECTOR_STREAM
	Start   *Border
	End     *Border
	Points  []Point
}

// Model — модель IDEF0 поверх таблиц. Квалификаторы и атрибуты ищутся по имени:
// числовые id стабильны только для типичного файла, полагаться на них не стоит
// (RSF-FORMAT.md §4).
type Model struct {
	File *File

	// Element — элемент модели из F_BASE_FUNCTIONS. Для контекстной диаграммы
	// A-0 именно он играет роль владельца сегментов.
	Element int64
	// FunctionQualifier — квалификатор, в котором лежат работы модели.
	FunctionQualifier int64
	// NameAttribute — атрибут, играющий роль имени работы (обычно «Название»).
	NameAttribute int64

	attributes map[string]string
	qualifiers map[string]string

	elements, texts, hierarchicals, longs, others *Table
	rectangles, functionTypes                     *Table
	borders, points                               *Table
}

// NewModel собирает фасад над уже прочитанным файлом.
func NewModel(f *File) (*Model, error) {
	m := &Model{File: f}

	var err error
	tables := []struct {
		name string
		dst  **Table
	}{
		{"elements", &m.elements},
		{"attribute_texts", &m.texts},
		{"attribute_hierarchicals", &m.hierarchicals},
		{"attribute_longs", &m.longs},
		{"attribute_other_elements", &m.others},
		{"attribute_rectangles", &m.rectangles},
		{"attribute_function_types", &m.functionTypes},
		{"attribute_sector_borders", &m.borders},
		{"attribute_sector_points", &m.points},
	}
	for _, t := range tables {
		if *t.dst, err = f.Table(t.name); err != nil {
			return nil, err
		}
	}

	attributes, err := f.Table("attributes")
	if err != nil {
		return nil, err
	}
	qualifiers, err := f.Table("qualifiers")
	if err != nil {
		return nil, err
	}
	m.attributes = index(attributes, "ATTRIBUTE_NAME", "ATTRIBUTE_ID")
	m.qualifiers = index(qualifiers, "QUALIFIER_NAME", "QUALIFIER_ID")

	if err := m.findModel(qualifiers); err != nil {
		return nil, err
	}
	return m, nil
}

func index(t *Table, key, value string) map[string]string {
	out := make(map[string]string, len(t.Rows))
	for _, row := range t.Rows {
		out[t.Str(row, key)] = t.Str(row, value)
	}
	return out
}

// findModel ищет модель так, как описано в §4: берём элемент из
// F_BASE_FUNCTIONS, у которого F_BASE_FUNCTION_QUALIFIER_ID указывает
// не на сам F_BASE_FUNCTIONS, — он и называет квалификатор работ.
func (m *Model) findModel(qualifiers *Table) error {
	base, ok := m.qualifiers["F_BASE_FUNCTIONS"]
	if !ok {
		return fmt.Errorf("в файле нет квалификатора F_BASE_FUNCTIONS")
	}
	attr, ok := m.attributes["F_BASE_FUNCTION_QUALIFIER_ID"]
	if !ok {
		return fmt.Errorf("в файле нет атрибута F_BASE_FUNCTION_QUALIFIER_ID")
	}

	for _, el := range m.elements.Select(Eq("QUALIFIER_ID", base), Eq("REMOVED_BRANCH_ID", AliveBranch)) {
		id := m.elements.Str(el, "ELEMENT_ID")
		for _, row := range m.longs.Select(Eq("ATTRIBUTE_ID", attr), Eq("ELEMENT_ID", id)) {
			value := m.longs.Str(row, "VALUE")
			if value == base {
				continue // служебная «пустая» модель
			}
			m.Element = parseID(id)
			m.FunctionQualifier = parseID(value)
		}
	}
	if m.FunctionQualifier == 0 {
		return fmt.Errorf("в файле не нашлось модели с квалификатором работ")
	}

	row, ok := qualifiers.First(Eq("QUALIFIER_ID", fmt.Sprint(m.FunctionQualifier)))
	if !ok {
		return fmt.Errorf("квалификатор работ %d не описан", m.FunctionQualifier)
	}
	m.NameAttribute = parseID(qualifiers.Str(row, "ATTRIBUTE_FOR_NAME"))
	return nil
}

// Attribute отдаёт идентификатор системного атрибута по имени, например
// F_BOUNDS. Второе значение — нашёлся ли он вообще.
//
// Открыто наружу ради генератора: номера атрибутов свои в каждом файле, и
// искать их надо по имени. Заводить второй такой поиск рядом значило бы
// держать одно знание в двух местах.
func (m *Model) Attribute(name string) (int64, bool) {
	raw, ok := m.attributes[name]
	if !ok {
		return -1, false
	}
	return parseID(raw), true
}

// QualifierName отдаёт имя квалификатора работ — им же названа модель в дереве.
func (m *Model) QualifierName() string {
	qualifiers, err := m.File.Table("qualifiers")
	if err != nil {
		return ""
	}
	row, ok := qualifiers.First(Eq("QUALIFIER_ID", fmt.Sprint(m.FunctionQualifier)))
	if !ok {
		return ""
	}
	return qualifiers.Str(row, "QUALIFIER_NAME")
}

// text читает значение текстового атрибута элемента.
func (m *Model) text(element, attribute int64) string {
	row, ok := m.texts.First(
		Eq("ELEMENT_ID", fmt.Sprint(element)),
		Eq("ATTRIBUTE_ID", fmt.Sprint(attribute)))
	if !ok {
		return ""
	}
	return m.texts.Str(row, "VALUE")
}

// aliveOf отбирает живые элементы квалификатора, отсортированные по id.
func (m *Model) aliveOf(qualifier string) []int64 {
	var ids []int64
	for _, el := range m.elements.Select(Eq("QUALIFIER_ID", qualifier), Eq("REMOVED_BRANCH_ID", AliveBranch)) {
		ids = append(ids, parseID(m.elements.Str(el, "ELEMENT_ID")))
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// Functions отдаёт работы модели в порядке идентификаторов.
func (m *Model) Functions() []Function {
	nameAttr := m.NameAttribute
	hierAttr := m.attributes["HierarchicalAttribute"]

	var out []Function
	for _, id := range m.aliveOf(fmt.Sprint(m.FunctionQualifier)) {
		f := Function{ID: id, Name: m.text(id, nameAttr), Parent: -1, Previous: -1, Type: -1}

		if row, ok := m.hierarchicals.First(Eq("ELEMENT_ID", fmt.Sprint(id)), Eq("ATTRIBUTE_ID", hierAttr)); ok {
			f.Parent = m.hierarchicals.Int64Or(row, "PARENT_ELEMENT_ID", -1)
			f.Previous = m.hierarchicals.Int64Or(row, "PREVIOUS_ELEMENT_ID", -1)
		}
		if row, ok := m.rectangles.First(Eq("ELEMENT_ID", fmt.Sprint(id))); ok {
			x, _ := m.rectangles.Float(row, "X")
			y, _ := m.rectangles.Float(row, "Y")
			w, _ := m.rectangles.Float(row, "WIDTH")
			h, _ := m.rectangles.Float(row, "HEIGHT")
			f.Bounds = &Rect{X: x, Y: y, Width: w, Height: h}
		}
		if row, ok := m.functionTypes.First(Eq("ELEMENT_ID", fmt.Sprint(id))); ok {
			f.Type = int(m.functionTypes.Int64Or(row, "TYPE", -1))
		}
		out = append(out, f)
	}
	return out
}

// Streams отдаёт потоки модели. Имя потока лежит в системном F_STREAM_NAME,
// а не в elements.ELEMENT_NAME.
func (m *Model) Streams() []Stream {
	attr := parseID(m.attributes["F_STREAM_NAME"])
	var out []Stream
	for _, id := range m.aliveOf(m.qualifiers["F_STREAMS"]) {
		out = append(out, Stream{ID: id, Name: m.text(id, attr)})
	}
	return out
}

// Sectors отдаёт сегменты стрелок: по одному на каждый нарисованный кусок
// на каждой диаграмме.
func (m *Model) Sectors() []Sector {
	functionAttr := m.attributes["F_FUNCTION_SECTOR"]
	streamAttr := m.attributes["F_SECTOR_STREAM"]
	startAttr := m.attributes["F_SECTOR_BORDER_START"]
	endAttr := m.attributes["F_SECTOR_BORDER_END"]

	var out []Sector
	for _, id := range m.aliveOf(m.qualifiers["F_SECTORS"]) {
		key := fmt.Sprint(id)
		s := Sector{ID: id, Diagram: -1, Stream: -1}

		if row, ok := m.others.First(Eq("ELEMENT_ID", key), Eq("ATTRIBUTE_ID", functionAttr)); ok {
			s.Diagram = m.others.Int64Or(row, "OTHER_ELEMENT", -1)
		}
		if row, ok := m.others.First(Eq("ELEMENT_ID", key), Eq("ATTRIBUTE_ID", streamAttr)); ok {
			s.Stream = m.others.Int64Or(row, "OTHER_ELEMENT", -1)
		}
		s.Start = m.border(key, startAttr)
		s.End = m.border(key, endAttr)
		s.Points = m.sectorPoints(key)
		out = append(out, s)
	}
	return out
}

// border читает конец сегмента. Строки может не быть вовсе — тогда конец
// «висит» неприсоединённым.
func (m *Model) border(element, attribute string) *Border {
	row, ok := m.borders.First(Eq("ELEMENT_ID", element), Eq("ATTRIBUTE_ID", attribute))
	if !ok {
		return nil
	}
	tunnel, _ := m.borders.Int64(row, "TUNNEL_SOFT")
	return &Border{
		Function:     m.borders.Int64Or(row, "FUNCTION", -1),
		FunctionType: Side(m.borders.Int64Or(row, "FUNCTION_TYPE", -1)),
		BorderType:   int(m.borders.Int64Or(row, "BORDER_TYPE", -1)),
		Crosspoint:   m.borders.Int64Or(row, "CROSSPOINT", -1),
		TunnelSoft:   tunnel != 0,
	}
}

func (m *Model) sectorPoints(element string) []Point {
	var out []Point
	for _, row := range m.points.Select(Eq("ELEMENT_ID", element)) {
		x, _ := m.points.Float(row, "X_POSITION")
		y, _ := m.points.Float(row, "Y_POSITION")
		out = append(out, Point{
			Position:  m.points.Int64Or(row, "POSITION", -1),
			Type:      m.points.Int64Or(row, "POINT_TYPE", -1),
			X:         x,
			Y:         y,
			XOrdinate: m.points.Int64Or(row, "X_ORDINATE_ID", -1),
			YOrdinate: m.points.Int64Or(row, "Y_ORDINATE_ID", -1),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out
}

// ICOM — стрелки, прицепленные к работе, разложенные по сторонам. Это то
// представление, в котором модель сверяется с входным языком: у работы IDEF0
// должны быть все четыре стороны.
type ICOM struct {
	In        []string
	Control   []string
	Mechanism []string
	Out       []string
}

// Side отдаёт список имён потоков для стороны.
func (i *ICOM) Side(s Side) []string {
	switch s {
	case SideLeft:
		return i.In
	case SideTop:
		return i.Control
	case SideBottom:
		return i.Mechanism
	case SideRight:
		return i.Out
	default:
		return nil
	}
}

func (i *ICOM) add(s Side, flow string) {
	list := i.Side(s)
	for _, existing := range list {
		if existing == flow {
			return
		}
	}
	switch s {
	case SideLeft:
		i.In = append(i.In, flow)
	case SideTop:
		i.Control = append(i.Control, flow)
	case SideBottom:
		i.Mechanism = append(i.Mechanism, flow)
	case SideRight:
		i.Out = append(i.Out, flow)
	}
}

// ICOM собирает стороны всех работ по концам сегментов, прицепленным к блокам.
func (m *Model) ICOM() map[int64]*ICOM {
	names := make(map[int64]string)
	for _, s := range m.Streams() {
		names[s.ID] = s.Name
	}

	out := make(map[int64]*ICOM)
	for _, sector := range m.Sectors() {
		flow, ok := names[sector.Stream]
		if !ok {
			continue
		}
		for _, end := range []*Border{sector.Start, sector.End} {
			if !end.OnFunction() {
				continue
			}
			if out[end.Function] == nil {
				out[end.Function] = &ICOM{}
			}
			out[end.Function].add(end.FunctionType, flow)
		}
	}
	return out
}

func parseID(s string) int64 {
	var n int64
	if _, err := fmt.Sscan(s, &n); err != nil {
		return -1
	}
	return n
}

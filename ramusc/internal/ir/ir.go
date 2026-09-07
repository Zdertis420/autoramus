// Пакет ir — промежуточное представление модели: работы, потоки, связи,
// классификаторы и раскладка, отвязанные от синтаксиса (Р16). Смысловые
// проверки, раскладка и генератор .rsf знают только про него, поэтому смена
// фронтенда их не задевает.
//
// Каждое имя в IR — это Ref: нормализованное имя, исходное написание и позиция
// в документе. Без позиции валидатор не поставит маркер на поле редактора GUI,
// а восстанавливать её потом по JSON Pointer мучительно.
package ir

import (
	"encoding/json"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

// Виды диаграммы-декомпозиции (поле kind у работы, Р8).
const (
	KindIDEF0 = "idef0"
	KindDFD   = "dfd"
)

// Виды элемента (поле type у работы, Р8). Всё, кроме process, живёт только
// на диаграмме DFD.
const (
	TypeProcess  = "process"
	TypeStore    = "store"
	TypeExternal = "external"
	TypeRole     = "role"
)

// Стороны блока. В связи значение относится к концу `to`: конец `from` —
// всегда выход, так устроен ICOM. В геометрии стороной помечается любой конец,
// поэтому там возможен и SideOut.
const (
	SideIn        = "in"
	SideControl   = "control"
	SideMechanism = "mechanism"
	SideOut       = "out"
)

// Края листа (поле border у конца сегмента). Названия геометрические: у края
// нет роли ICOM, он просто сторона диаграммы.
const (
	BorderLeft   = "left"
	BorderRight  = "right"
	BorderTop    = "top"
	BorderBottom = "bottom"
)

// Типы колонок классификатора (Р8); словарь повторяет типы Ramus.
const (
	ColumnText   = "text"
	ColumnNumber = "number"
	ColumnLong   = "long"
	ColumnDate   = "date"
	ColumnBool   = "bool"
	ColumnRef    = "ref"
	ColumnList   = "list"
)

// Ref — имя вместе с тем, откуда оно взято.
type Ref struct {
	Name string   // нормализованное имя: оно и есть идентификатор (Р4)
	Raw  string   // как написано в документе — для текста сообщения
	Path string   // JSON Pointer до значения
	Pos  diag.Pos // позиция значения в документе
}

// Set сообщает, было ли имя вообще указано.
func (r Ref) Set() bool { return r.Path != "" }

// Num — число вместе с позицией: координаты приходят из документа, и ошибку
// вроде бесконечности надо показать на самом числе.
type Num struct {
	Val  float64
	Set  bool
	Path string
	Pos  diag.Pos
}

// RawAttr — запись люка Р9: атрибут Ramus и его значение как есть.
type RawAttr struct {
	Attribute Ref
	Value     any
	Path      string
	Pos       diag.Pos
}

// Model — документ целиком.
type Model struct {
	Name        Ref
	Author      string
	Page        string
	Flows       []Ref
	Streams     []*Stream
	Functions   []*Function
	Links       []*Link
	Classifiers []*Classifier
	Layout      *Layout

	Pos diag.Pos

	byFunction   map[string]*Function
	byClassifier map[string]*Classifier
}

// Function — работа (функциональный блок) или элемент DFD.
type Function struct {
	Name Ref
	Of   Ref // не задано — корневая работа контекстной диаграммы
	Kind Ref // idef0 | dfd; не задано — idef0
	Type Ref // process | store | external | role; не задано — process
	Note string
	Raw  []RawAttr

	Path string
	Pos  diag.Pos
}

// KindName — вид декомпозиции с учётом умолчания.
func (f *Function) KindName() string {
	if f.Kind.Name == "" {
		return KindIDEF0
	}
	return f.Kind.Name
}

// TypeName — вид элемента с учётом умолчания.
func (f *Function) TypeName() string {
	if f.Type.Name == "" {
		return TypeProcess
	}
	return f.Type.Name
}

// Stream — атрибуты потока (Р12). Сам словарь имён — это Model.Flows.
type Stream struct {
	Name Ref
	Note string
	Raw  []RawAttr

	Path string
	Pos  diag.Pos
}

// Link — связь. Обе формы записи, ICOM и явная секция links, опускаются
// сюда же (Р7). Отсутствующий конец означает границу листа.
type Link struct {
	Flow Ref
	From Ref
	To   Ref
	Side string // сторона у конца To; пусто — по умолчанию in
	Note string

	// Sugar — связь пришла из списков in/control/out/mechanism, а не из
	// секции links. Влияет только на текст сообщений.
	Sugar bool

	Path string
	Pos  diag.Pos
}

// SideName — сторона потребителя с учётом умолчания.
func (l *Link) SideName() string {
	if l.Side == "" {
		return SideIn
	}
	return l.Side
}

// Classifier — таблица классификатора Ramus (Р8).
type Classifier struct {
	Name    Ref
	Columns []*Column
	Rows    []*Row
	Note    string

	Path string
	Pos  diag.Pos
}

// Column возвращает колонку по нормализованному имени.
func (c *Classifier) Column(name string) *Column {
	for _, col := range c.Columns {
		if col.Name.Name == name {
			return col
		}
	}
	return nil
}

// Column — колонка классификатора.
type Column struct {
	Name Ref
	Type Ref // text | number | long | date | bool | ref | list
	Of   Ref // цель ref или list: классификатор; не задано — работа
	Note string

	Path string
	Pos  diag.Pos
}

// Row — строка классификатора.
type Row struct {
	Cells []*Cell
	Note  string
	Raw   []RawAttr

	Path string
	Pos  diag.Pos
}

// Cell — значение в строке. Значение хранится как есть (string, json.Number,
// bool или []any): подходит ли оно типу колонки, решает смысловая проверка.
type Cell struct {
	Column Ref
	Value  any

	Path string
	Pos  diag.Pos // позиция значения, а не ключа: маркер нужен на значении
}

// Layout — оверрайд автораскладки (Р2).
type Layout struct {
	Functions []*FunctionLayout
	Arrows    []*ArrowLayout

	Path string
	Pos  diag.Pos
}

// FunctionLayout — координаты и размер блока.
type FunctionLayout struct {
	Function Ref
	X, Y     Num
	Width    Num
	Height   Num

	Path string
	Pos  diag.Pos
}

// ArrowLayout — геометрия одной стрелки: сегменты, сшитые узлами. Ординаты
// Ramus сюда не попадают: это способ выравнивать соседние стрелки, дело
// компилятора, а не автора (Р1).
type ArrowLayout struct {
	Flow     Ref
	Segments []*Segment

	Path string
	Pos  diag.Pos
}

// Segment — один нарисованный кусок стрелки на одной диаграмме.
type Segment struct {
	On Ref // чья диаграмма: работа или, для контекстной, имя модели
	// Context — сегмент лежит на контекстной диаграмме A-0. Корневая работа
	// и модель зовутся одинаково, и без этой пометки диаграмма A-0
	// неотличима от декомпозиции корневой работы.
	Context bool
	// From и To равны nil, когда конец висит неприсоединённым: в моделях
	// Ramus строки границы у такого конца нет вовсе.
	From   *Endpoint
	To     *Endpoint
	Points []Point

	Path string
	Pos  diag.Pos
}

// Endpoint — конец сегмента: блок, край листа или узел. Ровно одно из трёх;
// что именно, говорит Kind.
type Endpoint struct {
	Function Ref
	Side     string // сторона блока: in | control | mechanism | out
	Border   string // край листа: left | right | top | bottom
	Node     Ref    // узел ветвления; область видимости — одна стрелка
	Tunnel   bool   // туннельная стрелка: конец в скобках

	Path string
	Pos  diag.Pos
}

// Виды конца сегмента.
const (
	EndpointNone     = ""
	EndpointFunction = "function"
	EndpointBorder   = "border"
	EndpointNode     = "node"
)

// Kind сообщает, чем конец является. Если заполнено больше одного поля,
// об этом скажет валидатор, а Kind назовёт первое по порядку.
func (e *Endpoint) Kind() string {
	switch {
	case e == nil:
		return EndpointNone
	case e.Function.Set():
		return EndpointFunction
	case e.Border != "":
		return EndpointBorder
	case e.Node.Set():
		return EndpointNode
	default:
		return EndpointNone
	}
}

// Point — точка ломаной.
type Point struct {
	X, Y Num

	Path string
	Pos  diag.Pos
}

// Function возвращает работу по нормализованному имени. При дублях побеждает
// первая по документу: сам дубль — отдельная ошибка валидатора.
func (m *Model) Function(name string) *Function { return m.byFunction[name] }

// Classifier возвращает классификатор по нормализованному имени.
func (m *Model) Classifier(name string) *Classifier { return m.byClassifier[name] }

// Index перестраивает карты имён. Build зовёт её сам; декомпилятору,
// который собирает Model руками, приходится звать её самому.
func (m *Model) Index() { m.index() }

// index собирает карты имён. Вызывается в конце Build.
func (m *Model) index() {
	m.byFunction = make(map[string]*Function, len(m.Functions))
	for _, f := range m.Functions {
		if _, dup := m.byFunction[f.Name.Name]; !dup {
			m.byFunction[f.Name.Name] = f
		}
	}
	m.byClassifier = make(map[string]*Classifier, len(m.Classifiers))
	for _, c := range m.Classifiers {
		if _, dup := m.byClassifier[c.Name.Name]; !dup {
			m.byClassifier[c.Name.Name] = c
		}
	}
}

// Normalize приводит имя к каноническому виду: пробелы по краям снимаются,
// внутренние схлопываются в один. Регистр не трогается — так решено в Р4.
func Normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// IsInt сообщает, что значение — целое число без дробной части и порядка.
// Нужно для колонок типа long.
func IsInt(v any) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	_, err := n.Int64()
	return err == nil
}

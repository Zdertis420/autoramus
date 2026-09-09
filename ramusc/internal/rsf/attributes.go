package rsf

import (
	"sort"
	"strings"
)

// Опись атрибутов элемента: что вообще лежит в файле про этот элемент,
// независимо от того, что мы умеем читать.
//
// Нужна, чтобы обнаруживать потери вычитанием, а не перечислением. Перечень
// «чего мы не умеем» верен ровно до появления незнакомого атрибута и молча
// пропускает всё остальное — так терялись классификаторы.

// ElementKind — вид элемента модели.
type ElementKind int

const (
	// KindOther — элемент метамодели: описание атрибута, квалификатора,
	// дерево модели, иконки. Данными автора не является.
	KindOther ElementKind = iota
	KindFunction
	KindStream
	KindSector
	KindClassifierRow
	KindModel
)

func (k ElementKind) String() string {
	switch k {
	case KindFunction:
		return "работа"
	case KindStream:
		return "поток"
	case KindSector:
		return "сектор"
	case KindClassifierRow:
		return "строка справочника"
	case KindModel:
		return "модель"
	default:
		return "служебное"
	}
}

// Attribute — один атрибут одного элемента вместе со значением.
type Attribute struct {
	ID      int64
	Name    string
	Table   string // имя таблицы значений, напр. attribute_statuses
	Element int64
	// Columns — значение по колонкам. Ключи записи сюда не входят: это адрес
	// значения, а не оно само.
	Columns map[string]string
}

// keyColumns — колонки, которые адресуют значение, а не несут его. В опись не
// попадают: иначе люк заполнится служебными числами и читать его станет нельзя.
var keyColumns = map[string]bool{
	"ATTRIBUTE_ID":      true,
	"ELEMENT_ID":        true,
	"ELEMENT1_ID":       true,
	"VALUE_BRANCH_ID":   true,
	"CREATED_BRANCH_ID": true,
	"REMOVED_BRANCH_ID": true,
}

// nonElementTables — таблицы `attribute_*`, описывающие не элемент, а сам
// атрибут: куда ссылается колонка справочника. Ключа элемента у них нет, и в
// опись они не входят — но перечислены явно, чтобы проверка полноты (см.
// attributes_test.go) отличала их от забытых.
var nonElementTables = map[string]bool{
	"attribute_other_element_properties": true,
	"attribute_element_list_properties":  true,
}

// Attributes отдаёт все атрибуты элемента со значениями.
func (m *Model) Attributes(element int64) []Attribute {
	names := m.attributeNames()
	id := formatID(element)

	var out []Attribute
	for _, t := range m.File.Tables {
		if !strings.HasPrefix(t.Name, "attribute_") || nonElementTables[t.Name] {
			continue
		}
		key := elementKeyColumn(t)
		if key == "" {
			continue
		}

		for _, row := range t.Select(Eq(key, id)) {
			if removed, ok := t.Column("REMOVED_BRANCH_ID"); ok {
				if v := t.Str(row, removed.Name); v != "" && v != AliveBranch {
					continue
				}
			}
			aid := parseID(t.Str(row, "ATTRIBUTE_ID"))
			out = append(out, Attribute{
				ID:      aid,
				Name:    names[aid],
				Table:   t.Name,
				Element: element,
				Columns: valueColumns(t, row),
			})
		}
	}

	// Порядок задаётся явно: обход Tables — обход отображения, а вывод обязан
	// быть детерминированным.
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Table < out[j].Table
	})
	return out
}

// elementKeyColumn находит колонку, которой таблица адресует элемент.
func elementKeyColumn(t *Table) string {
	for _, name := range []string{"ELEMENT_ID", "ELEMENT1_ID"} {
		if _, ok := t.Column(name); ok {
			return name
		}
	}
	return ""
}

// valueColumns собирает значение по колонкам, отбрасывая ключи записи.
func valueColumns(t *Table, row Row) map[string]string {
	out := make(map[string]string, len(t.Fields))
	for _, f := range t.Fields {
		if keyColumns[strings.ToUpper(f.Name)] {
			continue
		}
		if v := row[f.Name]; v.Valid {
			out[f.Name] = v.Text
		}
	}
	return out
}

// attributeNames строит обратный указатель: идентификатор атрибута → имя.
//
// Читается из таблицы, а не из указателя, собранного при открытии модели: опись
// обязана видеть и то, что появилось в файле после открытия. Иначе незнакомый
// атрибут попадёт в перечень потерь безымянным, и смысл чёрного списка потеряется.
func (m *Model) attributeNames() map[int64]string {
	attributes, err := m.File.Table("attributes")
	if err != nil {
		return nil
	}
	out := make(map[int64]string, len(attributes.Rows))
	for _, row := range attributes.Rows {
		out[parseID(attributes.Str(row, "ATTRIBUTE_ID"))] = attributes.Str(row, "ATTRIBUTE_NAME")
	}
	return out
}

// Kind определяет вид элемента.
//
// Признак QUALIFIER_SYSTEM для этого НЕ ГОДИТСЯ, хотя и напрашивается: F_SECTORS
// и F_STREAMS помечены системными, но несут данные автора — всю геометрию
// стрелок и имена потоков. Отбор по нему выбросил бы их из описи вместе с
// метамоделью, и потери подписей стрелок остались бы незамеченными.
//
// Поэтому список переносимых видов положительный и закрытый: открыто множество
// атрибутов, там и живёт незнакомое, а видов элементов мало и шестой добавляется
// осознанно.
func (m *Model) Kind(element int64) ElementKind {
	if element == m.Element {
		return KindModel
	}

	elements, err := m.File.Table("elements")
	if err != nil {
		return KindOther
	}
	row, ok := elements.First(Eq("ELEMENT_ID", formatID(element)))
	if !ok {
		return KindOther
	}
	qualifier := elements.Str(row, "QUALIFIER_ID")

	switch qualifier {
	case formatID(m.FunctionQualifier):
		return KindFunction
	case m.qualifiers["F_STREAMS"]:
		return KindStream
	case m.qualifiers["F_SECTORS"]:
		return KindSector
	}
	for _, c := range m.Classifiers() {
		if qualifier == formatID(c.ID) {
			return KindClassifierRow
		}
	}
	return KindOther
}

// TransferableElements отдаёт все элементы, которые переносятся во входной
// язык, вместе с их видом.
func (m *Model) TransferableElements() map[int64]ElementKind {
	elements, err := m.File.Table("elements")
	if err != nil {
		return nil
	}

	out := make(map[int64]ElementKind)
	for _, row := range elements.Select(Eq("REMOVED_BRANCH_ID", AliveBranch)) {
		id := parseID(elements.Str(row, "ELEMENT_ID"))
		if kind := m.Kind(id); kind != KindOther {
			out[id] = kind
		}
	}
	// Элемент модели живёт в F_BASE_FUNCTIONS и в отбор выше попадает, но
	// подстрахуемся: без него потеряются метаданные модели.
	out[m.Element] = KindModel
	return out
}

// IsNonElementTable сообщает, что таблица описывает не элемент, а сам атрибут.
// Нужна проверке полноты описи: такую таблицу пропускают осознанно, а не по
// забывчивости.
func IsNonElementTable(name string) bool { return nonElementTables[name] }

// HasElementKey сообщает, что опись умеет разобрать эту таблицу.
func HasElementKey(t *Table) bool { return elementKeyColumn(t) != "" }

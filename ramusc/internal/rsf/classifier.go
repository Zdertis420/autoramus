package rsf

import (
	"sort"
	"strconv"
)

// formatID — обратная parseID: идентификаторы в таблицах лежат строками,
// и отбор идёт по строке.
func formatID(id int64) string { return strconv.FormatInt(id, 10) }

// Классификаторы Ramus хранит той же метамоделью EAV, что и всё остальное
// (RSF-FORMAT.md §4): справочник — это квалификатор, его колонки — атрибуты,
// привязанные к квалификатору через qualifiers_attributes, его строки —
// элементы с этим QUALIFIER_ID, а значения ячеек лежат в attribute_<тип>
// по паре (ELEMENT_ID, ATTRIBUTE_ID).

// Classifier — пользовательский справочник модели.
type Classifier struct {
	ID   int64
	Name string
	// NameAttribute — колонка, играющая роль имени строки.
	NameAttribute int64
	Columns       []ClassifierColumn
	Rows          []ClassifierRow
}

// ClassifierColumn — колонка справочника.
type ClassifierColumn struct {
	ID   int64
	Name string
	// Type — полное имя типа, например «Core.Text»: без имени модуля
	// названия повторяются между модулями.
	Type     string
	Position int64
	// Target — квалификатор, на который ссылается колонка вида ref или list.
	// Ноль — колонка ни на что не ссылается.
	Target int64
}

// ClassifierRow — строка справочника. Значения лежат по идентификатору
// колонки: отсутствие ключа означает, что значения нет вовсе, и это не то же
// самое, что пустая строка (§6 формата).
type ClassifierRow struct {
	ID     int64
	Values map[int64]ClassifierValue
}

// ClassifierValue — содержимое ячейки. Ссылочные колонки хранят не текст, а
// указание на другие строки, и сводить их к тексту нельзя: связь справочников
// при этом теряется.
type ClassifierValue struct {
	// Text — значение скалярной колонки как оно лежит в файле.
	Text string
	// Refs — строки, на которые ссылается колонка ref или list.
	Refs []int64
}

// valueTables — где лежат значения ячеек для каждого типа колонки. Тип, для
// которого таблицы нет, языком не выражается: его ловит Unsupported.
var valueTables = map[string]string{
	"Core.Text":         "attribute_texts",
	"Core.Long":         "attribute_longs",
	"Core.Double":       "attribute_doubles",
	"Core.Date":         "attribute_dates",
	"Core.Boolean":      "attribute_booleans",
	"Core.ElementList":  elementListTable,
	"Core.OtherElement": otherElementTable,
}

// Две таблицы ссылок устроены не как остальные: значения в них нет, есть
// указание на другой элемент. Читать их тем же кодом нельзя, поэтому они
// названы отдельно.
const (
	// elementListTable — список ссылок: пара ELEMENT1_ID → ELEMENT2_ID.
	elementListTable = "attribute_element_lists"
	// otherElementTable — одиночная ссылка: колонка OTHER_ELEMENT.
	otherElementTable = "attribute_other_elements"
	// Куда ссылается колонка, описано не значением, а свойствами атрибута.
	otherElementPropsTable = "attribute_other_element_properties"
	elementListPropsTable  = "attribute_element_list_properties"
)

// Classifiers отдаёт справочники автора: квалификаторы, не помеченные
// системными и не являющиеся квалификатором работ этой модели.
//
// Отбор идёт по признаку QUALIFIER_SYSTEM, а не по имени: имена
// пользовательские и ни о чём не говорят. Квалификатор работ исключается
// отдельно — системным он не помечен, но справочником не является.
func (m *Model) Classifiers() []Classifier {
	qualifiers, err := m.File.Table("qualifiers")
	if err != nil {
		return nil
	}

	var out []Classifier
	for _, row := range qualifiers.Select(Eq("QUALIFIER_SYSTEM", "FALSE")) {
		id := parseID(qualifiers.Str(row, "QUALIFIER_ID"))
		if id == 0 || id == m.FunctionQualifier {
			continue
		}
		columns := m.classifierColumns(id)
		out = append(out, Classifier{
			ID:            id,
			Name:          qualifiers.Str(row, "QUALIFIER_NAME"),
			NameAttribute: parseID(qualifiers.Str(row, "ATTRIBUTE_FOR_NAME")),
			Columns:       columns,
			Rows:          m.classifierRows(id, columns),
		})
	}

	// Порядок задаётся явно: обход таблицы сам по себе стабилен, но опираться
	// на это нельзя, а вывод обязан быть детерминированным.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// classifierColumns собирает колонки справочника, отбрасывая системные:
// HierarchicalAttribute и подобные Ramus выдаёт сам, автор их не создавал,
// и в документе на входном языке им не место.
func (m *Model) classifierColumns(qualifier int64) []ClassifierColumn {
	links, err := m.File.Table("qualifiers_attributes")
	if err != nil {
		return nil
	}
	attributes, err := m.File.Table("attributes")
	if err != nil {
		return nil
	}

	var out []ClassifierColumn
	for _, link := range links.Select(Eq("QUALIFIER_ID", formatID(qualifier))) {
		id := links.Str(link, "ATTRIBUTE_ID")
		attr, ok := attributes.First(Eq("ATTRIBUTE_ID", id))
		if !ok || attributes.Str(attr, "ATTRIBUTE_SYSTEM") == "TRUE" {
			continue
		}
		column := ClassifierColumn{
			ID:       parseID(id),
			Name:     attributes.Str(attr, "ATTRIBUTE_NAME"),
			Type:     attributes.Str(attr, "ATTRIBUTE_TYPE_PLUGIN_NAME") + "." + attributes.Str(attr, "ATTRIBUTE_TYPE_NAME"),
			Position: parseID(links.Str(link, "ATTRIBUTE_POSITION")),
		}
		column.Target = m.columnTarget(column)
		out = append(out, column)
	}

	// Вторичный ключ обязателен: у справочника, созданного и не заполненного,
	// позиции всех колонок нулевые, и без него порядок зависел бы от обхода.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// classifierRows собирает строки справочника вместе со значениями ячеек.
func (m *Model) classifierRows(qualifier int64, columns []ClassifierColumn) []ClassifierRow {
	elements, err := m.File.Table("elements")
	if err != nil {
		return nil
	}

	var out []ClassifierRow
	for _, el := range elements.Select(
		Eq("QUALIFIER_ID", formatID(qualifier)),
		Eq("REMOVED_BRANCH_ID", AliveBranch),
	) {
		id := parseID(elements.Str(el, "ELEMENT_ID"))
		out = append(out, ClassifierRow{ID: id, Values: m.cellValues(id, columns)})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// cellValues читает значения одной строки по всем колонкам справочника.
func (m *Model) cellValues(element int64, columns []ClassifierColumn) map[int64]ClassifierValue {
	values := make(map[int64]ClassifierValue, len(columns))
	for _, col := range columns {
		name, ok := valueTables[col.Type]
		if !ok {
			continue // тип без соответствия — забота Unsupported
		}
		table, err := m.File.Table(name)
		if err != nil {
			continue
		}

		switch name {
		case elementListTable:
			// Список ссылок: строк может быть несколько, и порядок задаётся
			// явно, иначе вывод перестанет быть детерминированным.
			var refs []int64
			for _, row := range table.Select(
				Eq("ELEMENT1_ID", formatID(element)),
				Eq("ATTRIBUTE_ID", formatID(col.ID)),
				Eq("REMOVED_BRANCH_ID", AliveBranch),
			) {
				refs = append(refs, parseID(table.Str(row, "ELEMENT2_ID")))
			}
			if len(refs) > 0 {
				sort.Slice(refs, func(i, j int) bool { return refs[i] < refs[j] })
				values[col.ID] = ClassifierValue{Refs: refs}
			}

		case otherElementTable:
			row, ok := table.First(
				Eq("ELEMENT_ID", formatID(element)),
				Eq("ATTRIBUTE_ID", formatID(col.ID)),
			)
			if ok {
				values[col.ID] = ClassifierValue{Refs: []int64{parseID(table.Str(row, "OTHER_ELEMENT"))}}
			}

		default:
			row, ok := table.First(
				Eq("ELEMENT_ID", formatID(element)),
				Eq("ATTRIBUTE_ID", formatID(col.ID)),
			)
			if !ok {
				continue // записи нет — значения нет; это не пустая строка
			}
			values[col.ID] = ClassifierValue{Text: table.Str(row, "VALUE")}
		}
	}
	return values
}

// columnTarget находит квалификатор, на который ссылается колонка.
//
// Устройство у двух видов ссылок разное, и оба раза «свой» квалификатор
// записан явно, а цель — нет:
//
//   - ref (Core.OtherElement): в attribute_other_element_properties колонка
//     QUALIFIER — это владелец, а цель выводится из QUALIFIER_ATTRIBUTE:
//     это атрибут, играющий роль имени в целевом квалификаторе. Проверено на
//     F_SECTOR_STREAM: владелец F_SECTORS, QUALIFIER_ATTRIBUTE = F_STREAM_NAME,
//     то есть цель — F_STREAMS;
//   - list (Core.ElementList): в attribute_element_list_properties QUALIFIER1 —
//     владелец, QUALIFIER2 — цель. Проверено на QualifierAttributes: владелец
//     QualifiersQualifier, цель AttributesQualifier.
func (m *Model) columnTarget(c ClassifierColumn) int64 {
	switch c.Type {
	case "Core.OtherElement":
		props, err := m.File.Table(otherElementPropsTable)
		if err != nil {
			return 0
		}
		row, ok := props.First(Eq("ATTRIBUTE", formatID(c.ID)))
		if !ok {
			return 0
		}
		return m.qualifierOfAttribute(parseID(props.Str(row, "QUALIFIER_ATTRIBUTE")), parseID(props.Str(row, "QUALIFIER")))

	case "Core.ElementList":
		props, err := m.File.Table(elementListPropsTable)
		if err != nil {
			return 0
		}
		row, ok := props.First(Eq("ATTRIBUTE_ID", formatID(c.ID)))
		if !ok {
			return 0
		}
		return parseID(props.Str(row, "QUALIFIER2"))
	}
	return 0
}

// qualifierOfAttribute ищет квалификатор, которому принадлежит атрибут,
// пропуская владельца самой ссылки: атрибут-имя может быть заведён у обоих.
func (m *Model) qualifierOfAttribute(attribute, owner int64) int64 {
	if attribute == 0 {
		return 0
	}
	links, err := m.File.Table("qualifiers_attributes")
	if err != nil {
		return 0
	}
	for _, row := range links.Select(Eq("ATTRIBUTE_ID", formatID(attribute))) {
		if id := parseID(links.Str(row, "QUALIFIER_ID")); id != owner {
			return id
		}
	}
	return 0
}

package generate

import (
	"fmt"
	"math"
	"strconv"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Оформление работы: значения взяты из настоящих моделей Ramus, а не
// придуманы. В языке их нет, но и опустить их нельзя — не проверено, как Ramus
// отрисует работу без шрифта и цвета, а цикл такой проверки идёт через GUI и
// стоит дорого. Значения постоянны, поэтому детерминированности не мешают.
const (
	fontName       = "Dialog"
	fontSize       = "10"
	fontStyle      = "0"
	colorBack      = "-1"
	colorFore      = "-16777216"
	statusType     = "0"
	visualDataMask = "8280808080808080"
)

// writeFunctions записывает работы: элемент, имя, место в дереве, тип,
// прямоугольник и оформление.
//
// Порядок обхода — порядок документа. Он же задаёт цепочку соседей и номера
// элементов: обход map дал бы разные файлы на одном и том же входе.
func writeFunctions(m *rsf.Model, source *ir.Model, boxes map[string]*ir.FunctionLayout) error {
	next, err := m.File.NextID("elements", "ELEMENT_ID")
	if err != nil {
		return err
	}

	// Номера раздаются заранее и все сразу: родитель может быть объявлен
	// после ребёнка, и ссылаться на ещё не созданный элемент иначе не выйдет.
	ids := make(map[string]int64, len(source.Functions))
	for _, f := range source.Functions {
		ids[f.Name.Name] = next
		next++
	}

	// Предыдущий сосед — последняя работа с тем же родителем. Ключ — имя
	// родителя; у корневой оно пустое, и она в своей группе одна.
	last := make(map[string]int64)

	for _, f := range source.Functions {
		id := ids[f.Name.Name]

		parent := m.Element
		if f.Of.Name != "" {
			owner, ok := ids[f.Of.Name]
			if !ok {
				return fmt.Errorf("работа «%s»: родителя «%s» нет в документе", f.Name.Name, f.Of.Name)
			}
			parent = owner
		}

		previous := int64(-1)
		if prev, ok := last[f.Of.Name]; ok {
			previous = prev
		}
		last[f.Of.Name] = id

		// Тип выводится из положения в дереве, а не из поля языка: Type в IR
		// описывает вид элемента DFD, снятый с объёма решением фичи 001.
		// У корневой работы Ramus пишет 3, у остальных 1 — проверено на всех
		// трёх настоящих моделях.
		kind := rsf.TypeProcess
		if f.Of.Name == "" {
			kind = rsf.TypeOperation
		}

		if err := writeFunction(m, id, parent, previous, kind, f, boxes[f.Name.Name]); err != nil {
			return err
		}
	}
	return nil
}

// writeFunction кладёт одну работу во все её таблицы.
func writeFunction(m *rsf.Model, id, parent, previous int64, kind int, f *ir.Function, box *ir.FunctionLayout) error {
	element := fmt.Sprint(id)

	name, ok := m.Attribute("Name")
	if !ok {
		name = m.NameAttribute
	}
	hierarchical, ok := m.Attribute("HierarchicalAttribute")
	if !ok {
		return fmt.Errorf("в заготовке нет атрибута HierarchicalAttribute")
	}

	rows := []struct {
		table  string
		values map[string]string
	}{
		{"elements", map[string]string{
			// Имя работы живёт значением атрибута, а не в ELEMENT_NAME:
			// в настоящих файлах эта колонка у работ пуста. Два места для
			// одного значения однажды разошлись бы.
			"ELEMENT_ID":        element,
			"ELEMENT_NAME":      "",
			"QUALIFIER_ID":      fmt.Sprint(m.FunctionQualifier),
			"CREATED_BRANCH_ID": "0",
			"REMOVED_BRANCH_ID": rsf.AliveBranch,
		}},
		{"attribute_hierarchicals", map[string]string{
			"ATTRIBUTE_ID":        fmt.Sprint(hierarchical),
			"ELEMENT_ID":          element,
			"ICON_ID":             "-1",
			"PARENT_ELEMENT_ID":   fmt.Sprint(parent),
			"PREVIOUS_ELEMENT_ID": fmt.Sprint(previous),
			"VALUE_BRANCH_ID":     "0",
		}},
		{"attribute_texts", map[string]string{
			"ATTRIBUTE_ID":    fmt.Sprint(name),
			"ELEMENT_ID":      element,
			"VALUE":           f.Name.Name,
			"VALUE_BRANCH_ID": "0",
		}},
		{"attribute_function_types", map[string]string{
			"ELEMENT_ID": element, "TYPE": fmt.Sprint(kind), "VALUE_BRANCH_ID": "0",
		}},
		{"attribute_rectangles", map[string]string{
			"ELEMENT_ID": element,
			"X":          number(box.X.Val), "Y": number(box.Y.Val),
			"WIDTH": number(box.Width.Val), "HEIGHT": number(box.Height.Val),
			"VALUE_BRANCH_ID": "0",
		}},
		{"attribute_fonts", map[string]string{
			"ELEMENT_ID": element, "NAME": fontName, "SIZE": fontSize, "STYLE": fontStyle,
			"VALUE_BRANCH_ID": "0",
		}},
		{"attribute_statuses", map[string]string{
			"ELEMENT_ID": element, "TYPE": statusType, "OTHER_NAME": "", "VALUE_BRANCH_ID": "0",
		}},
		{"attribute_visual_datas", map[string]string{
			"ELEMENT_ID": element, "DATA": visualDataMask, "VALUE_BRANCH_ID": "0",
		}},
	}

	// У атрибутов, чьё имя известно, идентификатор проставляется по имени:
	// номера свои в каждом файле, и зашивать их числом нельзя.
	attributes := map[string]string{
		"attribute_function_types": "F_TYPE",
		"attribute_rectangles":     "F_BOUNDS",
		"attribute_fonts":          "F_FONT",
		"attribute_statuses":       "F_STATUS",
		"attribute_visual_datas":   "F_VISUAL_DATA",
	}

	for _, r := range rows {
		if attribute, ok := attributes[r.table]; ok {
			id, found := m.Attribute(attribute)
			if !found {
				return fmt.Errorf("в заготовке нет атрибута %s", attribute)
			}
			r.values["ATTRIBUTE_ID"] = fmt.Sprint(id)
		}
		table, err := m.File.Table(r.table)
		if err != nil {
			return err
		}
		if _, err := table.Add(r.values); err != nil {
			return fmt.Errorf("%s: %w", r.table, err)
		}
	}

	// Цвета лежат в одной таблице двумя строками — фон и текст.
	colors, err := m.File.Table("attribute_colors")
	if err != nil {
		return err
	}
	for _, c := range []struct{ attribute, value string }{
		{"F_BACKGROUND", colorBack},
		{"F_FOREGROUND", colorFore},
	} {
		attribute, found := m.Attribute(c.attribute)
		if !found {
			return fmt.Errorf("в заготовке нет атрибута %s", c.attribute)
		}
		if _, err := colors.Add(map[string]string{
			"ATTRIBUTE_ID": fmt.Sprint(attribute), "ELEMENT_ID": element,
			"COLOR": c.value, "VALUE_BRANCH_ID": "0",
		}); err != nil {
			return err
		}
	}

	// Даты (F_CREATE_DATE, F_REV_DATE) не пишутся вовсе: взять их неоткуда,
	// а текущее время сделало бы две сборки одной модели разными файлами.
	return nil
}

// number печатает координату так, как её пишет Ramus: целое значение — с
// нулём после точки. Разбору всё равно, а глазами файлы сравнивают часто.
func number(v float64) string {
	if v == math.Trunc(v) && !math.IsInf(v, 0) {
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

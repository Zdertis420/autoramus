package decompile

import (
	"encoding/json"
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// columnTypes переводит тип колонки Ramus в тип входного языка (Р8).
// Тип, которого здесь нет, до сюда не доходит: его ловит rsf.Unsupported
// и декомпиляция отказывает целиком.
var columnTypes = map[string]string{
	"Core.Text":         "text",
	"Core.Long":         "long",
	"Core.Double":       "number",
	"Core.Date":         "date",
	"Core.Boolean":      "bool",
	"Core.OtherElement": "ref",
	"Core.ElementList":  "list",
}

// classifiers переводит справочники файла в IR.
func classifiers(m *rsf.Model, carried map[int64]bool) []*ir.Classifier {
	source := m.Classifiers()
	if len(source) == 0 {
		return nil
	}

	// Ссылки в языке адресуют строку по имени, а в файле — по идентификатору,
	// поэтому имена собираются заранее: колонка может ссылаться на справочник,
	// объявленный ниже по списку.
	names := make(map[int64]string)
	for _, c := range source {
		for _, row := range c.Rows {
			if v, ok := row.Values[c.NameAttribute]; ok {
				names[row.ID] = v.Text
			}
		}
	}

	// Цель ссылочной колонки в языке адресуется именем справочника (Р4),
	// а в файле — идентификатором квалификатора.
	byQualifier := make(map[int64]string, len(source))
	for _, c := range source {
		byQualifier[c.ID] = c.Name
	}

	out := make([]*ir.Classifier, 0, len(source))
	for i, c := range source {
		path := fmt.Sprintf("/classifiers/%d", i)
		dst := &ir.Classifier{Name: ref(c.Name, path+"/name"), Path: path}

		for j, col := range c.Columns {
			colPath := fmt.Sprintf("%s/columns/%d", path, j)
			column := &ir.Column{
				Name: ref(col.Name, colPath+"/name"),
				Type: ref(columnTypes[col.Type], colPath+"/type"),
				Path: colPath,
			}
			// Поле цели печатается только у ссылочных колонок: у обычной его
			// быть не должно, валидатор считает это ошибкой.
			if name, ok := byQualifier[col.Target]; ok {
				column.Of = ref(name, colPath+"/of")
			}
			dst.Columns = append(dst.Columns, column)
		}

		for j, row := range c.Rows {
			rowPath := fmt.Sprintf("%s/rows/%d", path, j)
			dst.Rows = append(dst.Rows, &ir.Row{
				Cells: cells(c, row, names, rowPath),
				Raw:   rawOf(m, row.ID),
				Path:  rowPath,
			})
			carried[row.ID] = true
		}
		out = append(out, dst)
	}
	return out
}

// cells переводит значения одной строки. Порядок повторяет порядок колонок:
// ячейка называет свою колонку, но детерминированность вывода требует
// устойчивого порядка и здесь.
func cells(c rsf.Classifier, row rsf.ClassifierRow, names map[int64]string, rowPath string) []*ir.Cell {
	var out []*ir.Cell
	for _, col := range c.Columns {
		value, ok := row.Values[col.ID]
		if !ok {
			continue // значения нет вовсе — ячейку не пишем
		}
		cellPath := fmt.Sprintf("%s/cells/%d", rowPath, len(out))
		out = append(out, &ir.Cell{
			Column: ref(col.Name, cellPath+"/column"),
			Value:  cellValue(columnTypes[col.Type], value, names),
			Path:   cellPath,
		})
	}
	return out
}

// cellValue приводит значение к тому виду, которого ждёт входной язык:
// число числом, признак признаком, ссылка именем строки. Валидатор сверяет
// вид значения с типом колонки, поэтому подсунуть всё строками нельзя.
func cellValue(kind string, v rsf.ClassifierValue, names map[int64]string) any {
	switch kind {
	case "long", "number":
		if v.Text == "" {
			return json.Number("0")
		}
		return json.Number(v.Text)
	case "bool":
		return v.Text == "TRUE"
	case "ref":
		if len(v.Refs) == 0 {
			return ""
		}
		return names[v.Refs[0]]
	case "list":
		list := make([]any, 0, len(v.Refs))
		for _, id := range v.Refs {
			list = append(list, names[id])
		}
		return list
	default:
		return v.Text
	}
}

package validate

import (
	"encoding/json"
	"time"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// checkClassifiers проверяет таблицы классификаторов (Р8). Тип значения со
// своей колонкой схемой не связать — это делается здесь, и сообщение выходит
// куда понятнее, чем «не подходит ни под один вариант anyOf».
func (c *checker) checkClassifiers() {
	names := make([]string, 0, len(c.m.Classifiers))
	for _, cl := range c.m.Classifiers {
		if cl.Name.Name != "" {
			names = append(names, cl.Name.Name)
		}
	}
	c.classifierNames = names

	seen := make(map[string]ir.Ref, len(c.m.Classifiers))
	for _, cl := range c.m.Classifiers {
		if !c.name(cl.Name, "имя классификатора") {
			continue
		}
		if first, dup := seen[cl.Name.Name]; dup {
			c.add(diag.New(diag.CodeDuplicateClassifier, cl.Name.Pos, cl.Name.Path,
				"классификатор «%s» уже объявлен в строке %d", cl.Name.Name, first.Pos.Line))
			continue
		}
		seen[cl.Name.Name] = cl.Name

		c.checkColumns(cl)
		for _, row := range cl.Rows {
			c.checkRow(cl, row)
		}
	}
}

func (c *checker) checkColumns(cl *ir.Classifier) {
	seen := make(map[string]ir.Ref, len(cl.Columns))
	for _, col := range cl.Columns {
		if !c.name(col.Name, "имя колонки") {
			continue
		}
		if first, dup := seen[col.Name.Name]; dup {
			c.add(diag.New(diag.CodeDuplicateColumn, col.Name.Pos, col.Name.Path,
				"колонка «%s» уже объявлена в строке %d", col.Name.Name, first.Pos.Line))
			continue
		}
		seen[col.Name.Name] = col.Name

		switch col.Type.Name {
		case ir.ColumnRef, ir.ColumnList:
			if !col.Of.Set() {
				continue // ссылка на работу — цель по умолчанию
			}
			if !c.name(col.Of, "цель колонки") {
				continue
			}
			if c.m.Classifier(col.Of.Name) == nil {
				c.add(diag.New(diag.CodeUnknownClassifier, col.Of.Pos, col.Of.Path,
					"классификатор «%s» не найден%s",
					col.Of.Name, hint(col.Of.Name, c.classifierNames)))
			}
		default:
			if col.Of.Set() {
				c.add(diag.New(diag.CodeUnexpectedColumnTarget, col.Of.Pos, col.Of.Path,
					"поле of имеет смысл только у колонок ref и list, а у «%s» тип «%s»",
					col.Name.Name, col.Type.Name))
			}
		}
	}
}

func (c *checker) checkRow(cl *ir.Classifier, row *ir.Row) {
	columns := make([]string, 0, len(cl.Columns))
	for _, col := range cl.Columns {
		if col.Name.Name != "" {
			columns = append(columns, col.Name.Name)
		}
	}

	seen := make(map[string]ir.Ref, len(row.Cells))
	for _, cell := range row.Cells {
		if !c.name(cell.Column, "имя колонки") {
			continue
		}
		col := cl.Column(cell.Column.Name)
		if col == nil {
			c.add(diag.New(diag.CodeUnknownColumn, cell.Column.Pos, cell.Column.Path,
				"у классификатора «%s» нет колонки «%s»%s",
				cl.Name.Name, cell.Column.Name, hint(cell.Column.Name, columns)))
			continue
		}
		if first, dup := seen[cell.Column.Name]; dup {
			c.add(diag.New(diag.CodeDuplicateCell, cell.Column.Pos, cell.Column.Path,
				"колонка «%s» в этой строке уже заполнена в строке %d",
				cell.Column.Name, first.Pos.Line))
			continue
		}
		seen[cell.Column.Name] = cell.Column

		c.checkCell(col, cell)
	}
}

// checkCell сверяет значение с типом колонки. Словарь типов повторяет Ramus,
// поэтому и проверки буквальные: long — целое, date — календарная дата.
func (c *checker) checkCell(col *ir.Column, cell *ir.Cell) {
	switch col.Type.Name {
	case ir.ColumnText:
		c.expectString(col, cell)

	case ir.ColumnNumber:
		if _, ok := cell.Value.(json.Number); !ok {
			c.badCell(col, cell, "число")
		}

	case ir.ColumnLong:
		if !ir.IsInt(cell.Value) {
			c.badCell(col, cell, "целое число")
		}

	case ir.ColumnBool:
		if _, ok := cell.Value.(bool); !ok {
			c.badCell(col, cell, "логическое значение")
		}

	case ir.ColumnDate:
		s, ok := cell.Value.(string)
		if !ok {
			c.badCell(col, cell, "дата строкой вида ГГГГ-ММ-ДД")
			return
		}
		if _, err := time.Parse(time.DateOnly, s); err != nil {
			c.add(diag.New(diag.CodeCellType, cell.Pos, cell.Path,
				"«%s» не похоже на дату: колонка «%s» ждёт ГГГГ-ММ-ДД", s, col.Name.Name))
		}

	case ir.ColumnRef:
		if s, ok := c.expectString(col, cell); ok {
			c.checkReference(col, cell, s)
		}

	case ir.ColumnList:
		items, ok := cell.Value.([]any)
		if !ok {
			c.badCell(col, cell, "список имён")
			return
		}
		for _, item := range items {
			s, ok := item.(string)
			if !ok {
				c.badCell(col, cell, "список имён")
				return
			}
			c.checkReference(col, cell, s)
		}
	}
}

func (c *checker) expectString(col *ir.Column, cell *ir.Cell) (string, bool) {
	s, ok := cell.Value.(string)
	if !ok {
		c.badCell(col, cell, "строка")
		return "", false
	}
	return s, true
}

func (c *checker) badCell(col *ir.Column, cell *ir.Cell, want string) {
	c.add(diag.New(diag.CodeCellType, cell.Pos, cell.Path,
		"колонка «%s» имеет тип «%s»: ожидается %s, получено %s",
		col.Name.Name, col.Type.Name, want, valueTypeName(cell.Value)))
}

// checkReference разрешает значение ссылочной колонки: без of ссылка ведёт на
// работу, с of — на строку другого классификатора (Р8).
func (c *checker) checkReference(col *ir.Column, cell *ir.Cell, raw string) {
	name := ir.Normalize(raw)
	if name == "" {
		c.add(diag.New(diag.CodeEmptyName, cell.Pos, cell.Path,
			"ссылка не может быть пустой: в ней одни пробелы"))
		return
	}

	if !col.Of.Set() {
		if _, ok := c.functions[name]; !ok {
			c.add(diag.New(diag.CodeUnknownFunction, cell.Pos, cell.Path,
				"работа «%s» не найдена%s", name, hint(name, c.funcNames)))
		}
		return
	}

	target := c.m.Classifier(col.Of.Name)
	if target == nil {
		return // о неизвестной цели уже сказано на самой колонке
	}
	key := keyColumn(target)
	if key == nil {
		return // строки такого классификатора звать не по чему
	}
	var keys []string
	for _, row := range target.Rows {
		for _, keyCell := range row.Cells {
			if keyCell.Column.Name != key.Name.Name {
				continue
			}
			if s, ok := keyCell.Value.(string); ok {
				keys = append(keys, ir.Normalize(s))
			}
		}
	}
	for _, k := range keys {
		if k == name {
			return
		}
	}
	c.add(diag.New(diag.CodeUnknownRow, cell.Pos, cell.Path,
		"в классификаторе «%s» нет строки «%s» (колонка «%s»)%s",
		target.Name.Name, name, key.Name.Name, hint(name, keys)))
}

// keyColumn — колонка, по которой зовут строки классификатора: первая
// текстовая. В самом Ramus роль имени играет атрибут из ATTRIBUTE_FOR_NAME,
// и это ровно такой же выбор.
func keyColumn(cl *ir.Classifier) *ir.Column {
	for _, col := range cl.Columns {
		if col.Type.Name == ir.ColumnText && col.Name.Name != "" {
			return col
		}
	}
	return nil
}

func valueTypeName(v any) string {
	switch v.(type) {
	case string:
		return "строка"
	case json.Number:
		return "число"
	case bool:
		return "логическое значение"
	case []any:
		return "список"
	case map[string]any:
		return "объект"
	case nil:
		return "null"
	default:
		return "неизвестно"
	}
}

package rsf_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Чтение значений ячеек проверяется на таблицах, собранных вручную поверх
// настоящего файла: ни одна из имеющихся моделей Ramus непустого справочника
// не содержит (см. research.md, Р-4). Строки добавляются в те же таблицы того
// же файла, поэтому проверяется настоящий код чтения, а не его подобие.
//
// Ограничение осознанное: подтверждено, что мы читаем так, как сами записали,
// но не что Ramus записывает именно так. Снимается моделью с заполненным
// справочником.

const catalog = 15 // «каталог 1» в testModel.rsf

// fillClassifier дописывает в файл колонки, строки и значения справочника.
func fillClassifier(t *testing.T, m *rsf.Model) {
	t.Helper()

	add := func(table string, values map[string]string) {
		tb, err := m.File.Table(table)
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if _, err := tb.Add(values); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
	}

	columns := []struct {
		id, name, plugin, typ, position string
	}{
		{"900", "Обозначение", "Core", "Text", "1"},
		{"901", "Количество", "Core", "Long", "2"},
		{"902", "Активна", "Core", "Boolean", "3"},
		{"903", "Родитель", "Core", "ElementList", "4"},
	}
	for _, c := range columns {
		add("attributes", map[string]string{
			"ATTRIBUTE_ID":               c.id,
			"ATTRIBUTE_NAME":             c.name,
			"ATTRIBUTE_TYPE_PLUGIN_NAME": c.plugin,
			"ATTRIBUTE_TYPE_NAME":        c.typ,
			"ATTRIBUTE_TYPE_COMPARABLE":  "TRUE",
			"ATTRIBUTE_SYSTEM":           "FALSE",
			"CREATED_BRANCH_ID":          "0",
			"REMOVED_BRANCH_ID":          rsf.AliveBranch,
		})
		add("qualifiers_attributes", map[string]string{
			"QUALIFIER_ID":       "15",
			"ATTRIBUTE_ID":       c.id,
			"ATTRIBUTE_SYSTEM":   "FALSE",
			"ATTRIBUTE_POSITION": c.position,
			"CREATED_BRANCH_ID":  "0",
		})
	}

	for _, id := range []string{"800", "801"} {
		add("elements", map[string]string{
			"ELEMENT_ID":        id,
			"ELEMENT_NAME":      "",
			"QUALIFIER_ID":      "15",
			"CREATED_BRANCH_ID": "0",
			"REMOVED_BRANCH_ID": rsf.AliveBranch,
		})
	}

	// Строка 800 заполнена целиком, у строки 801 значения нет вовсе —
	// это не пустая строка, и различать их обязательно.
	add("attribute_texts", map[string]string{
		"ATTRIBUTE_ID": "900", "ELEMENT_ID": "800", "VALUE": "шт", "VALUE_BRANCH_ID": "0",
	})
	add("attribute_longs", map[string]string{
		"ATTRIBUTE_ID": "901", "ELEMENT_ID": "800", "VALUE": "42", "VALUE_BRANCH_ID": "0",
	})
	add("attribute_booleans", map[string]string{
		"ATTRIBUTE_ID": "902", "ELEMENT_ID": "800", "VALUE": "TRUE", "VALUE_BRANCH_ID": "0",
	})
	add("attribute_element_lists", map[string]string{
		"ATTRIBUTE_ID": "903", "CONNECTION_TYPE": "0",
		"ELEMENT1_ID": "800", "ELEMENT2_ID": "801",
		"VALUE_BRANCH_ID": "0", "REMOVED_BRANCH_ID": rsf.AliveBranch,
	})
	// Пустая строка у 801 по колонке 900: именно пустая, а не отсутствующая.
	add("attribute_texts", map[string]string{
		"ATTRIBUTE_ID": "900", "ELEMENT_ID": "801", "VALUE": "", "VALUE_BRANCH_ID": "0",
	})
}

func filledCatalog(t *testing.T) rsf.Classifier {
	t.Helper()
	m := openModel(t, fixtureByName(t, "testModel").Path)
	fillClassifier(t, m)

	for _, c := range m.Classifiers() {
		if c.ID == catalog {
			return c
		}
	}
	t.Fatal("справочник 15 не найден")
	return rsf.Classifier{}
}

// TestClassifierColumnsRead — колонки читаются с типами и в порядке позиции.
func TestClassifierColumnsRead(t *testing.T) {
	c := filledCatalog(t)

	want := []struct{ name, typ string }{
		{"Name", "Core.Text"}, // позиция 0, выданная Ramus по умолчанию
		{"Обозначение", "Core.Text"},
		{"Количество", "Core.Long"},
		{"Активна", "Core.Boolean"},
		{"Родитель", "Core.ElementList"},
	}
	if len(c.Columns) != len(want) {
		t.Fatalf("колонок %d, ожидалось %d: %+v", len(c.Columns), len(want), c.Columns)
	}
	for i, w := range want {
		if c.Columns[i].Name != w.name || c.Columns[i].Type != w.typ {
			t.Errorf("колонка %d: %q типа %q, ожидалось %q типа %q",
				i, c.Columns[i].Name, c.Columns[i].Type, w.name, w.typ)
		}
	}
}

// TestClassifierValuesRead — значения читаются по типу колонки из своей таблицы.
func TestClassifierValuesRead(t *testing.T) {
	c := filledCatalog(t)

	if len(c.Rows) != 2 {
		t.Fatalf("строк %d, ожидалось 2", len(c.Rows))
	}
	row := c.Rows[0]
	if row.ID != 800 {
		t.Fatalf("первая строка %d, ожидалась 800", row.ID)
	}
	for _, want := range []struct {
		column int64
		value  string
	}{
		{900, "шт"},
		{901, "42"},
		{902, "TRUE"},
	} {
		if got := row.Values[want.column].Text; got != want.value {
			t.Errorf("колонка %d: %q, ожидалось %q", want.column, got, want.value)
		}
	}
}

// TestElementListKeepsReference — ссылка на строку остаётся ссылкой.
// Подставить вместо неё значение означало бы потерять связь справочников.
func TestElementListKeepsReference(t *testing.T) {
	c := filledCatalog(t)

	refs := c.Rows[0].Values[903].Refs
	if len(refs) != 1 || refs[0] != 801 {
		t.Errorf("ссылки %v, ожидалась одна на строку 801", refs)
	}
}

// TestMissingValueIsNotEmptyString — отсутствие записи и пустая строка
// различаются: §6 формата предупреждает об этом отдельно.
func TestMissingValueIsNotEmptyString(t *testing.T) {
	c := filledCatalog(t)

	second := c.Rows[1]
	if v, ok := second.Values[900]; !ok || v.Text != "" {
		t.Errorf("колонка 900 строки 801: %q (есть=%v), ожидалась пустая строка", v.Text, ok)
	}
	if _, ok := second.Values[901]; ok {
		t.Error("у строки 801 значения по колонке 901 нет, а оно нашлось")
	}
}

// TestReferenceColumnTarget — ссылочная колонка помнит, на какой справочник
// указывает. Без этого после переноса непонятно, строки чего допустимы в
// колонке.
//
// Проверяется на собранных вручную таблицах: ссылочных колонок нет ни в одной
// из трёх настоящих моделей.
func TestReferenceColumnTarget(t *testing.T) {
	m := openModel(t, fixtureByName(t, "testModel").Path)

	add := func(table string, values map[string]string) {
		tb, err := m.File.Table(table)
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if _, err := tb.Add(values); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
	}

	// Колонка-ссылка и колонка-список у справочника 15, обе указывают на 16.
	for _, c := range []struct{ id, name, typ string }{
		{"910", "Ссылка", "OtherElement"},
		{"911", "Список", "ElementList"},
	} {
		add("attributes", map[string]string{
			"ATTRIBUTE_ID": c.id, "ATTRIBUTE_NAME": c.name,
			"ATTRIBUTE_TYPE_PLUGIN_NAME": "Core", "ATTRIBUTE_TYPE_NAME": c.typ,
			"ATTRIBUTE_TYPE_COMPARABLE": "TRUE", "ATTRIBUTE_SYSTEM": "FALSE",
			"CREATED_BRANCH_ID": "0", "REMOVED_BRANCH_ID": rsf.AliveBranch,
		})
		add("qualifiers_attributes", map[string]string{
			"QUALIFIER_ID": "15", "ATTRIBUTE_ID": c.id,
			"ATTRIBUTE_SYSTEM": "FALSE", "ATTRIBUTE_POSITION": "9",
			"CREATED_BRANCH_ID": "0",
		})
	}
	// Имя строки в целевом справочнике 16 — колонка Name (атрибут 55).
	add("attribute_other_element_properties", map[string]string{
		"ATTRIBUTE": "910", "QUALIFIER": "15",
		"QUALIFIER_ATTRIBUTE": "55", "VALUE_BRANCH_ID": "0",
	})
	add("attribute_element_list_properties", map[string]string{
		"ATTRIBUTE_ID": "911", "QUALIFIER1": "15", "QUALIFIER2": "16",
		"VALUE_BRANCH_ID": "0",
	})

	var catalog1 rsf.Classifier
	for _, c := range m.Classifiers() {
		if c.ID == 15 {
			catalog1 = c
		}
	}

	targets := make(map[string]int64)
	for _, col := range catalog1.Columns {
		targets[col.Name] = col.Target
	}
	if targets["Список"] != 16 {
		t.Errorf("колонка-список указывает на %d, ожидался справочник 16", targets["Список"])
	}
	if targets["Ссылка"] == 0 || targets["Ссылка"] == 15 {
		t.Errorf("колонка-ссылка указывает на %d: цель не найдена либо принята за владельца", targets["Ссылка"])
	}
	if targets["Обозначение"] != 0 {
		t.Errorf("обычная колонка получила цель %d", targets["Обозначение"])
	}
}

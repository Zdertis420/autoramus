package rsf_test

import (
	"strconv"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

func losses(t *testing.T, model string) map[string]rsf.Loss {
	t.Helper()
	m := openModel(t, fixtureByName(t, model).Path)

	out := make(map[string]rsf.Loss)
	for _, l := range m.Losses() {
		out[l.Attribute] = l
	}
	return out
}

// TestLossesFoundByComparison — перечень собирается вычитанием описи из
// перенесённого, а не из зашитого списка видов. Проверяется тем, что в нём
// оказываются все одиннадцать атрибутов, которые сегодня теряются: ни один из
// них нигде в коде не перечислен как «непереносимый».
func TestLossesFoundByComparison(t *testing.T) {
	got := losses(t, "ФормированиеТП")

	for _, want := range []string{
		"F_BACKGROUND", "F_FOREGROUND", "F_FONT",
		"F_STATUS", "F_DECOMPOSITION_TYPE", "F_VISUAL_DATA",
		"F_CREATE_DATE", "F_REV_DATE", "F_SYSTEM_REV_DATE",
		"F_SECTOR_PROPERTIES", "F_SECTOR_ATTRIBUTE",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("потеря %s не найдена", want)
		}
	}
}

// TestServiceAttributesAreNotLosses — метамодель в перечень не идёт: перечень,
// на девять десятых состоящий из шума, никто читать не станет.
func TestServiceAttributesAreNotLosses(t *testing.T) {
	got := losses(t, "ФормированиеТП")

	for _, service := range []string{
		"AttributeId", "AttributeName", "AttributeTypeName",
		"QualifierId", "QualifierAttributes", "F_BASE_FUNCTION_QUALIFIER_ID",
	} {
		if l, ok := got[service]; ok {
			t.Errorf("служебный атрибут %s попал в перечень: %s", service, l)
		}
	}
}

// TestSectorAndStreamContentIsCounted — опровергнутая гипотеза, вынесенная в
// проверку. F_SECTORS и F_STREAMS помечены системными квалификаторами, но несут
// данные автора. Отбор по этому признаку выбросил бы 103 записи свойств подписи
// вместе с метамоделью, и потеря снова стала бы молчаливой.
func TestSectorAndStreamContentIsCounted(t *testing.T) {
	got := losses(t, "ФормированиеТП")

	for _, want := range []string{"F_SECTOR_PROPERTIES", "F_SECTOR_ATTRIBUTE"} {
		l, ok := got[want]
		if !ok {
			t.Fatalf("потеря %s не найдена: содержимое F_SECTORS отсечено как служебное", want)
		}
		if l.Kind != rsf.KindSector {
			t.Errorf("%s отнесена к виду «%s», ожидался сектор", want, l.Kind)
		}
	}
}

// TestPartialReadCountsAsLoss — из записи о модели читаются две колонки из
// девяти. Считать её перенесённой целиком значило бы тихо потерять семь полей —
// ровно тот дефект, ради которого затевалась работа.
func TestPartialReadCountsAsLoss(t *testing.T) {
	got := losses(t, "ФормированиеТП")

	l, ok := got["F_PROJECT_PREFERENCES"]
	if !ok {
		t.Fatal("запись о модели сочтена перенесённой целиком, хотя читаются две колонки из девяти")
	}

	lost := make(map[string]bool, len(l.Columns))
	for _, c := range l.Columns {
		lost[c] = true
	}
	for _, want := range []string{"DEFINITION", "MODEL_LETTER", "PROJECT_NAME", "USED_AT"} {
		if !lost[want] {
			t.Errorf("колонка %s не заявлена потерянной: %v", want, l.Columns)
		}
	}
	for _, read := range []string{"PROJECT_AUTOR", "DIAGRAM_SIZE"} {
		if lost[read] {
			t.Errorf("колонка %s читается, но заявлена потерянной", read)
		}
	}
}

// TestLossesAreFolded — одна и та же потеря на сотне элементов даёт одну запись
// с количеством. Перечень из ста одинаковых строк нечитаем.
func TestLossesAreFolded(t *testing.T) {
	all := openModel(t, fixtureByName(t, "ФормированиеТП").Path).Losses()

	seen := make(map[string]int)
	for _, l := range all {
		seen[l.Attribute+"|"+l.Kind.String()]++
	}
	for key, n := range seen {
		if n > 1 {
			t.Errorf("%s: записей %d, ожидалась одна", key, n)
		}
	}

	var props rsf.Loss
	for _, l := range all {
		if l.Attribute == "F_SECTOR_PROPERTIES" {
			props = l
		}
	}
	if props.Count < 100 {
		t.Errorf("F_SECTOR_PROPERTIES: количество %d, в файле их 103", props.Count)
	}
}

// TestUnknownAttributeIsFound — то, ради чего чёрный список и затевался.
// Атрибут, которого раньше не встречалось, попадает в перечень без единой
// правки кода. С белым списком это было невозможно по построению.
func TestUnknownAttributeIsFound(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ФормированиеТП").Path)

	before := len(m.Losses())

	// Атрибут, которого нет ни в одной модели и ни в одном списке кода.
	add := func(table string, values map[string]string) {
		tb, err := m.File.Table(table)
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if _, err := tb.Add(values); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
	}
	add("attributes", map[string]string{
		"ATTRIBUTE_ID": "990", "ATTRIBUTE_NAME": "F_НЕВИДАННЫЙ",
		"ATTRIBUTE_TYPE_PLUGIN_NAME": "Core", "ATTRIBUTE_TYPE_NAME": "Text",
		"ATTRIBUTE_TYPE_COMPARABLE": "TRUE", "ATTRIBUTE_SYSTEM": "FALSE",
		"CREATED_BRANCH_ID": "0", "REMOVED_BRANCH_ID": rsf.AliveBranch,
	})

	var function int64
	for id, kind := range m.TransferableElements() {
		if kind == rsf.KindFunction {
			function = id
			break
		}
	}
	add("attribute_texts", map[string]string{
		"ATTRIBUTE_ID": "990", "ELEMENT_ID": formatIDForTest(function),
		"VALUE": "нечто", "VALUE_BRANCH_ID": "0",
	})

	var found bool
	for _, l := range m.Losses() {
		if l.Attribute == "F_НЕВИДАННЫЙ" {
			found = true
		}
	}
	if !found {
		t.Errorf("незнакомый атрибут не попал в перечень потерь (было %d записей)", before)
	}
}

// formatIDForTest — идентификаторы в таблицах лежат строками.
func formatIDForTest(id int64) string { return strconv.FormatInt(id, 10) }

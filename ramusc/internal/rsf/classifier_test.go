package rsf_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// model открывает модель по пути, минуя перечень: часть проверок смотрит
// на конкретный файл, а не на весь набор.
func openModel(t *testing.T, path string) *rsf.Model {
	t.Helper()
	f, err := rsf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(f)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func fixtureByName(t *testing.T, name string) fixtures.Model {
	t.Helper()
	for _, m := range fixtures.All() {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("в перечне нет модели %s", name)
	return fixtures.Model{}
}

// TestClassifiersFound — главная проверка истории 1: справочники автора
// отделяются от служебных квалификаторов Ramus. В testModel.rsf квалификаторов
// девятнадцать, а справочников — пять; остальные тринадцать системные, и ещё
// один описывает саму диаграмму.
func TestClassifiersFound(t *testing.T) {
	m := openModel(t, fixtureByName(t, "testModel").Path)

	got := m.Classifiers()
	want := []string{"каталог 1", "каталог 2", "каталог 3", "каталог 4", "каталог 5"}
	if len(got) != len(want) {
		names := make([]string, len(got))
		for i, c := range got {
			names[i] = c.Name
		}
		t.Fatalf("справочников %d, ожидалось %d: %v", len(got), len(want), names)
	}
	for i, c := range got {
		if c.Name != want[i] {
			t.Errorf("справочник %d: %q, ожидалось %q", i, c.Name, want[i])
		}
	}
}

// TestClassifierShape — у справочников этого файла нет ни строк, ни колонок
// сверх той единственной, что Ramus выдаёт каждому новому квалификатору.
// Проверка фиксирует именно это: заявлять, что чтение строк подтверждено
// настоящим файлом, было бы неправдой.
func TestClassifierShape(t *testing.T) {
	m := openModel(t, fixtureByName(t, "testModel").Path)

	for _, c := range m.Classifiers() {
		if len(c.Rows) != 0 {
			t.Errorf("%s: строк %d, в этом файле справочники пусты", c.Name, len(c.Rows))
		}
		if len(c.Columns) != 1 {
			t.Fatalf("%s: колонок %d, ожидалась одна", c.Name, len(c.Columns))
		}
		col := c.Columns[0]
		if col.Name != "Name" || col.Type != "Core.Text" {
			t.Errorf("%s: колонка %q типа %q, ожидалась Name типа Core.Text", c.Name, col.Name, col.Type)
		}
	}
}

// TestSystemQualifiersSkipped — модель без справочников их и не выдумывает.
// Тринадцать системных квалификаторов и квалификатор работ в перенос не идут.
func TestSystemQualifiersSkipped(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ИзготовлениеЮбки").Path)

	if got := m.Classifiers(); len(got) != 0 {
		names := make([]string, len(got))
		for i, c := range got {
			names[i] = c.Name
		}
		t.Errorf("справочников %d, ожидалось ноль: %v", len(got), names)
	}
}

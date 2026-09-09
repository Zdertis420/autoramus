package generate_test

import (
	"bytes"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Заготовка — пустая, но настоящая модель IDEF0, сделанная в самом Ramus.
// Сочинять 55 таблиц, метаданные и служебные квалификаторы с нуля не нужно;
// генератор наполняет готовую пустую модель.

// TestTemplateIsEmptyModel — заготовка читается и не содержит модели.
// Если бы в ней оказалась хоть одна работа, она уехала бы в каждый
// собранный файл, и автор получил бы содержимое, которого не писал.
func TestTemplateIsEmptyModel(t *testing.T) {
	file, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(file)
	if err != nil {
		t.Fatal(err)
	}

	if n := len(m.Functions()); n != 0 {
		t.Errorf("работ в заготовке %d, ожидалось ноль", n)
	}
	if n := len(m.Streams()); n != 0 {
		t.Errorf("потоков в заготовке %d, ожидалось ноль", n)
	}
	if n := len(m.Sectors()); n != 0 {
		t.Errorf("секторов в заготовке %d, ожидалось ноль", n)
	}
}

// TestTemplateHasNoClassifiers — справочников в заготовке нет.
//
// Проверка сейчас выполняется сама собой: заготовку пересобрали без пяти
// «Каталогов», которые в ней были. Она стоит здесь стражем на будущее —
// заготовку могут пересобрать снова, и мусор вернётся незамеченным.
func TestTemplateHasNoClassifiers(t *testing.T) {
	file, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(file)
	if err != nil {
		t.Fatal(err)
	}

	if c := m.Classifiers(); len(c) != 0 {
		t.Errorf("справочников в заготовке %d, ожидалось ноль:", len(c))
		for _, one := range c {
			t.Errorf("  «%s»", one.Name)
		}
	}
}

// TestTemplateIsStable — две подряд сборки нетронутой заготовки дают одинаковые
// байты. На этом стоит вся детерминированность вывода: если заготовка
// пересобирается по-разному, дальше проверять нечего.
func TestTemplateIsStable(t *testing.T) {
	first, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}

	a, err := first.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("две сборки заготовки разошлись: %d и %d байт", len(a), len(b))
	}
}

// TestTemplateIndependent — каждый вызов отдаёт свою копию. Иначе одна сборка
// испортила бы заготовку для следующей, и два вызова в одном процессе дали бы
// разные файлы.
func TestTemplateIndependent(t *testing.T) {
	first, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}
	elements, err := first.Table("elements")
	if err != nil {
		t.Fatal(err)
	}
	before := len(elements.Rows)
	if _, err := elements.Add(map[string]string{
		"ELEMENT_ID": "999", "ELEMENT_NAME": "", "QUALIFIER_ID": "14",
		"CREATED_BRANCH_ID": "0", "REMOVED_BRANCH_ID": rsf.AliveBranch,
	}); err != nil {
		t.Fatal(err)
	}

	second, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}
	again, err := second.Table("elements")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Rows) != before {
		t.Errorf("правка первой заготовки видна во второй: строк %d, ожидалось %d",
			len(again.Rows), before)
	}
}

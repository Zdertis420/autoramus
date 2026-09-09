package rsf_test

import (
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// TestInventoryCoversEveryTable — проверка самой проверки. Опись строится по
// таблицам `attribute_*`, и если хоть одна непустая таблица не разбирается и не
// названа неэлементной, её содержимое не попадёт ни в опись, ни в перечень
// потерь. То есть дефект, ради которого затевалась работа, воспроизведётся
// внутри средства его обнаружения.
func TestInventoryCoversEveryTable(t *testing.T) {
	for _, model := range fixtures.All() {
		t.Run(model.Name, func(t *testing.T) {
			f, err := rsf.Open(model.Path)
			if err != nil {
				t.Fatal(err)
			}

			for _, table := range f.Tables {
				if !strings.HasPrefix(table.Name, "attribute_") || len(table.Rows) == 0 {
					continue
				}
				if rsf.IsNonElementTable(table.Name) {
					continue
				}
				if !rsf.HasElementKey(table) {
					t.Errorf("таблица %s непуста (%d строк), но опись её не разбирает "+
						"и в перечне неэлементных её нет", table.Name, len(table.Rows))
				}
			}
		})
	}
}

// TestKindDoesNotUseSystemFlag — опровергнутая гипотеза, вынесенная в проверку.
// F_SECTORS и F_STREAMS помечены системными квалификаторами, но несут данные
// автора. Отбор по QUALIFIER_SYSTEM выбросил бы их, и потери подписей стрелок
// остались бы незамеченными. Регрессия сюда вероятна: правило выглядит
// очевидным.
func TestKindDoesNotUseSystemFlag(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ФормированиеТП").Path)

	seen := make(map[rsf.ElementKind]int)
	for _, kind := range m.TransferableElements() {
		seen[kind]++
	}

	for _, want := range []struct {
		kind rsf.ElementKind
		name string
	}{
		{rsf.KindFunction, "работы"},
		{rsf.KindStream, "потоки"},
		{rsf.KindSector, "секторы"},
		{rsf.KindModel, "модель"},
	} {
		if seen[want.kind] == 0 {
			t.Errorf("среди переносимых элементов нет ни одного вида «%s»", want.name)
		}
	}
}

// TestAttributesOfFunction — опись работы содержит и то, что мы читаем, и то,
// что теряем. Она описывает файл, а не наши умения.
func TestAttributesOfFunction(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ФормированиеТП").Path)

	var function int64
	for id, kind := range m.TransferableElements() {
		if kind == rsf.KindFunction {
			function = id
			break
		}
	}
	if function == 0 {
		t.Fatal("в модели не нашлось ни одной работы")
	}

	names := make(map[string]bool)
	for _, a := range m.Attributes(function) {
		names[a.Name] = true
	}

	for _, want := range []string{
		"HierarchicalAttribute", // читаем: родитель
		"F_BOUNDS",              // читаем: геометрия
		"F_BACKGROUND",          // теряем: цвет
		"F_STATUS",              // теряем: статус
	} {
		if !names[want] {
			t.Errorf("в описи работы нет атрибута %s: %v", want, names)
		}
	}
}

// TestAttributeValuesHaveNoKeys — ключи записи в значение не входят. Иначе люк
// заполнится служебными числами, и читать его станет невозможно.
func TestAttributeValuesHaveNoKeys(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ФормированиеТП").Path)

	for element := range m.TransferableElements() {
		for _, a := range m.Attributes(element) {
			for _, key := range []string{"ATTRIBUTE_ID", "ELEMENT_ID", "ELEMENT1_ID", "VALUE_BRANCH_ID"} {
				if _, ok := a.Columns[key]; ok {
					t.Errorf("%s: в значении осталась ключевая колонка %s", a.Name, key)
				}
			}
		}
	}
}

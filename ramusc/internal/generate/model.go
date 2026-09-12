package generate

import (
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// writeModel записывает идентичность модели: подпись квалификатора работ,
// автора и лист.
//
// Самого имени модели здесь нет: оно приезжает вместе с корневой работой,
// потому что моделью и является она (Р0-2). Квалификатор подписывается тем же
// именем не ради идентичности — в настоящих файлах он зовётся «Работы» или
// «Activity boxes», — а ради того, чтобы автор не увидел в дереве Ramus
// заготовочный плейсхолдер «Имя модели».
func writeModel(m *rsf.Model, source *ir.Model) error {
	qualifiers, err := m.File.Table("qualifiers")
	if err != nil {
		return err
	}
	row, ok := qualifiers.First(rsf.Eq("QUALIFIER_ID", fmt.Sprint(m.FunctionQualifier)))
	if !ok {
		return fmt.Errorf("в заготовке нет квалификатора работ %d", m.FunctionQualifier)
	}
	if err := qualifiers.Set(row, "QUALIFIER_NAME", rsf.Text(source.Name.Name)); err != nil {
		return err
	}

	prefs, err := m.File.Table("attribute_model_preferences")
	if err != nil {
		return err
	}
	prefsRow, ok := prefs.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(m.Element)))
	if !ok {
		return fmt.Errorf("в заготовке нет свойств модели для элемента %d", m.Element)
	}

	// Автор и лист — единственное, что язык из свойств модели выражает.
	// Остальные текстовые поля заготовки — заглушки мастера Ramus
	// («Определение», «Использовано в»); в настоящей модели они пусты, и
	// класть чужой текст в модель автора нельзя.
	values := map[string]string{
		"PROJECT_AUTOR": source.Author,
		"DIAGRAM_SIZE":  source.Page,
		"DEFINITION":    "",
		"USED_AT":       "",
		"MODEL_LETTER":  "",
	}
	for column, value := range values {
		if err := prefs.Set(prefsRow, column, rsf.Text(value)); err != nil {
			return err
		}
	}

	// PROJECT_NAME не трогаем: языком оно не выражается, смысл его неизвестен,
	// а в настоящей модели там лежит остаток мастера («новый2»). Записать туда
	// имя модели значило бы выдать догадку за знание.

	return writeVisualData(m, m.Element)
}

// writeVisualData отмечает элемент как владельца диаграммы.
//
// Без этой строки Ramus не рисует на его диаграмме ни одного сегмента:
// SectorRefactor.loadFromFunction выходит по `return`, не дойдя до секторов,
// если у владельца нет F_VISUAL_DATA. Работам строку пишет writeFunction,
// элементу модели — эта функция: он владеет сегментами контекстной диаграммы
// A-0, и без неё A-0 оставалась пустой.
//
// Мера снята дифом: файл, собранный нами, открытый в Ramus и сохранённый
// после сдвига блока, отличается от исходного ровно этой строкой — секторы,
// точки и концы Ramus не тронул вовсе.
func writeVisualData(m *rsf.Model, element int64) error {
	attribute, found := m.Attribute("F_VISUAL_DATA")
	if !found {
		return fmt.Errorf("в заготовке нет атрибута F_VISUAL_DATA")
	}
	table, err := m.File.Table("attribute_visual_datas")
	if err != nil {
		return err
	}
	_, err = table.Add(map[string]string{
		"ATTRIBUTE_ID":    fmt.Sprint(attribute),
		"ELEMENT_ID":      fmt.Sprint(element),
		"DATA":            visualDataMask,
		"VALUE_BRANCH_ID": "0",
	})
	if err != nil {
		return fmt.Errorf("attribute_visual_datas: %w", err)
	}
	return nil
}

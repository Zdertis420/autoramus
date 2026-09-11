package generate

import (
	"fmt"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// File собирает файл Ramus из модели.
//
// Модель приходит уже проверенной: конвейер идёт лесенкой, и генератор
// запускается, только если валидатор не нашёл ошибок. Поэтому здесь не
// проверяется форма документа — только то, чего валидатор знать не может:
// хватает ли данных для записи файла.
//
// Стрелки эта версия не пишет. Файл получается неполным по существу, и
// сказать об этом автору обязана команда: молчаливая потеря на выходном конце
// конвейера ничем не лучше молчаливой потери на входном.
func File(m *ir.Model) (*rsf.File, error) {
	file, err := Template()
	if err != nil {
		return nil, err
	}
	target, err := rsf.NewModel(file)
	if err != nil {
		return nil, fmt.Errorf("заготовка не разбирается как модель: %w", err)
	}

	boxes, err := geometry(m)
	if err != nil {
		return nil, err
	}
	if err := writeModel(target, m); err != nil {
		return nil, err
	}
	functions, err := writeFunctions(target, m, boxes)
	if err != nil {
		return nil, err
	}

	// Порядок записи — работы, потоки, секторы — задаёт и номера элементов:
	// сектор ссылается на работу и на поток, и оба обязаны существовать
	// раньше него.
	streams, err := writeStreams(target, m)
	if err != nil {
		return nil, err
	}
	numbers, err := newCounters(file)
	if err != nil {
		return nil, err
	}
	if err := writeSectors(target, m, functions, streams, numbers); err != nil {
		return nil, err
	}
	// Последовательности ординат и кросспоинтов Ramus сам не перематывает,
	// поэтому выросшие значения возвращаются в файл.
	if err := numbers.flush(); err != nil {
		return nil, err
	}
	return file, nil
}

// geometry сводит раскладку в отображение «работа → прямоугольник» и требует,
// чтобы координаты нашлись у каждой работы.
//
// Автораскладки пока нет, и работа без координат — не мелочь, которую можно
// восполнить нулями: блоки легли бы друг на друга, а результат выглядел бы
// как работа программы, не будучи ею.
func geometry(m *ir.Model) (map[string]*ir.FunctionLayout, error) {
	boxes := make(map[string]*ir.FunctionLayout)
	if m.Layout != nil {
		for _, box := range m.Layout.Functions {
			boxes[box.Function.Name] = box
		}
	}

	var missing []string
	for _, f := range m.Functions {
		if _, ok := boxes[f.Name.Name]; !ok {
			missing = append(missing, "«"+f.Name.Name+"»")
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"у работ нет координат, а автораскладки пока нет: %s\nдобавьте секцию layout или дождитесь автораскладки",
			strings.Join(missing, ", "))
	}
	return boxes, nil
}

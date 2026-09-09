// Пакет generate собирает файл Ramus из IR.
//
// Знает только про ir и rsf: за счёт этого фронтенд можно заменить, не трогая
// работу с файлом, и наоборот (Р16). Файл собирается не с нуля, а поверх
// заготовки — пустой модели, сделанной самим Ramus.
package generate

import (
	"bytes"
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
	"github.com/Zdertis420/autoramus/ramusc/templates"
)

// Template открывает встроенную заготовку.
//
// Каждый вызов отдаёт свою копию: файл при наполнении правится на месте, и
// общая на всех заготовка означала бы, что вторая сборка в том же процессе
// получит следы первой.
//
// Ошибка здесь — вина программы, а не автора модели: заготовка лежит внутри
// бинаря, и испортить её вводом невозможно. Вызывающему следует ответить
// кодом 2.
func Template() (*rsf.File, error) {
	file, err := rsf.Read(bytes.NewReader(templates.IDEF0), int64(len(templates.IDEF0)))
	if err != nil {
		return nil, fmt.Errorf("встроенная заготовка не читается: %w", err)
	}
	return file, nil
}

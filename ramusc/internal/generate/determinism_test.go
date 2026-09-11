package generate_test

import (
	"bytes"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
)

// TestDeterministic — две сборки одной модели дают одинаковые байты.
//
// Без этого тесты пришлось бы писать через распаковку и структурное сравнение
// вместо сравнения хеша, а регрессия по раскладке перестала бы быть видимой
// диффом (принцип III конституции). Ловится этим в первую очередь обход map:
// раздача номеров элементам и порядок записи строк обязаны идти по документу.
func TestDeterministic(t *testing.T) {
	model := sourceModel(t)

	first, err := generate.File(model)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate.File(model)
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
		t.Errorf("две сборки одной модели разошлись: %d и %d байт", len(a), len(b))
	}
}

// TestDeterministicWithArrows — то же на документе со стрелками.
//
// Стрелки принесли три новых источника недетерминизма: раздачу номеров
// ординат, номеров узлов и порядок сегментов. Все три обязаны идти по
// документу, а не по обходу отображений.
func TestDeterministicWithArrows(t *testing.T) {
	for _, name := range []string{"skirt.yaml", "skirt.json", "skirt-full.yaml"} {
		t.Run(name, func(t *testing.T) {
			first, err := generate.File(documentModel(t, examplePath(name)))
			if err != nil {
				t.Fatal(err)
			}
			second, err := generate.File(documentModel(t, examplePath(name)))
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
				t.Errorf("две сборки одного документа разошлись: %d и %d байт", len(a), len(b))
			}
		})
	}
}

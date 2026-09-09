package generate_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Круг «документ → файл → разбор → документ» проходит целиком внутри проекта.
// Ради него декомпилятор и делали раньше срока: иначе генератор проверялся бы
// глазами в чужой программе, то есть не проверялся бы.
//
// Чего круг НЕ проверяет: стрелок, классификаторов и люка raw в файле нет по
// объёму фичи, поэтому расхождение по этим разделам ожидаемо и сравнением не
// покрывается. Появятся стрелки — сравнение расширится вместе с ними.

// modelOf декомпилирует настоящий файл в IR.
func modelOf(t *testing.T, path string) *ir.Model {
	t.Helper()

	file, err := rsf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	source, err := rsf.NewModel(file)
	if err != nil {
		t.Fatal(err)
	}
	model, err := decompile.Model(source)
	if err != nil {
		t.Fatal(err)
	}
	return model
}

// TestRoundTrip — что вошло, то и вышло.
//
// Идёт по перечню проверочных моделей, пропуская отвергаемые: у них документа
// нет по существу, и сравнивать нечего. Новая модель добавляется в перечень
// без правки этого теста.
func TestRoundTrip(t *testing.T) {
	for _, f := range fixtures.All() {
		if f.ExpectRefusal {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			want := modelOf(t, f.Path)

			built, err := generate.File(want)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := rsf.NewModel(built)
			if err != nil {
				t.Fatalf("собранный файл не читается: %v", err)
			}
			got, err := decompile.Model(parsed)
			if err != nil {
				t.Fatalf("собранный файл не декомпилируется: %v", err)
			}

			if got.Name.Name != want.Name.Name {
				t.Errorf("имя модели %q, ожидалось %q", got.Name.Name, want.Name.Name)
			}
			if got.Author != want.Author {
				t.Errorf("автор %q, ожидалось %q", got.Author, want.Author)
			}
			if got.Page != want.Page {
				t.Errorf("лист %q, ожидалось %q", got.Page, want.Page)
			}

			compareFunctions(t, got, want)
			compareGeometry(t, got, want)
		})
	}
}

// compareFunctions сверяет набор работ, их порядок и дерево.
//
// Порядок сравнивается вместе с составом: он осмыслен (Р3), и потерять его
// значило бы перемешать дерево в Ramus.
func compareFunctions(t *testing.T, got, want *ir.Model) {
	t.Helper()

	if len(got.Functions) != len(want.Functions) {
		t.Fatalf("работ %d, ожидалось %d", len(got.Functions), len(want.Functions))
	}
	for i := range want.Functions {
		if got.Functions[i].Name.Name != want.Functions[i].Name.Name {
			t.Errorf("работа %d: «%s», ожидалась «%s»",
				i, got.Functions[i].Name.Name, want.Functions[i].Name.Name)
			continue
		}
		if got.Functions[i].Of.Name != want.Functions[i].Of.Name {
			t.Errorf("работа «%s»: родитель «%s», ожидался «%s»",
				want.Functions[i].Name.Name, got.Functions[i].Of.Name, want.Functions[i].Of.Name)
		}
	}
}

// compareGeometry сверяет прямоугольники блоков.
func compareGeometry(t *testing.T, got, want *ir.Model) {
	t.Helper()

	if got.Layout == nil || want.Layout == nil {
		t.Fatal("в одной из моделей нет раскладки")
	}
	boxes := make(map[string]*ir.FunctionLayout)
	for _, box := range got.Layout.Functions {
		boxes[box.Function.Name] = box
	}
	for _, box := range want.Layout.Functions {
		other, ok := boxes[box.Function.Name]
		if !ok {
			t.Errorf("для работы «%s» геометрия не записана", box.Function.Name)
			continue
		}
		if other.X.Val != box.X.Val || other.Y.Val != box.Y.Val ||
			other.Width.Val != box.Width.Val || other.Height.Val != box.Height.Val {
			t.Errorf("работа «%s»: (%v, %v, %v, %v), ожидалось (%v, %v, %v, %v)",
				box.Function.Name,
				other.X.Val, other.Y.Val, other.Width.Val, other.Height.Val,
				box.X.Val, box.Y.Val, box.Width.Val, box.Height.Val)
		}
	}
}

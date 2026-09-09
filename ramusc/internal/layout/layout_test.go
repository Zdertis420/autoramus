package layout_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// Раскладка проверяется на настоящих документах проекта: skirt.yaml написан без
// единой координаты и ради него фича и затевалась.

// modelOf разбирает документ проекта в IR. Валидатор нужен не ради проверки —
// раскладка получает документ уже проверенным, — а потому что IR строит он.
func modelOf(t *testing.T, path string) *ir.Model {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	model, diags, internal := validate.Build(src)
	if internal != nil {
		t.Fatal(internal)
	}
	if diags.HasErrors() {
		t.Fatalf("документ %s невалиден", path)
	}
	if model == nil {
		t.Fatalf("модель по документу %s не построена", path)
	}
	return model
}

func example(name string) string { return filepath.Join("..", "..", "..", "examples", name) }

// boxes сводит раскладку модели в отображение «работа → её блок».
func boxes(m *ir.Model) map[string]*ir.FunctionLayout {
	out := make(map[string]*ir.FunctionLayout)
	if m.Layout == nil {
		return out
	}
	for _, box := range m.Layout.Functions {
		out[box.Function.Name] = box
	}
	return out
}

// TestEveryFunctionPlaced — после раскладки геометрия есть у каждой работы.
//
// Генератор сохраняет свою проверку на пустые координаты, но она — страховка от
// нашего же дефекта. Ловить его надо здесь.
func TestEveryFunctionPlaced(t *testing.T) {
	m := modelOf(t, example("skirt.yaml"))
	layout.Apply(m)

	placed := boxes(m)
	if len(placed) != len(m.Functions) {
		t.Fatalf("размещено %d работ из %d", len(placed), len(m.Functions))
	}
	for _, f := range m.Functions {
		if _, ok := placed[f.Name.Name]; !ok {
			t.Errorf("работа «%s» осталась без координат", f.Name.Name)
		}
	}
}

// TestDiagramsAreSeparate — дети разных родителей не смешиваются.
//
// Диаграмма — работа вместе с её непосредственными детьми, и у каждой своя
// диагональ. Если бы все работы модели раскладывались единым списком, дети
// одного родителя разъехались бы по чужим диаграммам (FR-004).
func TestDiagramsAreSeparate(t *testing.T) {
	m := modelOf(t, example("skirt.yaml"))
	layout.Apply(m)

	placed := boxes(m)

	// Корневая работа лежит одна на контекстной диаграмме A-0; три подработы —
	// вместе на её диаграмме. Значит подработы обязаны занять всю диагональ, а
	// корневая — стоять отдельно от них.
	var children []string
	for _, f := range m.Functions {
		if f.Of.Name != "" {
			children = append(children, f.Name.Name)
		}
	}
	if len(children) != 3 {
		t.Fatalf("подработ %d, ожидалось 3", len(children))
	}

	first := placed[children[0]]
	if first == nil {
		t.Fatal("первая подработа не размещена")
	}
	if first.X.Val != 80 || first.Y.Val != 80 {
		t.Errorf("первая подработа в (%v, %v), ожидалось начало диагонали (80, 80)",
			first.X.Val, first.Y.Val)
	}

	root := placed["Изготовление юбки"]
	if root == nil {
		t.Fatal("корневая работа не размещена")
	}
	if root.X.Val == first.X.Val && root.Y.Val == first.Y.Val {
		t.Error("корневая работа села на ту же точку, что подработа: диаграммы смешались")
	}
}

// TestMatchesRamus — раскладка совпадает с той, что делает сам Ramus.
//
// Эталон читается из examples/тест.rsf разбором, а не переписан в тест
// константами: если формулу однажды поправят, сверяться надо с файлом, а не с
// нашим прежним пониманием файла.
//
// Модель эта — единственная, где Ramus разложил подработы сам и никто их не
// двигал: все четыре одного размера. Совпадение проверяется до последнего знака.
func TestMatchesRamus(t *testing.T) {
	file, err := rsf.Open(example("тест.rsf"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := rsf.NewModel(file)
	if err != nil {
		t.Fatal(err)
	}

	// Подработы: у корневой родитель — элемент модели, у прочих — работа.
	var want []rsf.Function
	for _, f := range source.Functions() {
		if f.Parent != source.Element {
			want = append(want, f)
		}
	}
	if len(want) != 4 {
		t.Fatalf("в тест.rsf подработ %d, ожидалось 4", len(want))
	}

	names := make([]string, len(want))
	for i, f := range want {
		names[i] = f.Name
	}
	m := model("корень", names...)
	layout.Apply(m)

	placed := boxes(m)
	for _, f := range want {
		got, ok := placed[f.Name]
		if !ok {
			t.Errorf("работа «%s» не размещена", f.Name)
			continue
		}
		if f.Bounds == nil {
			t.Fatalf("у работы «%s» в тест.rsf нет прямоугольника", f.Name)
		}
		if got.X.Val != f.Bounds.X || got.Y.Val != f.Bounds.Y ||
			got.Width.Val != f.Bounds.Width || got.Height.Val != f.Bounds.Height {
			t.Errorf("«%s»: (%v, %v, %v, %v), Ramus кладёт (%v, %v, %v, %v)",
				f.Name, got.X.Val, got.Y.Val, got.Width.Val, got.Height.Val,
				f.Bounds.X, f.Bounds.Y, f.Bounds.Width, f.Bounds.Height)
		}
	}
}

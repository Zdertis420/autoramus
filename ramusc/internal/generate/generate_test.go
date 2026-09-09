package generate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Проверки идут на настоящей модели, а не на придуманной: «ИзготовлениеЮбки»
// единственная из перечня несёт полную раскладку, а без координат генератор
// отказывает по существу (П-5).

// sourceModel отдаёт IR настоящей модели: файл Ramus, разобранный и
// декомпилированный. Сочинять модель руками незачем — эта проверена
// round-trip-тестом и содержит всё, что генератору предстоит записать.
func sourceModel(t *testing.T) *ir.Model {
	t.Helper()

	var path string
	for _, f := range fixtures.All() {
		if f.Name == "ИзготовлениеЮбки" {
			path = f.Path
		}
	}
	if path == "" {
		t.Fatal("в перечне нет модели ИзготовлениеЮбки")
	}

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

// built собирает файл из настоящей модели и открывает его обратно.
func built(t *testing.T) (*rsf.File, *rsf.Model) {
	t.Helper()

	file, err := generate.File(sourceModel(t))
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(file)
	if err != nil {
		t.Fatalf("собранный файл не читается: %v", err)
	}
	return file, m
}

// preference читает колонку из строки свойств модели.
func preference(t *testing.T, file *rsf.File, element int64, column string) string {
	t.Helper()

	prefs, err := file.Table("attribute_model_preferences")
	if err != nil {
		t.Fatal(err)
	}
	row, ok := prefs.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(element)))
	if !ok {
		t.Fatalf("в свойствах модели нет строки на элемент %d", element)
	}
	return prefs.Str(row, column)
}

// TestModelIdentity — имя, автор и лист доезжают до файла.
//
// Имя модели — это имя корневой работы (Р0-2): отдельной сущности «модель» в
// файле нет. Квалификатор работ подписывается тем же именем, но идентичностью
// не является — в «ФормированииТП» он зовётся «Работы».
func TestModelIdentity(t *testing.T) {
	source := sourceModel(t)
	file, m := built(t)

	var root *rsf.Function
	for _, f := range m.Functions() {
		if f.Parent == m.Element {
			root = &f
			break
		}
	}
	if root == nil {
		t.Fatal("в собранном файле нет корневой работы")
	}
	if root.Name != source.Name.Name {
		t.Errorf("имя корневой работы %q, ожидалось %q", root.Name, source.Name.Name)
	}
	if got := m.QualifierName(); got != source.Name.Name {
		t.Errorf("подпись квалификатора работ %q, ожидалось %q", got, source.Name.Name)
	}
	if got := preference(t, file, m.Element, "PROJECT_AUTOR"); got != source.Author {
		t.Errorf("автор %q, ожидалось %q", got, source.Author)
	}
	if got := preference(t, file, m.Element, "DIAGRAM_SIZE"); got != source.Page {
		t.Errorf("лист %q, ожидалось %q", got, source.Page)
	}
}

// TestModelPlaceholdersCleared — заглушки мастера Ramus не доезжают до автора.
//
// В заготовке свойства модели заполнены словами «Определение» и «Использовано
// в». Оставить их значило бы положить в каждую собранную модель чужой текст:
// автор его не писал (FR-015).
func TestModelPlaceholdersCleared(t *testing.T) {
	file, m := built(t)

	for _, column := range []string{"DEFINITION", "USED_AT", "MODEL_LETTER"} {
		if got := preference(t, file, m.Element, column); got != "" {
			t.Errorf("%s = %q, ожидалась пустая строка", column, got)
		}
	}
}

// TestProjectNameUntouched — «имя проекта» не трогаем.
//
// Языком оно не выражается, смысл его нам неизвестен, а в настоящей модели там
// лежит остаток мастера («новый2»). Записывать туда имя модели значило бы
// выдать догадку за знание.
func TestProjectNameUntouched(t *testing.T) {
	file, m := built(t)

	template, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}
	clean, err := rsf.NewModel(template)
	if err != nil {
		t.Fatal(err)
	}

	want := preference(t, template, clean.Element, "PROJECT_NAME")
	if got := preference(t, file, m.Element, "PROJECT_NAME"); got != want {
		t.Errorf("PROJECT_NAME = %q, ожидалось %q — поле не наше", got, want)
	}
}

// TestFunctionRows — на каждую работу появляются все нужные строки.
//
// Состав выведен из настоящей модели перебором таблиц: пять строк обязательны,
// без них работа неполна, — плюс оформление, без которого не проверено, как
// Ramus её отрисует.
func TestFunctionRows(t *testing.T) {
	file, m := built(t)

	required := []string{
		"elements",
		"attribute_hierarchicals",
		"attribute_texts",
		"attribute_rectangles",
		"attribute_function_types",
		"attribute_fonts",
		"attribute_colors",
		"attribute_statuses",
		"attribute_visual_datas",
	}

	functions := m.Functions()
	if len(functions) == 0 {
		t.Fatal("в собранном файле нет работ")
	}
	for _, f := range functions {
		for _, name := range required {
			table, err := file.Table(name)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if len(table.Select(rsf.Eq("ELEMENT_ID", fmt.Sprint(f.ID)))) == 0 {
				t.Errorf("работа %d («%s»): нет строки в %s", f.ID, f.Name, name)
			}
		}
	}
}

// TestNoDatesWritten — даты не пишутся вовсе.
//
// Записать «как в жизни» означало бы текущее время, и две сборки одной модели
// разошлись бы файлами. Детерминированность вывода дороже дат, которых в языке
// всё равно нет.
func TestNoDatesWritten(t *testing.T) {
	file, m := built(t)

	dates, err := file.Table("attribute_dates")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range m.Functions() {
		if rows := dates.Select(rsf.Eq("ELEMENT_ID", fmt.Sprint(f.ID))); len(rows) != 0 {
			t.Errorf("работа %d («%s»): записано дат %d, ожидалось ноль", f.ID, f.Name, len(rows))
		}
	}
}

// TestFunctionTree — дерево работ повторяет документ.
//
// Родитель корневой работы — элемент модели, а не -1: дерево подвешено к
// модели, а не висит в воздухе (П-2).
func TestFunctionTree(t *testing.T) {
	source := sourceModel(t)
	_, m := built(t)

	byName := make(map[string]rsf.Function)
	for _, f := range m.Functions() {
		byName[f.Name] = f
	}
	if len(byName) != len(source.Functions) {
		t.Fatalf("работ в файле %d, в документе %d", len(byName), len(source.Functions))
	}

	for _, want := range source.Functions {
		got, ok := byName[want.Name.Name]
		if !ok {
			t.Errorf("работы «%s» нет в файле", want.Name.Name)
			continue
		}
		if want.Of.Name == "" {
			if got.Parent != m.Element {
				t.Errorf("корневая «%s»: родитель %d, ожидался элемент модели %d",
					want.Name.Name, got.Parent, m.Element)
			}
			continue
		}
		parent, ok := byName[want.Of.Name]
		if !ok {
			t.Errorf("родителя «%s» нет в файле", want.Of.Name)
			continue
		}
		if got.Parent != parent.ID {
			t.Errorf("работа «%s»: родитель %d, ожидался %d («%s»)",
				want.Name.Name, got.Parent, parent.ID, parent.Name)
		}
	}
}

// TestFunctionOrder — порядок соседей задан цепочкой, а не номерами.
//
// Порядок объявления в документе осмыслен (Р3) и обязан быть виден в дереве
// Ramus. Цепочка PREVIOUS_ELEMENT_ID — единственное, что его несёт.
func TestFunctionOrder(t *testing.T) {
	source := sourceModel(t)
	_, m := built(t)

	byName := make(map[string]rsf.Function)
	for _, f := range m.Functions() {
		byName[f.Name] = f
	}

	// Ожидаемая цепочка: у первого ребёнка каждого родителя предыдущего нет,
	// у остальных — сосед, объявленный перед ним.
	previous := make(map[string]string)
	last := make(map[string]string)
	for _, f := range source.Functions {
		parent := f.Of.Name
		previous[f.Name.Name] = last[parent]
		last[parent] = f.Name.Name
	}

	for name, want := range previous {
		got := byName[name]
		if want == "" {
			if got.Previous != -1 {
				t.Errorf("работа «%s»: предыдущий %d, ожидался -1", name, got.Previous)
			}
			continue
		}
		if got.Previous != byName[want].ID {
			t.Errorf("работа «%s»: предыдущий %d, ожидался %d («%s»)",
				name, got.Previous, byName[want].ID, want)
		}
	}
}

// TestFunctionTypes — тип работы выводится из положения в дереве.
//
// У корневой 3, у остальных 1 — так пишет сам Ramus во всех трёх настоящих
// моделях. Поле Type в IR к этому отношения не имеет: оно про элементы DFD,
// снятые с объёма фичей 001 (П-8).
func TestFunctionTypes(t *testing.T) {
	_, m := built(t)

	for _, f := range m.Functions() {
		want := rsf.TypeProcess
		if f.Parent == m.Element {
			want = rsf.TypeOperation
		}
		if f.Type != want {
			t.Errorf("работа «%s»: F_TYPE = %d, ожидалось %d", f.Name, f.Type, want)
		}
	}
}

// TestGeometry — прямоугольники берутся из раскладки документа.
func TestGeometry(t *testing.T) {
	source := sourceModel(t)
	_, m := built(t)

	want := make(map[string]*ir.FunctionLayout)
	for _, l := range source.Layout.Functions {
		want[l.Function.Name] = l
	}

	for _, f := range m.Functions() {
		box, ok := want[f.Name]
		if !ok {
			t.Errorf("для работы «%s» в документе нет геометрии", f.Name)
			continue
		}
		if f.Bounds == nil {
			t.Errorf("работа «%s»: прямоугольник не записан", f.Name)
			continue
		}
		if f.Bounds.X != box.X.Val || f.Bounds.Y != box.Y.Val ||
			f.Bounds.Width != box.Width.Val || f.Bounds.Height != box.Height.Val {
			t.Errorf("работа «%s»: (%v, %v, %v, %v), ожидалось (%v, %v, %v, %v)",
				f.Name, f.Bounds.X, f.Bounds.Y, f.Bounds.Width, f.Bounds.Height,
				box.X.Val, box.Y.Val, box.Width.Val, box.Height.Val)
		}
	}
}

// TestRefusesWithoutLayout — работа без координат даёт отказ, а не блок в
// точке (0, 0).
//
// Автораскладки пока нет. Выдать блоки, наложенные друг на друга, значило бы
// отдать результат, который выглядит как работа программы, а является мусором
// (П-5, FR-013).
func TestRefusesWithoutLayout(t *testing.T) {
	model := sourceModel(t)
	model.Layout = nil

	_, err := generate.File(model)
	if err == nil {
		t.Fatal("модель без раскладки собралась, ожидался отказ")
	}
	if !strings.Contains(err.Error(), "координат") {
		t.Errorf("в сообщении не сказано о координатах: %v", err)
	}
	for _, f := range model.Functions {
		if !strings.Contains(err.Error(), f.Name.Name) {
			t.Errorf("в сообщении не названа работа «%s»: %v", f.Name.Name, err)
		}
	}
}

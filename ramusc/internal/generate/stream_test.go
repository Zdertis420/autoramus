package generate_test

import (
	"fmt"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// TestStreamOrder — потоки лежат в файле в порядке секции flows.
//
// Порядок задаётся цепочкой PREVIOUS_ELEMENT_ID, и Ramus показывает потоки
// именно в ней. Обход отображения дал бы разный порядок от запуска к запуску.
func TestStreamOrder(t *testing.T) {
	source := documentModel(t, examplePath("skirt.yaml"))
	file, err := generate.File(source)
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(file)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, s := range m.Streams() {
		got = append(got, s.Name)
	}
	if len(got) != len(source.Flows) {
		t.Fatalf("потоков %d, в документе объявлено %d", len(got), len(source.Flows))
	}
	for i, flow := range source.Flows {
		if got[i] != flow.Name {
			t.Errorf("поток %d: «%s», ожидался «%s»", i, got[i], flow.Name)
		}
	}
}

// TestStreamHierarchy — потоки сцеплены и не имеют родителя.
//
// В настоящих файлах PARENT_ELEMENT_ID у всех потоков равен -1, а PREVIOUS
// указывает на предыдущий. Родитель, поставленный по ошибке, спрятал бы поток
// внутрь работы.
func TestStreamHierarchy(t *testing.T) {
	file, m := buildFrom(t, examplePath("skirt.yaml"))

	hierarchicals, err := file.Table("attribute_hierarchicals")
	if err != nil {
		t.Fatal(err)
	}
	attribute, ok := m.Attribute("HierarchicalAttribute")
	if !ok {
		t.Fatal("в файле нет атрибута HierarchicalAttribute")
	}

	previous := int64(-1)
	for _, s := range m.Streams() {
		row, ok := hierarchicals.First(
			rsf.Eq("ELEMENT_ID", itoa(s.ID)),
			rsf.Eq("ATTRIBUTE_ID", itoa(attribute)))
		if !ok {
			t.Errorf("у потока «%s» нет места в дереве", s.Name)
			continue
		}
		if parent := hierarchicals.Int64Or(row, "PARENT_ELEMENT_ID", 0); parent != -1 {
			t.Errorf("у потока «%s» родитель %d, ожидался -1", s.Name, parent)
		}
		if got := hierarchicals.Int64Or(row, "PREVIOUS_ELEMENT_ID", 0); got != previous {
			t.Errorf("у потока «%s» предыдущий %d, ожидался %d", s.Name, got, previous)
		}
		previous = s.ID
	}
}

// itoa печатает номер элемента: в таблицах они лежат строками.
func itoa(v int64) string { return fmt.Sprint(v) }

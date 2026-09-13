package generate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// Туннель в Ramus не записан признаком, а вычисляется: скобки рисуются там,
// где у узла нет входов либо нет выходов. Значит компилятор отвечает за них,
// даже не подозревая об их существовании, — и проверять их надо на собранном
// файле, а не на IR.

// documentsWithTunnels — документы, по которым идёт проверка. Все валидны и все
// собираются; новый документ добавляется сюда, код теста при этом не трогается.
func documentsWithTunnels() []string {
	root := filepath.Join("..", "..", "..")
	docs := filepath.Join("..", "..", "testdata", "documents")
	return []string{
		filepath.Join(root, "examples", "skirt.yaml"),
		filepath.Join(root, "examples", "skirt-full.yaml"),
		filepath.Join(docs, "feedback.yaml"),
		filepath.Join(docs, "nested.yaml"),
		filepath.Join(docs, "staircase.yaml"),
		filepath.Join(docs, "two-sides.yaml"),
		filepath.Join(docs, "dfd-tunnel.yaml"),
	}
}

// TestOnlyRequestedTunnels — в собранном файле туннельных концов ровно столько,
// сколько автор объявил полем tunnel (SC-001).
//
// Скобки, которых автор не просил, означают оборванную стрелку: конец остался
// без пары на соседнем уровне. Именно так пропадала стрелка на A-0, когда
// оверрайд layout стирал поток целиком по всей модели.
func TestOnlyRequestedTunnels(t *testing.T) {
	for _, path := range documentsWithTunnels() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			model, diags, internal := validate.Build(src)
			if internal != nil {
				t.Fatal(internal)
			}
			if diags.HasErrors() {
				t.Fatalf("документ невалиден: %v", diags)
			}
			layout.Apply(model)

			file, err := generate.File(model)
			if err != nil {
				t.Fatal(err)
			}
			built, err := rsf.NewModel(file)
			if err != nil {
				t.Fatalf("собранный файл не читается: %v", err)
			}

			want := declaredTunnels(model)
			got := built.Tunnels()
			if len(got) != want {
				t.Errorf("туннельных концов %d, объявлено %d:", len(got), want)
				for _, tn := range got {
					t.Errorf("  сектор %d, узел %d (вход=%d выход=%d)",
						tn.Sector, tn.Crosspoint, tn.Ins, tn.Outs)
				}
			}
		})
	}
}

// declaredTunnels считает, сколько концов автор объявил туннельными.
func declaredTunnels(m *ir.Model) int {
	n := 0
	for _, f := range m.Functions {
		n += len(f.Tunnel)
	}
	return n
}

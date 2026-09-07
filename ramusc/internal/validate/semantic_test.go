package validate_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// TestSemanticClean — расширенный пример обязан проходить вообще без замечаний,
// включая предупреждения. Эталоны в testdata ловят только присутствие
// сообщений; ложное срабатывание на здоровой модели заметит именно этот тест.
func TestSemanticClean(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "skirt-full.yaml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	diags, err := validate.Source(src)
	if err != nil {
		t.Fatalf("внутренняя ошибка: %v", err)
	}
	if len(diags) != 0 {
		var text bytes.Buffer
		_ = diags.WriteText(&text, filepath.Base(path))
		t.Errorf("на здоровой модели есть замечания:\n%s", text.String())
	}
}

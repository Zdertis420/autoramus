package decompile_test

import (
	"bytes"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// TestOutputPasses — ради этого декомпилятор и затевался: настоящая модель,
// пропущенная через язык, обязана проходить проверку. Три предупреждения
// ожидаемы — в файле три потока без единой стрелки, остатки переименования.
func TestOutputPasses(t *testing.T) {
	m := decompiled(t)

	for _, format := range []struct {
		name  string
		write func(*bytes.Buffer) error
	}{
		{"YAML", func(b *bytes.Buffer) error { return decompile.WriteYAML(b, m, decompile.Options{}) }},
		{"JSON", func(b *bytes.Buffer) error { return decompile.WriteJSON(b, m) }},
	} {
		t.Run(format.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := format.write(&buf); err != nil {
				t.Fatal(err)
			}

			diags, err := validate.Source(buf.Bytes())
			if err != nil {
				t.Fatalf("внутренняя ошибка: %v", err)
			}
			errors, warnings := diags.Count()
			if errors != 0 {
				var text bytes.Buffer
				_ = diags.WriteText(&text, "вывод")
				t.Errorf("ошибок %d:\n%s", errors, text.String())
			}
			if warnings != 3 {
				t.Errorf("предупреждений %d, ожидалось 3", warnings)
			}
			for _, d := range diags {
				if d.Code != diag.CodeUnusedFlow {
					t.Errorf("неожиданная диагностика: %+v", d)
				}
			}
		})
	}
}

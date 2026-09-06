package validate_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

var update = flag.Bool("update", false, "переписать эталонные файлы диагностики")

// documents собирает документы моделей из каталога, не хватая заглушки
// и прочие посторонние файлы.
func documents(dir string) ([]string, error) {
	var out []string
	for _, ext := range []string{"*.json", "*.yaml", "*.yml"} {
		found, err := filepath.Glob(filepath.Join(dir, ext))
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	sort.Strings(out)
	return out, nil
}

// TestGolden гоняет весь набор моделей и сверяет диагностику с эталоном.
// Эталоны обновляются `go test ./internal/validate -update`.
func TestGolden(t *testing.T) {
	models, err := documents(filepath.Join("..", "..", "testdata", "models"))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("нет моделей в testdata/models")
	}

	for _, model := range models {
		t.Run(filepath.Base(model), func(t *testing.T) {
			src, err := os.ReadFile(model)
			if err != nil {
				t.Fatal(err)
			}
			diags, err := validate.Source(src)
			if err != nil {
				t.Fatalf("внутренняя ошибка: %v", err)
			}
			if !diags.HasErrors() {
				t.Fatalf("модель %s задумана битой, но ошибок не найдено", filepath.Base(model))
			}

			var got bytes.Buffer
			if err := diags.WriteJSON(&got); err != nil {
				t.Fatal(err)
			}

			stem := strings.TrimSuffix(filepath.Base(model), filepath.Ext(model))
			goldenPath := filepath.Join("..", "..", "testdata", "golden", stem+".diag.json")
			if *update {
				if err := os.WriteFile(goldenPath, got.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("нет эталона (создайте его флагом -update): %v", err)
			}
			if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got.Bytes())) {
				t.Errorf("диагностика разошлась с эталоном %s\n--- эталон ---\n%s\n--- получено ---\n%s",
					goldenPath, want, got.Bytes())
			}
		})
	}
}

// TestExamplesAreValid — примеры из поставки обязаны проходить проверку.
// Они же служат few-shot образцами для нейросети, битым там быть нечему.
func TestExamplesAreValid(t *testing.T) {
	examples, err := documents(filepath.Join("..", "..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) == 0 {
		t.Fatal("нет примеров в examples/")
	}
	for _, example := range examples {
		t.Run(filepath.Base(example), func(t *testing.T) {
			src, err := os.ReadFile(example)
			if err != nil {
				t.Fatal(err)
			}
			diags, err := validate.Source(src)
			if err != nil {
				t.Fatalf("внутренняя ошибка: %v", err)
			}
			if diags.HasErrors() {
				var text bytes.Buffer
				_ = diags.WriteText(&text, filepath.Base(example))
				t.Errorf("пример не проходит проверку:\n%s", text.String())
			}
		})
	}
}

// TestPositionsArePresent — контракт с GUI: без строки и колонки маркеры
// на полях редактора поставить не из чего.
func TestPositionsArePresent(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "models", "unknown-field.json"))
	if err != nil {
		t.Fatal(err)
	}
	diags, err := validate.Source(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Line == 0 || d.Column == 0 {
			t.Errorf("диагностика без позиции: %+v", d)
		}
		if d.Code == "" || d.Message == "" {
			t.Errorf("диагностика без кода или текста: %+v", d)
		}
	}
	if len(diags) != 1 {
		t.Fatalf("ожидалась одна ошибка, получено %d: %+v", len(diags), diags)
	}
	if got, want := diags[0].Code, diag.CodeSchemaUnknownField; got != want {
		t.Errorf("код = %s, ожидался %s", got, want)
	}
	if got, want := diags[0].Path, "/functions/0/inputs"; got != want {
		t.Errorf("путь = %s, ожидался %s", got, want)
	}
}

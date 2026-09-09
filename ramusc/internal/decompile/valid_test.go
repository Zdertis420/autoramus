package decompile_test

import (
	"bytes"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// warnings — сколько предупреждений ожидается от модели. Число задаётся по
// модели, а не общей константой: предупреждения бывают верным поведением,
// и требовать от всех файлов одинакового исхода значило бы проверять не то.
var warnings = map[string]int{
	// Три потока без единой стрелки — остатки переименования, они и правда
	// лежат в файле.
	"ИзготовлениеЮбки": 3,
}

// TestOutputPasses — ради этого декомпилятор и затевался: настоящая модель,
// пропущенная через язык, обязана проходить проверку. Идёт по всему перечню:
// раньше проверялась одна модель, и дефекты, которых она не показывала,
// оставались незамеченными.
func TestOutputPasses(t *testing.T) {
	for _, f := range fixtures.All() {
		if f.ExpectRefusal {
			// Файл языком не выражается, документа для него нет, и проверять
			// нечего. Это не пропуск проверки, а её отсутствие по существу.
			continue
		}

		t.Run(f.Name, func(t *testing.T) {
			m := decompiledByName(t, f.Name)

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
					errors, warns := diags.Count()
					if errors != 0 {
						var text bytes.Buffer
						_ = diags.WriteText(&text, "вывод")
						t.Errorf("ошибок %d:\n%s", errors, text.String())
					}
					if want := warnings[f.Name]; warns != want {
						t.Errorf("предупреждений %d, ожидалось %d", warns, want)
					}
					for _, d := range diags {
						if d.Code != diag.CodeUnusedFlow {
							t.Errorf("неожиданная диагностика: %+v", d)
						}
					}
				})
			}
		})
	}
}

// TestRefusedModelsAreRefused — модель, помеченная в перечне как невыразимая,
// обязана и правда давать находки. Иначе пометка тихо выключит проверку
// TestOutputPasses для файла, который на самом деле в порядке.
func TestRefusedModelsAreRefused(t *testing.T) {
	for _, f := range fixtures.All() {
		if !f.ExpectRefusal {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			if found := sourceOf(t, f.Name).Unsupported(); len(found) == 0 {
				t.Error("модель помечена невыразимой, но находок нет")
			}
		})
	}
}

// remaining — сколько видов содержимого не доехало до документа даже люком.
//
// Осталась одна модель: прежде здесь стояло «ФормированиеТП: 6», но шесть
// секторов там складывались в одну стрелку без имени, а такие модели теперь
// отвергаются целиком (Р18), и ожидать от них потерь больше нельзя.
//
// Ноль у «Изготовления юбки» означает, что люк уносит всё. Другого исхода на
// сегодняшнем наборе моделей не бывает: единственная известная причина
// остаточных потерь — сектор без потока, а он ведёт к отказу.
var remaining = map[string]int{
	"ИзготовлениеЮбки": 0,
}

// TestNothingIsLostSilently — после переноса через люк не остаётся ничего, о
// чём автору не сказали. Ноль там, где всё унесено; названное число там, где
// содержимое языком не выражается.
func TestNothingIsLostSilently(t *testing.T) {
	for _, f := range fixtures.All() {
		if f.ExpectRefusal {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			_, lost, err := decompile.ModelWithReport(sourceOf(t, f.Name))
			if err != nil {
				t.Fatal(err)
			}

			want, ok := remaining[f.Name]
			if !ok {
				t.Fatalf("для модели %s не задано ожидаемое число потерь", f.Name)
			}
			if len(lost) != want {
				t.Errorf("потерь %d, ожидалось %d:", len(lost), want)
				for _, l := range lost {
					t.Errorf("  %s", l)
				}
			}
		})
	}
}

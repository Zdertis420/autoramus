package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

func exec(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func example(name string) string { return filepath.Join("..", "..", "..", "examples", name) }
func model(name string) string   { return filepath.Join("..", "..", "testdata", "models", name) }

// document — валидный документ-фикстура. Битые модели лежат в models, эти — в
// documents: их битость никто не проверяет, они для другого.
func document(name string) string {
	return filepath.Join("..", "..", "testdata", "documents", name)
}

func TestValidateExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"валидный JSON", []string{"validate", example("skirt.json")}, exitOK},
		{"валидный YAML", []string{"validate", example("skirt.yaml")}, exitOK},
		{"невалидная модель", []string{"validate", model("unknown-field.json")}, exitInvalid},
		{"битый синтаксис", []string{"validate", model("broken-syntax.json")}, exitInvalid},
		{"файла нет", []string{"validate", model("такого-нет.json")}, exitInternal},
		{"без аргументов", nil, exitInternal},
		{"неизвестный флаг", []string{"validate", example("skirt.json"), "--что-то"}, exitInternal},
		{"два файла", []string{"validate", example("skirt.json"), example("skirt.yaml")}, exitInternal},
		{"справка", []string{"help"}, exitOK},
		{"версия", []string{"version"}, exitOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, _ := exec(t, tt.args...); code != tt.want {
				t.Errorf("код возврата = %d, ожидался %d", code, tt.want)
			}
		})
	}
}

// Флаг после имени файла — именно та форма вызова, что описана в CLAUDE.md.
func TestValidateJSONOutput(t *testing.T) {
	code, stdout, _ := exec(t, "validate", model("unknown-field.json"), "--json")
	if code != exitInvalid {
		t.Fatalf("код возврата = %d, ожидался %d", code, exitInvalid)
	}
	var diags diag.List
	if err := json.Unmarshal([]byte(stdout), &diags); err != nil {
		t.Fatalf("вывод --json не разбирается: %v\n%s", err, stdout)
	}
	if len(diags) != 1 {
		t.Fatalf("ожидалась одна диагностика, получено %d", len(diags))
	}
	if diags[0].Line == 0 || diags[0].Column == 0 {
		t.Errorf("диагностика без позиции: %+v", diags[0])
	}
}

// На валидной модели --json обязан печатать пустой массив, а не пустоту:
// GUI разбирает вывод безусловно.
func TestValidateJSONOutputOnSuccess(t *testing.T) {
	code, stdout, _ := exec(t, "validate", example("skirt.json"), "--json")
	if code != exitOK {
		t.Fatalf("код возврата = %d, ожидался %d", code, exitOK)
	}
	var diags diag.List
	if err := json.Unmarshal([]byte(stdout), &diags); err != nil {
		t.Fatalf("вывод --json не разбирается: %v\n%s", err, stdout)
	}
	if len(diags) != 0 {
		t.Errorf("ожидался пустой массив, получено: %+v", diags)
	}
}

// Здесь стояла проверка TestCompileNotImplemented: компиляция доходила до
// конца проверки и честно говорила, что генератора нет. Генератор подключён,
// и проверять больше нечего. Её место занял разбор настоящих исходов
// компиляции ниже — TestCompile*.

// Невалидную модель компиляция обязана отвергать теми же сообщениями,
// что и validate: двух наборов сообщений об одном и том же быть не должно.
func TestCompileSharesValidation(t *testing.T) {
	compileCode, compileOut, _ := exec(t, model("unknown-field.json"), "--json")
	validateCode, validateOut, _ := exec(t, "validate", model("unknown-field.json"), "--json")
	if compileCode != exitInvalid || validateCode != exitInvalid {
		t.Fatalf("коды возврата: компиляция %d, проверка %d", compileCode, validateCode)
	}
	if compileOut != validateOut {
		t.Errorf("диагностика разошлась:\n--- компиляция ---\n%s\n--- проверка ---\n%s",
			compileOut, validateOut)
	}
}

func TestParseArgs(t *testing.T) {
	opts, err := parseArgs([]string{"model.json", "-o", "out.rsf", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.files) != 1 || opts.files[0] != "model.json" {
		t.Errorf("файлы = %v", opts.files)
	}
	if opts.output != "out.rsf" {
		t.Errorf("-o = %q", opts.output)
	}
	if !opts.jsonOut {
		t.Error("--json не распознан")
	}
	if _, err := parseArgs([]string{"-o"}); err == nil {
		t.Error("флаг -o без значения принят")
	}
}

// Компиляция. Ветка проверки у неё общая с validate, поэтому двух наборов
// сообщений об одной и той же ошибке не возникает; расходятся они только тем,
// что делают после успешной проверки.

// compiled компилирует документ во временный файл и отдаёт путь к нему.
func compiled(t *testing.T, source string) (path string, code int, stderr string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "out.rsf")
	code, _, stderr = exec(t, source, "-o", path)
	return path, code, stderr
}

// decompiledYubka кладёт во временный файл документ, полученный из настоящей
// модели: он единственный из перечня несёт полную раскладку, а без координат
// компиляция отказывает по существу.
func decompiledYubka(t *testing.T) string {
	t.Helper()

	out := filepath.Join(t.TempDir(), "model.yaml")
	if code, _, stderr := exec(t, "decompile", example("ИзготовлениеЮбки.rsf"), "-o", out); code != exitOK {
		t.Fatalf("декомпиляция: код %d\n%s", code, stderr)
	}
	return out
}

// TestCompileWritesFile — главное: из документа получается файл.
func TestCompileWritesFile(t *testing.T) {
	out, code, stderr := compiled(t, decompiledYubka(t))

	if code != exitOK {
		t.Fatalf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("файл не создан: %v", err)
	}
	if info.Size() == 0 {
		t.Error("файл пуст")
	}
}

// TestCompileSaysNothingAboutArrows — предупреждения о ненаписанных связях
// больше нет (FR-013).
//
// Прежде компиляция честно сообщала, что стрелки в файл не попадают. Теперь
// попадают, и остаться предупреждению значило бы врать автору ровно наоборот.
func TestCompileSaysNothingAboutArrows(t *testing.T) {
	_, code, stderr := compiled(t, decompiledYubka(t))

	if code != exitOK {
		t.Fatalf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
	}
	if strings.Contains(stderr, "связи в .rsf пока не записываются") {
		t.Errorf("предупреждение о ненаписанных связях никуда не делось:\n%s", stderr)
	}
}

// TestCompileDocuments — весь набор документов компилируется молча.
//
// Ходит по тому же набору, что и раскладка: три примера поставки и две
// фикстуры, добавленные этой фичей. Предупреждений быть не должно ни одного —
// ни о связях, ни о координатах (SC-007).
func TestCompileDocuments(t *testing.T) {
	paths := []string{
		example("skirt.yaml"),
		example("skirt.json"),
		example("skirt-full.yaml"),
		document("feedback.yaml"),
		document("nested.yaml"),
		document("staircase.yaml"),
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			out, code, stderr := compiled(t, path)
			if code != exitOK {
				t.Fatalf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
			}
			if _, err := os.Stat(out); err != nil {
				t.Fatalf("файл не создан: %v", err)
			}
			for _, complaint := range []string{"пока не записываются", "нет координат", "warning"} {
				if strings.Contains(stderr, complaint) {
					t.Errorf("компиляция пожаловалась (%q):\n%s", complaint, stderr)
				}
			}
		})
	}
}

// TestCompileInvalidWritesNothing — при невалидном документе файла не остаётся,
// а диагностика та же, что у validate.
func TestCompileInvalidWritesNothing(t *testing.T) {
	out, code, _ := compiled(t, model("no-root.yaml"))

	if code != exitInvalid {
		t.Errorf("код возврата = %d, ожидался %d", code, exitInvalid)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("файл %s создан, хотя документ невалиден", out)
	}
}

// TestCompileDoesNotOverwriteOnError — существующий файл не портится.
func TestCompileDoesNotOverwriteOnError(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "old.rsf")
	const keep = "прежнее содержимое"
	if err := os.WriteFile(existing, []byte(keep), 0o644); err != nil {
		t.Fatal(err)
	}

	if code, _, _ := exec(t, model("no-root.yaml"), "-o", existing); code != exitInvalid {
		t.Fatalf("код возврата = %d, ожидался %d", code, exitInvalid)
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != keep {
		t.Errorf("существующий файл перезаписан: %q", got)
	}
}

// Здесь стояла проверка TestCompileRefusesWithoutLayout: документ без секции
// layout получал отказ, потому что автораскладки не было. Она появилась, и
// причины для отказа больше нет — теперь тот же документ компилируется, что и
// проверяет TestCompileWithoutLayout ниже.
//
// Сообщение «у работ нет координат» осталось в генераторе страховкой на случай
// его собственного дефекта: раскладка обязана заполнить всё, и если что-то
// осталось пустым, лучше отказ, чем тихая запись блока в точку (0, 0).

// TestCompileDefaultOutputName — без -o имя вывода получается из имени входа.
func TestCompileDefaultOutputName(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "model.yaml")

	document, err := os.ReadFile(decompiledYubka(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, document, 0o644); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := exec(t, source); code != exitOK {
		t.Fatalf("код возврата = %d\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "model.rsf")); err != nil {
		t.Errorf("файл model.rsf рядом с входным не создан: %v", err)
	}
}

// TestCompileWithoutLayout — документ без единой координаты компилируется.
//
// Ради этого раскладка и делалась: `skirt.yaml` — образцовый документ проекта,
// написанный словами, без геометрии, — до неё получал отказ с кодом 2.
func TestCompileWithoutLayout(t *testing.T) {
	out, code, stderr := compiled(t, example("skirt.yaml"))

	if code != exitOK {
		t.Fatalf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("файл не создан: %v", err)
	}
	if strings.Contains(stderr, "нет координат") {
		t.Errorf("отказ «нет координат» никуда не делся:\n%s", stderr)
	}
}

// TestLayoutDoesNotTouchGivenGeometry — документ с полной геометрией даёт тот
// же файл, что и до появления раскладки.
//
// Эталон в репозиторий не кладётся: .rsf бинарен, и держать его файлом значило
// бы обновлять при каждой правке формата. Сравниваются две сборки одного
// документа — этого достаточно, чтобы поймать вмешательство раскладки: она
// либо не трогает заданное, либо трогает, и тогда координаты «поплывут»
// относительно исходной модели, что видно по dump.
func TestLayoutDoesNotTouchGivenGeometry(t *testing.T) {
	document := decompiledYubka(t)

	first, code, stderr := compiled(t, document)
	if code != exitOK {
		t.Fatalf("код возврата = %d\n%s", code, stderr)
	}
	second, code, stderr := compiled(t, document)
	if code != exitOK {
		t.Fatalf("код возврата = %d\n%s", code, stderr)
	}

	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("две сборки одного документа разошлись: %d и %d байт", len(a), len(b))
	}

	// Координаты в собранном файле обязаны совпасть с теми, что стоят в
	// исходной модели: раскладке в документе с полной геометрией делать нечего.
	_, dumped, _ := exec(t, "dump", first)
	if !strings.Contains(dumped, "(288.0, 147.0, 186.0, 144.0)") {
		t.Errorf("геометрия корневой работы не та, что в исходной модели:\n%s", dumped)
	}
}

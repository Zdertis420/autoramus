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

// TestCompileWarnsAboutArrows — автор узнаёт, чего в файле пока нет.
//
// Связи не записываются, и файл поэтому заведомо неполон. Промолчать значило бы
// завести молчаливую потерю на выходном конце конвейера — ровно ту, которую
// только что убрали на входном.
func TestCompileWarnsAboutArrows(t *testing.T) {
	_, code, stderr := compiled(t, decompiledYubka(t))

	if code != exitOK {
		t.Fatalf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
	}
	if !strings.Contains(stderr, "связи в .rsf пока не записываются") {
		t.Errorf("автор не предупреждён о ненаписанных связях:\n%s", stderr)
	}
	// В модели четырнадцать потоков; число должно быть названо, иначе
	// предупреждение не отличить от общей отговорки.
	if !strings.Contains(stderr, "14") {
		t.Errorf("в предупреждении нет числа потоков:\n%s", stderr)
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

// TestCompileRefusesWithoutLayout — без координат файла не будет.
//
// Автораскладки пока нет. Выдать блоки, наложенные друг на друга, значило бы
// отдать результат, который выглядит как работа программы, а является мусором.
func TestCompileRefusesWithoutLayout(t *testing.T) {
	out, code, stderr := compiled(t, example("skirt.yaml"))

	if code != exitInternal {
		t.Errorf("код возврата = %d, ожидался %d\n%s", code, exitInternal, stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("файл %s создан, хотя координат нет", out)
	}
	if !strings.Contains(stderr, "координат") {
		t.Errorf("в сообщении не сказано о координатах:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Раскрой материала") {
		t.Errorf("в сообщении не названы работы без координат:\n%s", stderr)
	}
}

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

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
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

// Компиляция пока не доходит до генератора и обязана сказать об этом прямо,
// а не притвориться успехом.
func TestCompileNotImplemented(t *testing.T) {
	code, _, stderr := exec(t, example("skirt.json"))
	if code != exitInternal {
		t.Errorf("код возврата = %d, ожидался %d", code, exitInternal)
	}
	if !bytes.Contains([]byte(stderr), []byte("генератор")) {
		t.Errorf("в stderr нет объяснения: %q", stderr)
	}
}

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

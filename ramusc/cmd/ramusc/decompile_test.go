package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDecompileGolden держит канонический вывод неизменным: по Р14 он служит
// образцом для нейросети, и менять его стиль можно только осознанно.
// Эталоны обновляются `make golden`.
func TestDecompileGolden(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		golden string
	}{
		{"YAML", []string{"decompile", rsfExample("ИзготовлениеЮбки.rsf")}, "izgotovlenie-yubki.yaml"},
		{"JSON", []string{"decompile", rsfExample("ИзготовлениеЮбки.rsf"), "--json"}, "izgotovlenie-yubki.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := exec(t, tt.args...)
			if code != exitOK {
				t.Fatalf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
			}
			golden := filepath.Join("..", "..", "testdata", "golden", tt.golden)
			if *update {
				if err := os.WriteFile(golden, []byte(stdout), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("нет эталона (создайте его флагом -update): %v", err)
			}
			if strings.TrimSpace(string(want)) != strings.TrimSpace(stdout) {
				t.Errorf("вывод разошёлся с эталоном %s\n--- эталон ---\n%s\n--- получено ---\n%s",
					golden, want, stdout)
			}
		})
	}
}

// TestDecompileToFile — вывод в файл вместо стандартного потока.
func TestDecompileToFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "model.yaml")
	code, stdout, stderr := exec(t, "decompile", rsfExample("ИзготовлениеЮбки.rsf"), "-o", out)
	if code != exitOK {
		t.Fatalf("код возврата = %d\n%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("при записи в файл стандартный вывод должен молчать: %q", stdout)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "model: Изготовление юбки") {
		t.Errorf("в файле не та модель:\n%s", written)
	}
}

// TestDecompiledOutputValidates — сквозная проверка: то, что напечатал
// декомпилятор, обязано проходить validate.
func TestDecompiledOutputValidates(t *testing.T) {
	out := filepath.Join(t.TempDir(), "model.yaml")
	if code, _, stderr := exec(t, "decompile", rsfExample("ИзготовлениеЮбки.rsf"), "-o", out); code != exitOK {
		t.Fatalf("декомпиляция: код %d\n%s", code, stderr)
	}
	code, stdout, stderr := exec(t, "validate", out)
	if code != exitOK {
		t.Fatalf("проверка: код %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "ошибок нет") {
		t.Errorf("вывод проверки: %q", stdout)
	}
	if !strings.Contains(stdout, "предупреждений — 3") {
		t.Errorf("ожидались три предупреждения об неиспользуемых потоках: %q", stdout)
	}
}

func TestDecompileExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"не ZIP", []string{"decompile", example("skirt.yaml")}, exitInvalid},
		{"файла нет", []string{"decompile", rsfExample("такого-нет.rsf")}, exitInternal},
		{"без файла", []string{"decompile"}, exitInternal},
		{"неизвестный флаг", []string{"decompile", rsfExample("ИзготовлениеЮбки.rsf"), "--что-то"}, exitInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, _ := exec(t, tt.args...); code != tt.want {
				t.Errorf("код возврата = %d, ожидался %d", code, tt.want)
			}
		})
	}
}

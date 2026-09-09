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

// testModel — вторая проверочная модель. Лежит в testdata, а не в examples:
// она собрана ради проверок, а не как пример языка.
func testModelPath() string {
	return filepath.Join("..", "..", "testdata", "testModel.rsf")
}

// TestDecompileRefusesUnsupported — главная проверка истории 1. Файл содержит
// работы без имени, языком они не выражаются, и печатать что-либо нельзя:
// признак успеха на неполном результате и есть тот дефект, ради которого всё
// затевалось.
func TestDecompileRefusesUnsupported(t *testing.T) {
	code, stdout, stderr := exec(t, "decompile", testModelPath())

	if code != exitInternal {
		t.Errorf("код возврата = %d, ожидался %d", code, exitInternal)
	}
	if stdout != "" {
		t.Errorf("в поток вывода попало %d байт, ожидалась пустота:\n%s", len(stdout), stdout)
	}
	if !strings.Contains(stderr, "работ без имени") {
		t.Errorf("в сообщении не назван вид находки:\n%s", stderr)
	}
	if !strings.Contains(stderr, "ничего не записано") {
		t.Errorf("автор не предупреждён, что результата нет вовсе:\n%s", stderr)
	}
}

// TestRefusalWritesNothing — при отказе файл вывода не создаётся и уже
// существующий не портится. Иначе в пайплайне остался бы огрызок, а о коде
// возврата легко забыть.
func TestRefusalWritesNothing(t *testing.T) {
	dir := t.TempDir()

	fresh := filepath.Join(dir, "new.yaml")
	if code, _, _ := exec(t, "decompile", testModelPath(), "-o", fresh); code != exitInternal {
		t.Fatalf("код возврата = %d, ожидался %d", code, exitInternal)
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Errorf("файл %s создан, хотя декомпиляция отказала", fresh)
	}

	existing := filepath.Join(dir, "old.yaml")
	const keep = "прежнее содержимое"
	if err := os.WriteFile(existing, []byte(keep), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := exec(t, "decompile", testModelPath(), "-o", existing); code != exitInternal {
		t.Fatalf("код возврата = %d, ожидался %d", code, exitInternal)
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != keep {
		t.Errorf("существующий файл перезаписан: %q", got)
	}
}

// TestLossWarningDoesNotBlock — потеря не отказ: вывод печатается целиком,
// перечень идёт в поток ошибок и в шапку, код возврата нулевой. С автором,
// у которого есть перенесённая модель без части оформления, работать можно;
// блокировать его нечего.
func TestLossWarningDoesNotBlock(t *testing.T) {
	code, stdout, stderr := exec(t, "decompile", example("ФормированиеТП.rsf"))

	if code != exitOK {
		t.Errorf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
	}
	if stdout == "" {
		t.Error("вывод пуст, а модель перенесена")
	}
	if !strings.Contains(stderr, "перенесено не всё") {
		t.Errorf("в потоке ошибок нет перечня потерь:\n%s", stderr)
	}
	if !strings.Contains(stdout, "# Перенесено не всё") {
		t.Errorf("в шапке документа нет перечня потерь")
	}
}

// TestLossWarningNotInJSON — комментариев в JSON нет (Р10), поэтому там
// перечень остаётся только в потоке ошибок.
func TestLossWarningNotInJSON(t *testing.T) {
	_, stdout, stderr := exec(t, "decompile", example("ФормированиеТП.rsf"), "--json")

	if strings.Contains(stdout, "Перенесено не всё") {
		t.Error("перечень потерь попал в JSON, где комментариев не бывает")
	}
	if !strings.Contains(stderr, "перенесено не всё") {
		t.Errorf("в потоке ошибок нет перечня потерь:\n%s", stderr)
	}
}

// TestRefusalIsNotAWarning — два канала нельзя путать. Файл с работами без
// имени языком не описывается по существу и получает отказ, а не мягкое
// предупреждение: цена ошибки — молчаливая потеря обратно.
func TestRefusalIsNotAWarning(t *testing.T) {
	code, stdout, stderr := exec(t, "decompile", testModelPath())

	if code != exitInternal {
		t.Errorf("код возврата = %d, ожидался отказ %d", code, exitInternal)
	}
	if stdout != "" {
		t.Errorf("при отказе напечатано %d байт вывода", len(stdout))
	}
	if strings.Contains(stderr, "перенесено не всё") {
		t.Error("отказ выдан как предупреждение о потере")
	}
}

package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "переписать эталон вывода dump")

func rsfExample(name string) string { return filepath.Join("..", "..", "..", "examples", name) }

// TestDumpGolden держит вывод dump неизменным. Первые три раздела повторяют
// algodemo/rsf.py слово в слово: пока декомпилятора нет, эталоном служит он,
// и расхождение должно быть заметно сразу.
//
// Эталон обновляется `go test ./cmd/ramusc -update`.
func TestDumpGolden(t *testing.T) {
	code, stdout, stderr := exec(t, "dump", rsfExample("ИзготовлениеЮбки.rsf"))
	if code != exitOK {
		t.Fatalf("код возврата = %d, ожидался %d\n%s", code, exitOK, stderr)
	}

	golden := filepath.Join("..", "..", "testdata", "golden", "izgotovlenie-yubki.dump.txt")
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
		t.Errorf("вывод dump разошёлся с эталоном %s\n--- эталон ---\n%s\n--- получено ---\n%s",
			golden, want, stdout)
	}
}

// TestDumpExitCodes — повреждённый файл это невалидный вход (1), а не поломка
// компилятора (2).
func TestDumpExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"модель Ramus", []string{"dump", rsfExample("ИзготовлениеЮбки.rsf")}, exitOK},
		{"не ZIP", []string{"dump", example("skirt.yaml")}, exitInvalid},
		{"файла нет", []string{"dump", rsfExample("такого-нет.rsf")}, exitInternal},
		{"без файла", []string{"dump"}, exitInternal},
		{"два файла", []string{"dump", rsfExample("ИзготовлениеЮбки.rsf"), rsfExample("ИзготовлениеЮбки.rsf")}, exitInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, _ := exec(t, tt.args...); code != tt.want {
				t.Errorf("код возврата = %d, ожидался %d", code, tt.want)
			}
		})
	}
}

package diag_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// Вид диагностики, который не возникает ни на одном проверочном входе, ничем
// не подтверждён: его текст, код и позиция могут быть сломаны, а сборка
// останется зелёной. GUI ветвится по коду, поэтому цена такой поломки — не
// кривое сообщение, а неверная реакция редактора.
//
// Список видов читается разбором объявлений: перечислить константы во время
// выполнения Go не позволяет, а список-дубликат рядом с объявлениями разойдётся
// с ними при первом же добавлении.

// TestEveryCodeIsExercised — каждый объявленный вид возникает хотя бы на одной
// модели из testdata/models.
func TestEveryCodeIsExercised(t *testing.T) {
	declared, unused := declaredCodes(t)
	if len(declared) == 0 {
		t.Fatal("в code.go не нашлось ни одного вида диагностики")
	}

	seen := emittedCodes(t)

	var missing []string
	for _, code := range declared {
		if seen[diag.Code(code)] || unused[code] {
			continue
		}
		missing = append(missing, code)
	}
	if len(missing) > 0 {
		t.Errorf("видов диагностики без единой проверки: %d из %d\n  %v\n"+
			"добавьте модель в testdata/models, на которой вид возникает,\n"+
			"либо удалите вид из code.go, если он недостижим (FR-008)",
			len(missing), len(declared), missing)
	}
}

// TestNoUnknownCodesEmitted — обратная проверка: валидатор не выдаёт кодов,
// которых нет в закрытом списке. Иначе GUI получит код, о котором не знает.
func TestNoUnknownCodesEmitted(t *testing.T) {
	all, _ := declaredCodes(t)
	declared := make(map[string]bool, len(all))
	for _, code := range all {
		declared[code] = true
	}

	for code := range emittedCodes(t) {
		if !declared[string(code)] {
			t.Errorf("выдан код %q, которого нет в code.go", code)
		}
	}
}

// unusedMarker — пометка в комментарии к константе: вид объявлен, но схема
// соответствующего ключевого слова не содержит, и возникнуть он не может.
// Пометка живёт рядом с объявлением, поэтому не разъедется с ним: тот, кто
// добавит ключевое слово в схему, снимет её там же.
const unusedMarker = "Схемой не используется"

// declaredCodes читает значения констант типа Code из code.go и отдельно —
// те из них, что помечены как недостижимые.
func declaredCodes(t *testing.T) (all []string, unused map[string]bool) {
	t.Helper()

	unused = make(map[string]bool)
	file, err := parser.ParseFile(token.NewFileSet(), "code.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			// Интересны только константы, объявленные как Code: остальные
			// константы пакета видами диагностики не являются.
			if ident, ok := value.Type.(*ast.Ident); !ok || ident.Name != "Code" {
				continue
			}
			for _, v := range value.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("значение %s не разбирается: %v", lit.Value, err)
				}
				all = append(all, s)
				if value.Doc != nil && strings.Contains(value.Doc.Text(), unusedMarker) {
					unused[s] = true
				}
			}
		}
	}
	sort.Strings(all)
	return all, unused
}

// TestUnusedMarkersAreHonest — пометка «схемой не используется» не должна
// пережить своё основание: если вид всё-таки возник, пометку пора снять,
// а модель — оставить проверкой.
func TestUnusedMarkersAreHonest(t *testing.T) {
	_, unused := declaredCodes(t)
	seen := emittedCodes(t)

	for code := range unused {
		if seen[diag.Code(code)] {
			t.Errorf("вид %q помечен недостижимым, но возник: снимите пометку в code.go", code)
		}
	}
}

// emittedCodes прогоняет все проверочные модели и собирает виды, которые
// валидатор на них выдал.
func emittedCodes(t *testing.T) map[diag.Code]bool {
	t.Helper()

	dir := filepath.Join("..", "..", "testdata", "models")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	seen := make(map[diag.Code]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		diags, err := validate.Source(data)
		if err != nil {
			// Внутренняя ошибка на проверочной модели — сама по себе дефект,
			// но здесь важно не потерять остальные модели.
			t.Errorf("%s: внутренняя ошибка: %v", entry.Name(), err)
			continue
		}
		for _, d := range diags {
			seen[d.Code] = true
		}
	}
	return seen
}

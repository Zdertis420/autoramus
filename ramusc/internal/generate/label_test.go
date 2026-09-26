package generate_test

import (
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
)

// TestLayoutsBreakBetweenWords — имя разбивается на строки только между
// словами, и каждая следующая раскладка — на строку длиннее (FR-006).
func TestLayoutsBreakBetweenWords(t *testing.T) {
	for _, name := range []string{
		"Помидоры",
		"Растительное масло",
		"Хмели-сунели",
		"Рецепт приготовления чахохбили",
		"Швеёно-вышивальгая машинка",
		"a b c d e",
	} {
		t.Run(name, func(t *testing.T) {
			words := wordsOf(name)
			layouts := generate.Layouts(name)
			if len(layouts) == 0 {
				t.Fatal("ни одной раскладки")
			}
			if len(layouts) > len(words) {
				t.Errorf("раскладок %d, а слов %d", len(layouts), len(words))
			}
			if got := layouts[0].Lines; len(got) != 1 || got[0] != name {
				t.Errorf("первая раскладка %q, ожидалась одна строка во всё имя", got)
			}

			longest := 0.0
			for _, w := range words {
				longest = max(longest, generate.TextWidth(w))
			}
			for i, l := range layouts {
				if len(l.Lines) != i+1 {
					t.Errorf("раскладка %d: строк %d, ожидалось %d", i, len(l.Lines), i+1)
				}
				// Строки, склеенные обратно, — те же слова в том же порядке:
				// ни одна не начинается и не кончается внутри слова.
				var back []string
				widest := 0.0
				for _, line := range l.Lines {
					back = append(back, wordsOf(line)...)
					widest = max(widest, generate.TextWidth(line))
				}
				if strings.Join(back, "|") != strings.Join(words, "|") {
					t.Errorf("раскладка %q режет слово: слова %q", l.Lines, back)
				}
				if l.Width != widest {
					t.Errorf("раскладка %q: ширина %v, самая широкая строка %v", l.Lines, l.Width, widest)
				}
				if l.Width < longest {
					t.Errorf("раскладка %q уже самого длинного слова: %v < %v", l.Lines, l.Width, longest)
				}
				if want := float64(len(l.Lines)) * lineHeight; l.Height != want {
					t.Errorf("раскладка %q: высота %v, ожидалось %v", l.Lines, l.Height, want)
				}
			}
		})
	}

	// Дефис остаётся с предшествующей частью, как у Ramus.
	if got := generate.Layouts("Хмели-сунели"); len(got) < 2 || got[1].Lines[0] != "Хмели-" {
		t.Errorf("«Хмели-сунели» в две строки: %v, ожидалось «Хмели-» / «сунели»", got)
	}
}

// TestLabelNamedBoundary — места нет, и это видно (FR-012).
//
// В labels-crowded.yaml семи длинным подписям у последнего блока лестницы
// физически негде встать. Компилятор не падает и не уносит их прочь: каждая
// остаётся в пределах 34 единиц от своей линии и внутри листа, а наложение —
// названная граница, которую TestLabelsDoNotCollide печатает поимённо.
func TestLabelNamedBoundary(t *testing.T) {
	path := documentPath("labels-crowded.yaml")
	file, m := buildFrom(t, path)
	labels := shownLabels(t, file, m)

	collisions := 0
	for i, a := range labels {
		if d := distanceToOwnLine(m, a); d > labelReach+1e-9 {
			t.Errorf("«%s» унесена на %.1f от своей линии", a.flow, d)
		}
		if a.x < 7 || a.y < 7 || a.x+a.w > 793 || a.y+a.h > 437 {
			t.Errorf("«%s» за листом", a.flow)
		}
		for _, b := range labels[i+1:] {
			if a.diagram == b.diagram && overlaps(a.x, a.y, a.w, a.h, b.x, b.y, b.w, b.h) {
				collisions++
			}
		}
		for _, f := range m.Functions() {
			if f.Parent == a.diagram && f.Bounds != nil &&
				overlaps(a.x, a.y, a.w, a.h, f.Bounds.X, f.Bounds.Y, f.Bounds.Width, f.Bounds.Height) {
				collisions++
			}
		}
	}
	// Если наложений нет, документ перестал давить на границу — и проверяет
	// уже не её. Тогда его надо сделать теснее, а не радоваться.
	if collisions == 0 {
		t.Error("в labels-crowded.yaml не осталось ни одного наложения: документ больше не проверяет названную границу")
	}
	t.Logf("наложений на названной границе: %d", collisions)
}

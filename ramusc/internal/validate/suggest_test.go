package validate

import "testing"

func TestNearest(t *testing.T) {
	flows := []string{"Ткань", "Выкройка", "Юбка", "Швейная машинка"}

	tests := []struct {
		name       string
		input      string
		candidates []string
		want       string
		wantOK     bool
	}{
		{"опечатка в окончании", "Выкройки", flows, "Выкройка", true},
		{"другой регистр", "выкройка", flows, "Выкройка", true},
		{"короткое имя, одна правка", "Юбк", flows, "Юбка", true},
		{"две правки в длинном имени", "Швейная машина", flows, "Швейная машинка", true},
		{"ничего похожего", "Фурнитура", flows, "", false},
		{"нет кандидатов", "Ткань", nil, "", false},
		{"пустое имя", "", flows, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nearest(tt.input, tt.candidates)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("nearest(%q) = %q, %v; ожидалось %q, %v",
					tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestNearestIsStable — при равном расстоянии побеждает первый по документу,
// иначе подсказка плавала бы от прогона к прогону.
func TestNearestIsStable(t *testing.T) {
	candidates := []string{"Юбка", "Юбки"}
	for i := 0; i < 10; i++ {
		if got, _ := nearest("Юбкa", candidates); got != "Юбка" {
			t.Fatalf("подсказка = %q, ожидалась «Юбка»", got)
		}
	}
}

func TestDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"Ткань", "Ткань", 0},
		{"Ткань", "Ткнаь", 1}, // перестановка соседних — одна правка
		{"Выкройка", "Выкройки", 1},
		{"абв", "абг", 1}, // считаем по рунам, а не по байтам
		{"", "Юбка", 4},
		{"Юбка", "", 4},
	}
	for _, tt := range tests {
		if got := distance([]rune(tt.a), []rune(tt.b)); got != tt.want {
			t.Errorf("distance(%q, %q) = %d, ожидалось %d", tt.a, tt.b, got, tt.want)
		}
	}
}

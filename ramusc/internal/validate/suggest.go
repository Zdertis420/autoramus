package validate

import (
	"fmt"
	"strings"
)

// hint даёт хвост сообщения с подсказкой ближайшего имени или пустую строку.
// Ради этой подсказки Р13 и затевался: сообщение «поток «Выкройки» не объявлен,
// похоже на «Выкройка»» позволяет крутить цикл генерация → проверка →
// исправление без человека.
func hint(name string, candidates []string) string {
	near, ok := nearest(name, candidates)
	if !ok {
		return ""
	}
	return fmt.Sprintf("; похоже на «%s»", near)
}

// nearest ищет ближайшего кандидата. Совпадение с точностью до регистра
// побеждает сразу: регистр компилятор не нормализует (Р4), так что это самая
// частая опечатка. Дальше — минимальное расстояние в пределах порога.
//
// При равном расстоянии выигрывает первый по документу: иначе два прогона на
// одном входе давали бы разные подсказки.
func nearest(name string, candidates []string) (string, bool) {
	if name == "" {
		return "", false
	}
	target := []rune(strings.ToLower(name))

	// Порог: у коротких имён две правки уже меняют слово целиком.
	limit := 2
	if len(target) < 5 {
		limit = 1
	}

	best, bestDist := "", limit+1
	for _, cand := range candidates {
		if cand == name {
			continue
		}
		if strings.EqualFold(cand, name) {
			return cand, true
		}
		if d := distance(target, []rune(strings.ToLower(cand))); d < bestDist {
			best, bestDist = cand, d
		}
	}
	return best, best != ""
}

// distance — расстояние Дамерау—Левенштейна в варианте оптимального
// выравнивания: вставка, удаление, замена и перестановка соседних символов.
// Считается по рунам: имена в моделях кириллические, по байтам вышло бы вдвое
// больше правок на ровном месте.
func distance(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev2 := make([]int, len(b)+1) // строка i-2
	prev := make([]int, len(b)+1)  // строка i-1
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d := min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d = min(d, prev2[j-2]+1)
			}
			cur[j] = d
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(b)]
}

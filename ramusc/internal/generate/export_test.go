package generate

// Неэкспортированное — внешним тестам пакета (generate_test). Файл
// компилируется только в тестах.
var TextWidth = textWidth

// Layout — раскладка подписи для внешних тестов.
type Layout struct {
	Lines         []string
	Width, Height float64
}

func Layouts(name string) []Layout {
	var out []Layout
	for _, l := range layouts(name) {
		out = append(out, Layout{Lines: l.lines, Width: l.width, Height: l.height})
	}
	return out
}

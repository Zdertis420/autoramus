// Пакет diag описывает диагностику компилятора. Ошибка — это данные, а не текст:
// GUI разбирает вывод `--json` по полям, поэтому сообщение собирается из структуры,
// а разбирать его обратно регулярками не требуется.
package diag

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Severity — уровень диагностики.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Pos — позиция в исходном документе. Нумерация с единицы; нулевая позиция
// означает «позиции нет» (например, ошибка относится к документу целиком).
// Колонка считается в рунах, а не в байтах: имена в моделях кириллические,
// иначе маркеры в редакторе GUI уедут вправо.
type Pos struct {
	Line   int
	Column int
}

// Valid сообщает, известна ли позиция.
func (p Pos) Valid() bool { return p.Line > 0 }

// Diagnostic — одна ошибка или предупреждение. Форма полей зафиксирована
// в CLAUDE.md и является контрактом с GUI.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     Code     `json:"code"`
	Message  string   `json:"message"`
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
}

// New собирает диагностику уровня Error.
func New(code Code, pos Pos, path, format string, args ...any) Diagnostic {
	return Diagnostic{
		Severity: Error,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Path:     path,
		Line:     pos.Line,
		Column:   pos.Column,
	}
}

// List — набор диагностик одного запуска.
type List []Diagnostic

// Add дописывает диагностику в список.
func (l *List) Add(d Diagnostic) { *l = append(*l, d) }

// HasErrors сообщает, есть ли в списке хоть одна ошибка (предупреждения не в счёт).
func (l List) HasErrors() bool {
	for _, d := range l {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// Sort приводит список к детерминированному порядку: сверху вниз по документу,
// а при совпадении позиции — по пути, коду и тексту. Без этого два прогона
// на одном входе могут выдать разный порядок сообщений.
func (l List) Sort() {
	slices.SortStableFunc(l, func(a, b Diagnostic) int {
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		if a.Column != b.Column {
			return a.Column - b.Column
		}
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		if c := strings.Compare(string(a.Code), string(b.Code)); c != 0 {
			return c
		}
		return strings.Compare(a.Message, b.Message)
	})
}

// WriteJSON печатает машинный вывод для GUI: всегда массив, даже пустой.
func (l List) WriteJSON(w io.Writer) error {
	out := l
	if out == nil {
		out = List{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// WriteText печатает человекочитаемый вывод в формате, привычном компиляторам:
// `файл:строка:колонка: error: текст [код]`.
func (l List) WriteText(w io.Writer, filename string) error {
	for _, d := range l {
		loc := filename
		if d.Line > 0 {
			loc = fmt.Sprintf("%s:%d:%d", filename, d.Line, d.Column)
		}
		if _, err := fmt.Fprintf(w, "%s: %s: %s [%s]\n", loc, d.Severity, d.Message, d.Code); err != nil {
			return err
		}
	}
	return nil
}

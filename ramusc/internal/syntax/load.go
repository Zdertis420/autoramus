package syntax

import (
	"bytes"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

// bom — метка порядка байтов UTF-8, которую любят дописывать редакторы Windows.
var bom = []byte{0xEF, 0xBB, 0xBF}

// Load выбирает фронтенд по первому непробельному символу документа (Р10):
// `{` или `[` — JSON, иначе YAML. Разбирать JSON через YAML нельзя: в JSON
// законны и табы-разделители, и повторяющиеся ключи, а YAML на них спотыкается.
func Load(src []byte) (*Node, diag.List) {
	src = blankBOM(src)
	trimmed := bytes.TrimLeft(src, " \t\r\n")
	if len(trimmed) == 0 {
		li := newLineIndex(src)
		return nil, diag.List{diag.New(diag.CodeEmptyDocument, li.pos(len(src)), "", "документ пуст")}
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return ParseJSON(src)
	}
	return ParseYAML(src)
}

// blankBOM заменяет метку порядка байтов пробелами, а не вырезает её:
// длина документа сохраняется, и все позиции остаются верными.
func blankBOM(src []byte) []byte {
	if !bytes.HasPrefix(src, bom) {
		return src
	}
	out := bytes.Clone(src)
	copy(out, "   ")
	return out
}

package syntax

import (
	"sort"
	"unicode/utf8"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

// lineIndex переводит байтовое смещение в позицию «строка:колонка».
// Колонка считается в рунах: документы кириллические, и байтовая колонка
// поставила бы маркер редактора не туда.
type lineIndex struct {
	src    []byte
	starts []int // байтовое смещение начала каждой строки
}

func newLineIndex(src []byte) *lineIndex {
	starts := make([]int, 1, 1+len(src)/32)
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &lineIndex{src: src, starts: starts}
}

// pos переводит смещение в позицию. Смещение за пределами документа
// прижимается к его концу.
func (li *lineIndex) pos(offset int) diag.Pos {
	if offset < 0 {
		offset = 0
	}
	if offset > len(li.src) {
		offset = len(li.src)
	}
	line := sort.SearchInts(li.starts, offset+1) - 1
	if line < 0 {
		line = 0
	}
	return diag.Pos{
		Line:   line + 1,
		Column: 1 + utf8.RuneCount(li.src[li.starts[line]:offset]),
	}
}

// posFromByteColumn строит позицию по номеру строки и колонке в байтах —
// на случай, если сторонний разбор считает колонки байтами. Обе величины
// с единицы.
func (li *lineIndex) posFromByteColumn(line, byteColumn int) diag.Pos {
	if line < 1 || line > len(li.starts) {
		return diag.Pos{Line: line, Column: byteColumn}
	}
	start := li.starts[line-1]
	offset := start + byteColumn - 1
	if offset > len(li.src) {
		offset = len(li.src)
	}
	return li.pos(offset)
}

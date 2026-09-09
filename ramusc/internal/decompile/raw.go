package decompile

import (
	"encoding/json"
	"sort"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Люк: атрибуты, которым нет соответствия в полях языка, доезжают до документа
// как есть (Р9, Р17). Без него они терялись молча — цвета, статусы, свойства
// подписи стрелок.
//
// Что складывать в люк, решает не этот файл: перечень потерь считает
// `internal/rsf` вычитанием описи из перенесённого. Здесь только перевод
// найденного в записи документа.

// rawOf собирает люк одного элемента: все его атрибуты, которые не переносятся
// полями языка.
func rawOf(m *rsf.Model, element int64) []ir.RawAttr {
	var out []ir.RawAttr
	for _, a := range m.Attributes(element) {
		lost := rsf.LostColumns(a)
		if len(lost) == 0 {
			continue // атрибут переносится своим полем — в люк ему не место
		}
		out = append(out, ir.RawAttr{
			Attribute: ir.Ref{Name: ir.Normalize(a.Name), Raw: a.Name},
			Value:     rawValue(a, lost),
		})
	}

	// Порядок обхода описи детерминирован, но люк собирается из нескольких
	// источников, поэтому порядок задаётся ещё раз и явно.
	sort.Slice(out, func(i, j int) bool { return out[i].Attribute.Raw < out[j].Attribute.Raw })
	return out
}

// rawValue приводит значение к тому виду, которого ждёт документ: скаляр для
// одноколоночного атрибута, отображение «колонка → значение» для
// многоколоночного.
//
// Сплющивать многоколоночные нельзя: генератору предстоит разложить значение
// обратно по колонкам, и имена колонок ему для этого нужны.
func rawValue(a rsf.Attribute, lost []string) any {
	if len(lost) == 1 {
		return scalar(a.Columns[lost[0]])
	}
	out := make(map[string]any, len(lost))
	for _, name := range lost {
		out[name] = scalar(a.Columns[name])
	}
	return out
}

// scalar приводит строку из файла к числу там, где она числом и является:
// значение люка не проверяется, но печатать 42 строкой значило бы соврать о том,
// что лежит в файле.
func scalar(s string) any {
	if s == "" {
		return ""
	}
	if n := json.Number(s); looksNumeric(s) {
		return n
	}
	return s
}

func looksNumeric(s string) bool {
	dot := false
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '-' && i == 0:
		case r == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return s != "-" && s != "."
}

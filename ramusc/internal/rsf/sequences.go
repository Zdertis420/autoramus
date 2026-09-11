package rsf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
)

// SequencesPath — запись ZIP со значениями последовательностей.
const SequencesPath = "data/sequences.xml"

// Имена последовательностей, которые заводит плагин IDEF0.
//
// Прочие (elements_sequence, qualifiers_sequence, attributes_sequence) в файле
// не сохраняются: Ramus создаёт их заново и при коллизии перематывает до
// MAX(ID)+1. Эти две он не перематывает, и увеличивать их обязан тот, кто
// выдал номера (RSF-FORMAT.md §1).
const (
	SequenceOrdinates   = "ordinates__sequence"
	SequenceCrosspoints = "crosspoint_sequence"
)

// properties — форма java.util.Properties в XML-виде.
type properties struct {
	XMLName xml.Name        `xml:"properties"`
	Entries []propertyEntry `xml:"entry"`
}

type propertyEntry struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// Sequences читает значения последовательностей.
//
// Записи, значение которых числом не является, пропускаются: последовательность
// — это счётчик, и нечисловое значение означает, что файл говорит не о том, о
// чём мы думаем.
func (f *File) Sequences() (map[string]int64, error) {
	data, ok := f.Raw[SequencesPath]
	if !ok {
		return nil, fmt.Errorf("в файле нет записи %s", SequencesPath)
	}

	var doc properties
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s не разбирается: %w", SequencesPath, err)
	}

	out := make(map[string]int64, len(doc.Entries))
	for _, e := range doc.Entries {
		n, err := strconv.ParseInt(e.Value, 10, 64)
		if err != nil {
			continue
		}
		out[e.Key] = n
	}
	return out, nil
}

// SetSequence переписывает одно значение.
//
// Правка точечная, по байтам, а не через повторную сборку XML: sequences.xml —
// вывод java.util.Properties.storeToXML со своим DOCTYPE, комментарием и
// переводами строк, и собрать его заново побайтово тем же нечем. Всё, кроме
// самого числа, остаётся нетронутым, и файл без правок совпадает с исходным.
func (f *File) SetSequence(name string, value int64) error {
	data, ok := f.Raw[SequencesPath]
	if !ok {
		return fmt.Errorf("в файле нет записи %s", SequencesPath)
	}

	open := []byte(`<entry key="` + name + `">`)
	start := bytes.Index(data, open)
	if start < 0 {
		return fmt.Errorf("в %s нет последовательности %s", SequencesPath, name)
	}
	start += len(open)

	length := bytes.Index(data[start:], []byte("</entry>"))
	if length < 0 {
		return fmt.Errorf("в %s запись %s не закрыта", SequencesPath, name)
	}

	out := make([]byte, 0, len(data))
	out = append(out, data[:start]...)
	out = append(out, strconv.FormatInt(value, 10)...)
	out = append(out, data[start+length:]...)
	f.Raw[SequencesPath] = out
	return nil
}

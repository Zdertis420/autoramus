package rsf_test

import (
	"bytes"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Последовательности ординат и кросспоинтов Ramus сам не перематывает, поэтому
// их правит генератор. Проверяется здесь ровно одно: правка точечная и ничего,
// кроме числа, не задевает.

// TestSequencesRead — значения читаются из настоящего файла.
func TestSequencesRead(t *testing.T) {
	file, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := file.Sequences()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{rsf.SequenceOrdinates, rsf.SequenceCrosspoints} {
		if _, ok := seq[name]; !ok {
			t.Errorf("в файле не нашлось последовательности %s", name)
		}
	}
	if seq[rsf.SequenceOrdinates] <= 0 {
		t.Errorf("%s = %d, ожидалось положительное", rsf.SequenceOrdinates, seq[rsf.SequenceOrdinates])
	}
}

// TestSequencesUntouched — файл, который не правили, собирается побайтово тем
// же. Без этого точечная правка ничем не отличалась бы от пересборки XML.
func TestSequencesUntouched(t *testing.T) {
	file, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), file.Raw[rsf.SequencesPath]...)

	if _, err := file.Sequences(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, file.Raw[rsf.SequencesPath]) {
		t.Error("чтение последовательностей изменило запись файла")
	}
}

// TestSetSequence — меняется только число.
//
// Сравнение идёт по остатку: из записи вырезается значение, и всё остальное —
// объявление XML, DOCTYPE, комментарий, вторая запись, переводы строк —
// обязано совпасть с исходным. Так ловится и лишний перевод строки, и
// переставленные записи.
func TestSetSequence(t *testing.T) {
	file, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), file.Raw[rsf.SequencesPath]...)

	seq, err := file.Sequences()
	if err != nil {
		t.Fatal(err)
	}
	want := seq[rsf.SequenceOrdinates] + 17

	if err := file.SetSequence(rsf.SequenceOrdinates, want); err != nil {
		t.Fatal(err)
	}
	after := file.Raw[rsf.SequencesPath]

	seq, err = file.Sequences()
	if err != nil {
		t.Fatal(err)
	}
	if seq[rsf.SequenceOrdinates] != want {
		t.Errorf("%s = %d, ожидалось %d", rsf.SequenceOrdinates, seq[rsf.SequenceOrdinates], want)
	}

	if strip(before, rsf.SequenceOrdinates) != strip(after, rsf.SequenceOrdinates) {
		t.Errorf("правка задела не только число:\n--- было ---\n%s\n--- стало ---\n%s", before, after)
	}
}

// TestSetSequenceKeepsNeighbour — правка одной последовательности не трогает
// соседнюю. Их две, и раздаются они независимо.
func TestSetSequenceKeepsNeighbour(t *testing.T) {
	file, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := file.Sequences()
	if err != nil {
		t.Fatal(err)
	}
	neighbour := seq[rsf.SequenceCrosspoints]

	if err := file.SetSequence(rsf.SequenceOrdinates, 1000); err != nil {
		t.Fatal(err)
	}
	seq, err = file.Sequences()
	if err != nil {
		t.Fatal(err)
	}
	if seq[rsf.SequenceCrosspoints] != neighbour {
		t.Errorf("%s = %d, а было %d", rsf.SequenceCrosspoints, seq[rsf.SequenceCrosspoints], neighbour)
	}
}

// TestSetSequenceUnknown — неизвестное имя не создаёт записи молча: счётчик,
// которого в файле нет, заводит плагин, а не мы.
func TestSetSequenceUnknown(t *testing.T) {
	file, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.SetSequence("не_последовательность", 5); err == nil {
		t.Error("правка несуществующей последовательности прошла молча")
	}
}

// strip вырезает значение названной записи, оставляя всё прочее.
func strip(data []byte, name string) string {
	open := []byte(`<entry key="` + name + `">`)
	start := bytes.Index(data, open)
	if start < 0 {
		return string(data)
	}
	start += len(open)
	length := bytes.Index(data[start:], []byte("</entry>"))
	if length < 0 {
		return string(data)
	}
	return string(data[:start]) + string(data[start+length:])
}

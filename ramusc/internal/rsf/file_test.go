package rsf_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// fixture — настоящая модель Ramus. Отдельной копии в testdata нет намеренно:
// проверять порт надо ровно на том файле, по которому писались примеры.
const fixture = "../../../examples/ИзготовлениеЮбки.rsf"

type entry struct {
	name string
	data []byte
}

func readZip(t *testing.T, data []byte) []entry {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out []entry
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		payload, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, entry{name: f.Name, data: payload})
	}
	return out
}

// TestRoundTripIsByteExact — критерий приёмки порта: прочитать настоящий файл,
// записать обратно и получить те же самые байты в каждой записи. Пока это
// держится, слой таблиц ничего не теряет и не додумывает.
func TestRoundTripIsByteExact(t *testing.T) {
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	want := readZip(t, source)

	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := f.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	got := readZip(t, rebuilt)

	if len(got) != len(want) {
		t.Fatalf("записей %d, в оригинале %d", len(got), len(want))
	}
	for i := range want {
		if got[i].name != want[i].name {
			t.Fatalf("запись %d: %s, в оригинале %s", i, got[i].name, want[i].name)
		}
		if !bytes.Equal(got[i].data, want[i].data) {
			t.Errorf("%s: расхождение (%d байт против %d)\n%s",
				want[i].name, len(got[i].data), len(want[i].data),
				firstDiff(want[i].data, got[i].data))
		}
	}
}

// firstDiff показывает окрестность первого расхождения: диффать XML целиком
// в одну строку бесполезно.
func firstDiff(want, got []byte) string {
	n := min(len(want), len(got))
	for i := range n {
		if want[i] != got[i] {
			from := max(0, i-70)
			return "  оригинал: …" + string(want[from:min(i+70, len(want))]) +
				"\n  получено:  …" + string(got[from:min(i+70, len(got))])
		}
	}
	return "  одно является префиксом другого"
}

func TestTableLookup(t *testing.T) {
	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"elements",
		"attribute_rectangles",
		"data/IDEF0/attribute_rectangles.xml",
	} {
		if _, err := f.Table(name); err != nil {
			t.Errorf("таблица %s: %v", name, err)
		}
	}
	if _, err := f.Table("такой-таблицы-нет"); err == nil {
		t.Error("несуществующая таблица должна давать ошибку")
	}

	// В файле 53 таблицы и 9 прочих записей.
	if got, want := len(f.Tables), 53; got != want {
		t.Errorf("таблиц %d, ожидалось %d", got, want)
	}
	if got, want := len(f.Raw), 9; got != want {
		t.Errorf("прочих записей %d, ожидалось %d", got, want)
	}
}

func TestNextID(t *testing.T) {
	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.NextID("elements", "ELEMENT_ID")
	if err != nil {
		t.Fatal(err)
	}
	if id <= 1 {
		t.Errorf("следующий ELEMENT_ID = %d, ожидался MAX+1", id)
	}
}

func TestOpenRejectsNonZip(t *testing.T) {
	if _, err := rsf.Read(bytes.NewReader([]byte("не архив")), 8); err == nil {
		t.Error("мусор вместо ZIP должен давать ошибку")
	}
}

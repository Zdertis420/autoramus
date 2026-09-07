package rsf_test

import (
	"bytes"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

const header = `<?xml version="1.0" encoding="UTF-8"?>`

// source собирает дамп таблицы в той же форме, в какой его пишет Ramus.
func source(body string) string {
	return header + `<table generate-from-table="проба" generate-time="Sat Sep 05 18:31:17 MSK 2026" prefix="ramus_">` +
		`<fields><field id="0" name="ID" type="BIGINT"/><field id="1" name="VALUE" type="CLOB"/></fields>` +
		body + `</table>`
}

func parse(t *testing.T, xml string) *rsf.Table {
	t.Helper()
	table, generateTime, ok := rsf.ParseTable("data/проба.xml", []byte(xml))
	if !ok {
		t.Fatal("таблица не разобралась")
	}
	if generateTime == "" {
		t.Error("потеряна отметка generate-time")
	}
	return table
}

// TestEncodeIsByteExact — то, что разобрано, должно записаться обратно
// посимвольно. Отдельно проверяются случаи, которых в реальном файле нет:
// экранирование и пустая таблица.
func TestEncodeIsByteExact(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			"пустая таблица закрывается одним тегом",
			`<data/>`,
		},
		{
			"обычные строки",
			`<data><row><f id="0">1</f><f id="1">Ткань</f></row></data>`,
		},
		{
			"пустое значение и NULL — разные вещи",
			`<data><row><f id="0">1</f><f id="1"/></row><row><f id="0">2</f></row></data>`,
		},
		{
			"экранирование",
			`<data><row><f id="0">1</f><f id="1">A &amp; B &lt;c&gt; &quot;d&quot;</f></row></data>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := source(tt.body)
			table := parse(t, want)
			got := table.Encode("Sat Sep 05 18:31:17 MSK 2026")
			if !bytes.Equal(got, []byte(want)) {
				t.Errorf("запись разошлась с источником\n  источник: %s\n  получено: %s", want, got)
			}
		})
	}
}

// TestNullIsNotEmpty — путаница здесь тихо теряет значения, о чём отдельно
// предупреждает RSF-FORMAT.md §6.
func TestNullIsNotEmpty(t *testing.T) {
	table := parse(t, source(
		`<data><row><f id="0">1</f><f id="1"/></row><row><f id="0">2</f></row></data>`))

	empty := table.Value(table.Rows[0], "VALUE")
	if !empty.Valid || empty.Text != "" {
		t.Errorf("пустое значение = %+v, ожидалась пустая строка", empty)
	}
	null := table.Value(table.Rows[1], "VALUE")
	if null.Valid {
		t.Errorf("отсутствующее значение = %+v, ожидался NULL", null)
	}
}

func TestColumnLookupIgnoresCase(t *testing.T) {
	table := parse(t, source(`<data><row><f id="0">7</f></row></data>`))
	if got := table.Str(table.Rows[0], "id"); got != "7" {
		t.Errorf("поиск колонки по имени в другом регистре дал %q", got)
	}
	if _, ok := table.Column("НЕТ_ТАКОЙ"); ok {
		t.Error("несуществующая колонка не должна находиться")
	}
}

func TestSelectAndAdd(t *testing.T) {
	table := parse(t, source(
		`<data><row><f id="0">1</f><f id="1">Ткань</f></row><row><f id="0">2</f><f id="1">Юбка</f></row></data>`))

	rows := table.Select(rsf.Eq("VALUE", "Юбка"))
	if len(rows) != 1 || table.Str(rows[0], "ID") != "2" {
		t.Fatalf("отбор дал %d строк: %+v", len(rows), rows)
	}
	if _, ok := table.First(rsf.Eq("VALUE", "Фурнитура")); ok {
		t.Error("несуществующее значение не должно находиться")
	}

	if _, err := table.Add(map[string]string{"ID": "3", "VALUE": "Выкройка"}); err != nil {
		t.Fatal(err)
	}
	if len(table.Rows) != 3 {
		t.Errorf("строк после добавления %d, ожидалось 3", len(table.Rows))
	}
	if _, err := table.Add(map[string]string{"НЕТ_ТАКОЙ": "x"}); err == nil {
		t.Error("запись в несуществующую колонку должна давать ошибку")
	}
}

// TestPropertiesIsNotATable — под data/ лежат ещё и java.util.Properties;
// такие записи переносятся в новый файл как есть.
func TestPropertiesIsNotATable(t *testing.T) {
	properties := header + `<!DOCTYPE properties SYSTEM "http://java.sun.com/dtd/properties.dtd">` +
		`<properties><entry key="ApplicationName">Ramus</entry></properties>`
	if _, _, ok := rsf.ParseTable("data/application_metadata.xml", []byte(properties)); ok {
		t.Error("Properties не должны считаться таблицей")
	}
	if _, _, ok := rsf.ParseTable("data/битый.xml", []byte("<table><не закрыт>")); ok {
		t.Error("битый XML не должен считаться таблицей")
	}
}

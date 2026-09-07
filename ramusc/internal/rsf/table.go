// Пакет rsf читает и записывает файлы Ramus (*.rsf) — ZIP с дампами таблиц
// встроенной БД. Формат разобран в RSF-FORMAT.md, эталонная реализация —
// algodemo/rsf.py, этот пакет её переносит.
//
// Значения хранятся строками ровно так, как лежат в файле: типы приходят
// из объявления колонки, а типизированный доступ — это представление поверх
// строки. Благодаря этому нетронутые данные записываются обратно посимвольно,
// и round-trip не зависит от того, как Go форматирует числа.
package rsf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// Field — колонка таблицы. Ramus ищет колонку по имени, а id — лишь ссылка
// для <f id=…> внутри строки (RSF-FORMAT.md §2).
type Field struct {
	ID   string
	Name string
	Type string
}

// Value — значение ячейки. Отсутствие тега <f> и пустой <f/> — разные вещи:
// первое NULL, второе пустая строка. Путать их нельзя, об этом отдельно
// предупреждает §6 формата.
type Value struct {
	Text  string
	Valid bool // false — NULL, тега <f> в строке не было
}

// Text собирает непустое значение.
func Text(s string) Value { return Value{Text: s, Valid: true} }

// Null — значение-NULL.
var Null = Value{}

// Row — строка таблицы: значения по именам колонок.
type Row map[string]Value

// Cond — условие отбора: поле равно значению.
type Cond struct {
	Field string
	Value string
}

// Eq собирает условие отбора.
func Eq(field, value string) Cond { return Cond{Field: field, Value: value} }

// Table — одна таблица data/**/*.xml.
type Table struct {
	Path   string // путь внутри ZIP
	Name   string // generate-from-table
	Prefix string // обычно ramus_
	Fields []Field
	Rows   []Row

	byName map[string]Field // ИМЯ В ВЕРХНЕМ РЕГИСТРЕ → колонка
}

// Column находит колонку по имени; регистр не важен, как и в XMLToTable.
func (t *Table) Column(name string) (Field, bool) {
	f, ok := t.byName[strings.ToUpper(name)]
	return f, ok
}

// Value достаёт значение ячейки. Нет такой колонки или нет значения — Null.
func (t *Table) Value(row Row, name string) Value {
	f, ok := t.Column(name)
	if !ok {
		return Null
	}
	return row[f.Name]
}

// Str — значение ячейки строкой; NULL и отсутствие колонки дают пустую строку.
func (t *Table) Str(row Row, name string) string { return t.Value(row, name).Text }

// Set записывает значение в строку.
func (t *Table) Set(row Row, name string, v Value) error {
	f, ok := t.Column(name)
	if !ok {
		return fmt.Errorf("%s: нет колонки %s", t.Name, name)
	}
	row[f.Name] = v
	return nil
}

// Select отбирает строки, у которых все перечисленные поля равны заданным
// значениям.
func (t *Table) Select(conds ...Cond) []Row {
	var out []Row
	for _, row := range t.Rows {
		if t.matches(row, conds) {
			out = append(out, row)
		}
	}
	return out
}

// First отдаёт первую подходящую строку.
func (t *Table) First(conds ...Cond) (Row, bool) {
	for _, row := range t.Rows {
		if t.matches(row, conds) {
			return row, true
		}
	}
	return nil, false
}

func (t *Table) matches(row Row, conds []Cond) bool {
	for _, c := range conds {
		if t.Str(row, c.Field) != c.Value {
			return false
		}
	}
	return true
}

// Add добавляет строку: все колонки NULL, кроме перечисленных.
func (t *Table) Add(values map[string]string) (Row, error) {
	row := make(Row, len(t.Fields))
	for name, v := range values {
		if err := t.Set(row, name, Text(v)); err != nil {
			return nil, err
		}
	}
	t.Rows = append(t.Rows, row)
	return row, nil
}

// xmlHeader — Java пишет объявление без пробела перед закрывающим ?>.
const xmlHeader = `<?xml version="1.0" encoding="UTF-8"?>`

// Encode собирает XML таблицы. Форма повторяет вывод Ramus вплоть до байта:
// без переносов строк, с самозакрывающимися тегами и в том же порядке
// атрибутов — иначе round-trip перестаёт быть проверкой.
func (t *Table) Encode(generateTime string) []byte {
	var b bytes.Buffer
	b.WriteString(xmlHeader)
	b.WriteString(`<table generate-from-table="`)
	escape(&b, t.Name)
	b.WriteString(`" generate-time="`)
	escape(&b, generateTime)
	b.WriteString(`" prefix="`)
	escape(&b, t.Prefix)
	b.WriteString(`">`)

	b.WriteString(`<fields>`)
	for _, f := range t.Fields {
		b.WriteString(`<field id="`)
		escape(&b, f.ID)
		b.WriteString(`" name="`)
		escape(&b, f.Name)
		b.WriteString(`" type="`)
		escape(&b, f.Type)
		b.WriteString(`"/>`)
	}
	b.WriteString(`</fields>`)

	// Пустую секцию Java закрывает одним тегом. Мелочь, но без неё
	// побайтово расходятся 27 таблиц из 53.
	if len(t.Rows) == 0 {
		b.WriteString(`<data/></table>`)
		return b.Bytes()
	}

	b.WriteString(`<data>`)
	for _, row := range t.Rows {
		b.WriteString(`<row>`)
		for _, f := range t.Fields {
			v := row[f.Name]
			if !v.Valid {
				continue // NULL — тега нет вовсе
			}
			b.WriteString(`<f id="`)
			b.WriteString(f.ID)
			if v.Text == "" {
				b.WriteString(`"/>`)
				continue
			}
			b.WriteString(`">`)
			escape(&b, v.Text)
			b.WriteString(`</f>`)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</data></table>`)
	return b.Bytes()
}

// escape экранирует ровно те символы, что и Ramus. Апостроф не трогаем:
// в двойных кавычках он законен, и лишнее экранирование развалило бы
// побайтовое сравнение.
func escape(b *bytes.Buffer, s string) {
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
}

// разбор

type xmlTable struct {
	XMLName      xml.Name   `xml:"table"`
	Name         string     `xml:"generate-from-table,attr"`
	GenerateTime string     `xml:"generate-time,attr"`
	Prefix       string     `xml:"prefix,attr"`
	Fields       []xmlField `xml:"fields>field"`
	Rows         []xmlRow   `xml:"data>row"`
}

type xmlField struct {
	ID   string `xml:"id,attr"`
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type xmlRow struct {
	Cells []xmlCell `xml:"f"`
}

type xmlCell struct {
	ID   string `xml:"id,attr"`
	Text string `xml:",chardata"`
}

// ParseTable разбирает дамп таблицы. Второй результат — false, если это не
// таблица: под data/ лежат ещё и java.util.Properties, а такие записи мы
// переносим в новый файл как есть.
func ParseTable(path string, data []byte) (*Table, string, bool) {
	var doc xmlTable
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, "", false
	}

	t := &Table{
		Path:   path,
		Name:   doc.Name,
		Prefix: doc.Prefix,
		byName: make(map[string]Field, len(doc.Fields)),
	}
	byID := make(map[string]string, len(doc.Fields))
	for _, f := range doc.Fields {
		field := Field{ID: f.ID, Name: f.Name, Type: f.Type}
		t.Fields = append(t.Fields, field)
		t.byName[strings.ToUpper(f.Name)] = field
		byID[f.ID] = f.Name
	}

	for _, r := range doc.Rows {
		row := make(Row, len(t.Fields))
		for _, cell := range r.Cells {
			name, ok := byID[cell.ID]
			if !ok {
				continue // ссылка на несуществующую колонку — данных за ней нет
			}
			row[name] = Text(cell.Text)
		}
		t.Rows = append(t.Rows, row)
	}
	return t, doc.GenerateTime, true
}

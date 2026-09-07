package rsf

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TimeLayout — формат TIMESTAMP в дампах: DateFormat.SHORT/SHORT с английской
// локалью, то есть «9/4/26 10:23 AM» (RSF-FORMAT.md §2). Другой формат Ramus
// не разберёт и молча положит NULL.
const TimeLayout = "1/2/06 3:04 PM"

// Int64 читает целое из ячейки: типы BIGINT, LONG, int8, INTEGER, int4.
func (t *Table) Int64(row Row, name string) (int64, bool) {
	v := t.Value(row, name)
	if !v.Valid || v.Text == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v.Text), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// Int64Or читает целое, подставляя запасное значение. Удобно там, где
// отсутствие строки равносильно «не задано»: например -1 у ссылок.
func (t *Table) Int64Or(row Row, name string, fallback int64) int64 {
	if n, ok := t.Int64(row, name); ok {
		return n
	}
	return fallback
}

// Float читает число с плавающей точкой: DOUBLE, float8. Разделитель — точка.
func (t *Table) Float(row Row, name string) (float64, bool) {
	v := t.Value(row, name)
	if !v.Valid || v.Text == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v.Text), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// Bool читает BOOLEAN: в дампе это литералы TRUE и FALSE.
func (t *Table) Bool(row Row, name string) (bool, bool) {
	v := t.Value(row, name)
	if !v.Valid {
		return false, false
	}
	switch strings.ToUpper(strings.TrimSpace(v.Text)) {
	case "TRUE":
		return true, true
	case "FALSE":
		return false, true
	default:
		return false, false
	}
}

// Time читает TIMESTAMP.
func (t *Table) Time(row Row, name string) (time.Time, bool) {
	v := t.Value(row, name)
	if !v.Valid || v.Text == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(TimeLayout, v.Text)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

// Blob читает BLOB, VARBINARY, bytea.
func (t *Table) Blob(row Row, name string) ([]byte, bool) {
	v := t.Value(row, name)
	if !v.Valid {
		return nil, false
	}
	b, err := DecodeBlob(v.Text)
	if err != nil {
		return nil, false
	}
	return b, true
}

// DecodeBlob разворачивает блоб в байты. Ramus пишет hex не от самого байта,
// а от «байт + 128» (TableToXML.ByteAConverter), поэтому C4E9E1ECEFE7 — это
// строка Dialog, а не то, что можно прочитать hex-редактором напрямую.
func DecodeBlob(s string) ([]byte, error) {
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("блоб: %w", err)
	}
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b - 128
	}
	return out, nil
}

// EncodeBlob сворачивает байты обратно.
func EncodeBlob(b []byte) string {
	shifted := make([]byte, len(b))
	for i, v := range b {
		shifted[i] = v + 128
	}
	return strings.ToUpper(hex.EncodeToString(shifted))
}

// FormatFloat печатает число так, как это делает Java: с точкой и всегда
// с дробной частью, то есть 186.0, а не 186. Go по умолчанию дробную часть
// у целых значений опускает, и файл переставал бы совпадать с оригиналом.
func FormatFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// FormatTime печатает отметку времени в том виде, в каком её ждёт Ramus.
func FormatTime(ts time.Time) string { return ts.Format(TimeLayout) }

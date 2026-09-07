package rsf_test

import (
	"testing"
	"time"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// TestBlob — пример прямо из RSF-FORMAT.md §2.
func TestBlob(t *testing.T) {
	const encoded = "C4E9E1ECEFE7"
	decoded, err := rsf.DecodeBlob(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "Dialog" {
		t.Errorf("блоб раскодирован в %q, ожидалось «Dialog»", decoded)
	}
	if got := rsf.EncodeBlob(decoded); got != encoded {
		t.Errorf("обратно свернулось в %s, ожидалось %s", got, encoded)
	}
	if _, err := rsf.DecodeBlob("не hex"); err == nil {
		t.Error("мусор вместо hex должен давать ошибку")
	}
}

func TestFormatFloat(t *testing.T) {
	tests := map[float64]string{
		186:               "186.0",
		186.5:             "186.5",
		-1:                "-1.0",
		0:                 "0.0",
		203.6470588235294: "203.6470588235294",
	}
	for value, want := range tests {
		if got := rsf.FormatFloat(value); got != want {
			t.Errorf("FormatFloat(%v) = %s, ожидалось %s", value, got, want)
		}
	}
}

func TestTimeLayout(t *testing.T) {
	const stamp = "9/4/26 10:23 AM"
	ts, err := time.Parse(rsf.TimeLayout, stamp)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Year() != 2026 || ts.Month() != time.September || ts.Day() != 4 || ts.Hour() != 10 {
		t.Errorf("разобрано как %v", ts)
	}
	if got := rsf.FormatTime(ts); got != stamp {
		t.Errorf("обратно напечаталось как %q, ожидалось %q", got, stamp)
	}
}

// TestTypedAccess — типизированный доступ поверх настоящих данных.
func TestTypedAccess(t *testing.T) {
	f, err := rsf.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	rect := f.MustTable("attribute_rectangles")
	if len(rect.Rows) == 0 {
		t.Fatal("в таблице прямоугольников нет строк")
	}
	if _, ok := rect.Float(rect.Rows[0], "WIDTH"); !ok {
		t.Error("WIDTH не читается как число")
	}
	if _, ok := rect.Int64(rect.Rows[0], "ELEMENT_ID"); !ok {
		t.Error("ELEMENT_ID не читается как целое")
	}
	if _, ok := rect.Int64(rect.Rows[0], "WIDTH"); ok {
		t.Error("дробное значение не должно читаться как целое")
	}

	prefs := f.MustTable("attribute_model_preferences")
	if len(prefs.Rows) > 0 {
		if _, ok := prefs.Time(prefs.Rows[0], "CREATE_DATE"); !ok {
			t.Error("CREATE_DATE не читается как отметка времени")
		}
	}
}

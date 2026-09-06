package syntax

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

const jsonDoc = `{
  "model": "Юбка",
  "flows": ["Ткань", "Юбка"],
  "functions": [
    {"name": "Раскрой", "note": null}
  ]
}
`

func TestParseJSONPositions(t *testing.T) {
	root, diags := ParseJSON([]byte(jsonDoc))
	if len(diags) != 0 {
		t.Fatalf("неожиданные диагностики: %+v", diags)
	}

	// Колонка считается в рунах: если бы считались байты, кириллица сдвинула
	// бы позиции вправо и маркеры в редакторе встали бы не на то место.
	cases := []struct {
		pointer string
		want    diag.Pos
	}{
		{"", diag.Pos{Line: 1, Column: 1}},
		{"/model", diag.Pos{Line: 2, Column: 3}},
		{"/flows", diag.Pos{Line: 3, Column: 3}},
		{"/flows/0", diag.Pos{Line: 3, Column: 13}},
		{"/flows/1", diag.Pos{Line: 3, Column: 22}},
		{"/functions/0", diag.Pos{Line: 5, Column: 5}},
		{"/functions/0/name", diag.Pos{Line: 5, Column: 6}},
		{"/functions/0/note", diag.Pos{Line: 5, Column: 25}},
	}
	for _, c := range cases {
		if got := root.FindKeyPos(c.pointer); got != c.want {
			t.Errorf("FindKeyPos(%q) = %+v, ожидалось %+v", c.pointer, got, c.want)
		}
	}
}

func TestParseJSONValues(t *testing.T) {
	root, diags := ParseJSON([]byte(jsonDoc))
	if len(diags) != 0 {
		t.Fatalf("неожиданные диагностики: %+v", diags)
	}
	if got := root.Find("/model").Str; got != "Юбка" {
		t.Errorf("/model = %q", got)
	}
	if got := root.Find("/functions/0/note").Kind; got != Null {
		t.Errorf("/functions/0/note = %v, ожидался Null", got)
	}
	if root.Find("/functions/1") != nil {
		t.Error("Find вернул узел для несуществующего индекса")
	}
	if root.Find("/нет") != nil {
		t.Error("Find вернул узел для несуществующего поля")
	}
}

func TestParseJSONDuplicateKey(t *testing.T) {
	src := "{\n  \"model\": \"А\",\n  \"model\": \"Б\"\n}\n"
	_, diags := ParseJSON([]byte(src))
	if len(diags) != 1 || diags[0].Code != diag.CodeDuplicateKey {
		t.Fatalf("ожидалась одна диагностика duplicate_key, получено: %+v", diags)
	}
	if diags[0].Line != 3 || diags[0].Column != 3 {
		t.Errorf("позиция повтора = %d:%d, ожидалось 3:3", diags[0].Line, diags[0].Column)
	}
}

func TestParseJSONErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code diag.Code
	}{
		{"пропущена запятая", "{\n  \"a\": 1\n  \"b\": 2\n}", diag.CodeSyntax},
		{"оборван документ", "{\n  \"a\": [1, 2", diag.CodeSyntax},
		{"мусор после значения", "{\"a\": 1} лишнее", diag.CodeSyntax},
		{"пусто", "", diag.CodeEmptyDocument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := ParseJSON([]byte(tt.src))
			if !diags.HasErrors() {
				t.Fatalf("ошибка не обнаружена")
			}
			if diags[0].Code != tt.code {
				t.Errorf("код = %s, ожидался %s (%+v)", diags[0].Code, tt.code, diags)
			}
		})
	}
}

func TestLoadDispatch(t *testing.T) {
	// Документ, законный и как JSON, и как YAML: диспетчер обязан выбрать
	// JSON по первому непробельному символу (Р10).
	src := []byte("  {\"model\": \"Юбка\", \"flows\": [\"Ткань\"]}")
	root, diags := Load(src)
	if len(diags) != 0 {
		t.Fatalf("неожиданные диагностики: %+v", diags)
	}
	if got := root.Find("/model").Str; got != "Юбка" {
		t.Errorf("/model = %q", got)
	}

	// А документ с комментарием — только YAML.
	root, diags = Load([]byte("# комментарий\nmodel: Юбка\n"))
	if len(diags) != 0 {
		t.Fatalf("неожиданные диагностики: %+v", diags)
	}
	if got := root.Find("/model").Str; got != "Юбка" {
		t.Errorf("/model = %q", got)
	}
}

func TestLoadStripsBOM(t *testing.T) {
	src := append([]byte{0xEF, 0xBB, 0xBF}, []byte("{\"model\": \"Юбка\"}")...)
	root, diags := Load(src)
	if len(diags) != 0 {
		t.Fatalf("неожиданные диагностики: %+v", diags)
	}
	// Метка заменена пробелами, а не вырезана, поэтому позиции не съезжают.
	if got := root.FindKeyPos("/model"); got != (diag.Pos{Line: 1, Column: 5}) {
		t.Errorf("позиция /model = %+v, ожидалось 1:5", got)
	}
}

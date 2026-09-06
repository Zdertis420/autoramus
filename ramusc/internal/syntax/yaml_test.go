package syntax

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

const yamlDoc = `model: Юбка
flows: [Ткань, Юбка]
functions:
  - name: Раскрой
    note: Ручной раскрой
`

func TestParseYAMLPositions(t *testing.T) {
	root, diags := ParseYAML([]byte(yamlDoc))
	if len(diags) != 0 {
		t.Fatalf("неожиданные диагностики: %+v", diags)
	}

	// Та же проверка, что и для JSON: колонка должна быть в рунах.
	// Если yaml.v3 когда-нибудь начнёт считать байты, тест это поймает
	// и позицию надо будет пересчитывать через lineIndex.posFromByteColumn.
	cases := []struct {
		pointer string
		want    diag.Pos
	}{
		{"/model", diag.Pos{Line: 1, Column: 1}},
		{"/flows", diag.Pos{Line: 2, Column: 1}},
		{"/flows/0", diag.Pos{Line: 2, Column: 9}},
		{"/flows/1", diag.Pos{Line: 2, Column: 16}},
		{"/functions/0", diag.Pos{Line: 4, Column: 5}},
		{"/functions/0/name", diag.Pos{Line: 4, Column: 5}},
		{"/functions/0/note", diag.Pos{Line: 5, Column: 5}},
	}
	for _, c := range cases {
		if got := root.FindKeyPos(c.pointer); got != c.want {
			t.Errorf("FindKeyPos(%q) = %+v, ожидалось %+v", c.pointer, got, c.want)
		}
	}
}

// Р15: имена вроде «Да», «on», «off» не должны превращаться в логические
// значения. yaml.v3 разрешает скаляры по YAML 1.2, но полагаться на это
// без теста нельзя — от этого зависит корректность имён работ и потоков.
func TestParseYAMLScalarsStayStrings(t *testing.T) {
	src := `da: Да
on: on
off: off
yes: yes
no: no
truth: true
lie: FALSE
count: 12
frac: 1.5
hex: 0x10
empty:
`
	root, diags := ParseYAML([]byte(src))
	if len(diags) != 0 {
		t.Fatalf("неожиданные диагностики: %+v", diags)
	}
	strings := []string{"da", "on", "off", "yes", "no"}
	for _, key := range strings {
		n := root.Field(key)
		if n == nil || n.Kind != String {
			t.Errorf("поле %q: вид %v, ожидалась строка", key, n.Kind)
		}
	}
	if n := root.Field("truth"); n.Kind != Bool || !n.Bool {
		t.Errorf("truth: %+v, ожидалось true", n)
	}
	if n := root.Field("lie"); n.Kind != Bool || n.Bool {
		t.Errorf("lie: %+v, ожидалось false", n)
	}
	if n := root.Field("count"); n.Kind != Number || n.Num.String() != "12" {
		t.Errorf("count: %+v", n)
	}
	if n := root.Field("frac"); n.Kind != Number || n.Num.String() != "1.5" {
		t.Errorf("frac: %+v", n)
	}
	// Шестнадцатеричный литерал YAML в JSON недопустим — приводим к десятичному.
	if n := root.Field("hex"); n.Kind != Number || n.Num.String() != "16" {
		t.Errorf("hex: %+v, ожидалось 16", n)
	}
	if n := root.Field("empty"); n.Kind != Null {
		t.Errorf("empty: %+v, ожидался null", n)
	}
}

func TestParseYAMLRejectsAnchors(t *testing.T) {
	// Имя якоря — латиницей: кириллические якоря YAML не разрешает сам,
	// а проверяем мы здесь наш запрет, а не его лексер.
	src := "flows: &spisok [Ткань]\nin: *spisok\n"
	_, diags := ParseYAML([]byte(src))
	if len(diags) != 2 {
		t.Fatalf("ожидались две диагностики (якорь и алиас), получено: %+v", diags)
	}
	for _, d := range diags {
		if d.Code != diag.CodeYAMLAnchor {
			t.Errorf("код = %s, ожидался %s", d.Code, diag.CodeYAMLAnchor)
		}
	}
}

func TestParseYAMLRejectsDuplicateKeys(t *testing.T) {
	src := "model: А\nflows: [Т]\nmodel: Б\n"
	_, diags := ParseYAML([]byte(src))
	if len(diags) != 1 || diags[0].Code != diag.CodeDuplicateKey {
		t.Fatalf("ожидалась одна диагностика duplicate_key, получено: %+v", diags)
	}
	if diags[0].Line != 3 {
		t.Errorf("строка повтора = %d, ожидалось 3", diags[0].Line)
	}
}

func TestParseYAMLRejectsSecondDocument(t *testing.T) {
	src := "model: А\n---\nmodel: Б\n"
	_, diags := ParseYAML([]byte(src))
	if !diags.HasErrors() {
		t.Fatal("второй документ в потоке не замечен")
	}
}

func TestParseYAMLSyntaxError(t *testing.T) {
	src := "model: А\n  flows: [Т]\n"
	_, diags := ParseYAML([]byte(src))
	if len(diags) == 0 || diags[0].Code != diag.CodeSyntax {
		t.Fatalf("ожидалась синтаксическая ошибка, получено: %+v", diags)
	}
	if diags[0].Line == 0 {
		t.Errorf("у синтаксической ошибки YAML нет строки: %+v", diags[0])
	}
}

package decompile

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Options — настройки печати.
type Options struct {
	// Header — строки комментария в шапке документа. YAML их печатает,
	// JSON молча пропускает: комментариев в нём нет (Р10).
	Header []string
}

// Ширина, по которой переносятся длинные списки, и колонка, к которой
// выравниваются ключи ICOM: ровно как в examples/skirt.yaml.
const (
	wrapAt    = 78
	icomWidth = len("mechanism:")
)

// WriteYAML печатает модель на входном языке в каноническом виде (Р14):
// фиксированный порядок ключей, выровненные списки ICOM, перенос длинных
// строк. Любая существующая модель становится готовым образцом для
// нейросети, а стиль вывода — тем, чего ждут от генератора.
func WriteYAML(w io.Writer, m *ir.Model, opts Options) error {
	doc, err := newDocument(m)
	if err != nil {
		return err
	}

	var b strings.Builder
	for _, line := range opts.Header {
		b.WriteString("# " + line + "\n")
	}

	b.WriteString("model: " + yamlString(doc.Model) + "\n")
	if doc.Author != "" {
		b.WriteString("author: " + yamlString(doc.Author) + "\n")
	}
	if doc.Page != "" {
		b.WriteString("page: " + yamlString(doc.Page) + "\n")
	}

	b.WriteString("\n")
	writeFlowList(&b, "", "flows", doc.Flows)

	if len(doc.Streams) > 0 {
		b.WriteString("\nstreams:\n")
		for i, s := range doc.Streams {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  - name: " + yamlString(s.Name) + "\n")
			if s.Note != "" {
				b.WriteString("    note: " + yamlString(s.Note) + "\n")
			}
		}
	}

	b.WriteString("\nfunctions:\n")
	for i, f := range doc.Functions {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  - name: " + yamlString(f.Name) + "\n")
		for _, field := range []struct{ key, value string }{
			{"of", f.Of}, {"kind", f.Kind}, {"type", f.Type},
		} {
			if field.value != "" {
				b.WriteString("    " + field.key + ": " + yamlString(field.value) + "\n")
			}
		}
		for _, side := range []struct {
			key   string
			flows []string
		}{
			{"in", f.In}, {"control", f.Control},
			{"mechanism", f.Mechanism}, {"out", f.Out},
		} {
			if len(side.flows) > 0 {
				writeFlowList(&b, "    ", side.key, side.flows)
			}
		}
		if f.Note != "" {
			b.WriteString("    note: " + yamlString(f.Note) + "\n")
		}
	}

	if len(doc.Links) > 0 {
		b.WriteString("\nlinks:\n")
		for i, l := range doc.Links {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  - flow: " + yamlString(l.Flow) + "\n")
			for _, field := range []struct{ key, value string }{
				{"from", l.From}, {"to", l.To}, {"side", l.Side}, {"note", l.Note},
			} {
				if field.value != "" {
					b.WriteString("    " + field.key + ": " + yamlString(field.value) + "\n")
				}
			}
		}
	}

	if doc.Layout != nil {
		writeLayoutYAML(&b, doc.Layout)
	}

	_, err = io.WriteString(w, b.String())
	return err
}

func writeLayoutYAML(b *strings.Builder, l *layoutDoc) {
	b.WriteString("\nlayout:\n")
	if len(l.Functions) > 0 {
		b.WriteString("  functions:\n")
		for i, f := range l.Functions {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("    - function: " + yamlString(f.Function) + "\n")
			b.WriteString("      x: " + number(f.X) + "\n")
			b.WriteString("      y: " + number(f.Y) + "\n")
			if f.Width != nil {
				b.WriteString("      width: " + number(*f.Width) + "\n")
			}
			if f.Height != nil {
				b.WriteString("      height: " + number(*f.Height) + "\n")
			}
		}
	}
	if len(l.Arrows) == 0 {
		return
	}
	b.WriteString("  arrows:\n")
	for i, a := range l.Arrows {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("    - flow: " + yamlString(a.Flow) + "\n")
		b.WriteString("      on: " + yamlString(a.On) + "\n")
		if a.Context {
			b.WriteString("      context: true\n")
		}
		for _, field := range []struct{ key, value string }{{"from", a.From}, {"to", a.To}} {
			if field.value != "" {
				b.WriteString("      " + field.key + ": " + yamlString(field.value) + "\n")
			}
		}
		points := make([]string, len(a.Points))
		for j, p := range a.Points {
			points[j] = "[" + number(p[0]) + ", " + number(p[1]) + "]"
		}
		writeList(b, "      ", "points:", points)
	}
}

// writeFlowList печатает список имён потоков потоковым стилем и выравнивает
// ключ по колонке mechanism — так список читается как таблица.
func writeFlowList(b *strings.Builder, indent, key string, flows []string) {
	items := make([]string, len(flows))
	for i, f := range flows {
		items[i] = yamlString(f)
	}
	label := key + ":"
	if indent != "" {
		// Внутри работы ключи ICOM выравниваются по самому длинному,
		// mechanism, — так список читается таблицей.
		label = pad(label, icomWidth)
	}
	writeList(b, indent, label, items)
}

// writeList переносит длинные списки, выравнивая продолжение под первым
// элементом.
func writeList(b *strings.Builder, indent, label string, items []string) {
	prefix := indent + label + " "
	continuation := strings.Repeat(" ", len([]rune(prefix))+1)

	line := prefix + "["
	for i, item := range items {
		piece := item
		if i < len(items)-1 {
			piece += ","
		}
		if i > 0 {
			if runeLen(line)+1+runeLen(piece) > wrapAt {
				b.WriteString(line + "\n")
				line = continuation
			} else {
				line += " "
			}
		}
		line += piece
	}
	b.WriteString(line + "]\n")
}

// WriteJSON печатает тот же документ машинным форматом (Р10). Порядок ключей
// задан порядком полей структуры, поэтому вывод так же каноничен.
func WriteJSON(w io.Writer, m *ir.Model) error {
	doc, err := newDocument(m)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Иначе амперсанд и угловые скобки в именах уедут в &.
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}

// document — форма самого документа. Отдельная от IR структура: IR хранит
// связи рёбрами, а документ — списками ICOM, и порядок ключей здесь часть
// формата.
type document struct {
	Model     string        `json:"model"`
	Author    string        `json:"author,omitempty"`
	Page      string        `json:"page,omitempty"`
	Flows     []string      `json:"flows"`
	Streams   []streamDoc   `json:"streams,omitempty"`
	Functions []functionDoc `json:"functions"`
	Links     []linkDoc     `json:"links,omitempty"`
	Layout    *layoutDoc    `json:"layout,omitempty"`
}

type streamDoc struct {
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

type functionDoc struct {
	Name      string   `json:"name"`
	Of        string   `json:"of,omitempty"`
	Kind      string   `json:"kind,omitempty"`
	Type      string   `json:"type,omitempty"`
	In        []string `json:"in,omitempty"`
	Control   []string `json:"control,omitempty"`
	Out       []string `json:"out,omitempty"`
	Mechanism []string `json:"mechanism,omitempty"`
	Note      string   `json:"note,omitempty"`
}

type linkDoc struct {
	Flow string `json:"flow"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	Side string `json:"side,omitempty"`
	Note string `json:"note,omitempty"`
}

type layoutDoc struct {
	Functions []functionLayoutDoc `json:"functions,omitempty"`
	Arrows    []arrowLayoutDoc    `json:"arrows,omitempty"`
}

type functionLayoutDoc struct {
	Function string   `json:"function"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Width    *float64 `json:"width,omitempty"`
	Height   *float64 `json:"height,omitempty"`
}

type arrowLayoutDoc struct {
	Flow    string       `json:"flow"`
	On      string       `json:"on"`
	Context bool         `json:"context,omitempty"`
	From    string       `json:"from,omitempty"`
	To      string       `json:"to,omitempty"`
	Points  [][2]float64 `json:"points"`
}

// newDocument переводит IR в документ. Связь с обоими концами остаётся
// в секции links: списками ICOM её не записать, там у стрелки всегда один
// конец на блоке, а другой на границе листа.
func newDocument(m *ir.Model) (*document, error) {
	if len(m.Classifiers) > 0 {
		return nil, fmt.Errorf("классификаторы пока не печатаются, а в модели их %d", len(m.Classifiers))
	}
	for _, f := range m.Functions {
		if len(f.Raw) > 0 {
			return nil, fmt.Errorf("работа «%s»: блок raw пока не печатается", f.Name.Name)
		}
	}

	doc := &document{Model: m.Name.Raw, Author: m.Author, Page: m.Page}
	for _, flow := range m.Flows {
		doc.Flows = append(doc.Flows, flow.Raw)
	}
	for _, s := range m.Streams {
		doc.Streams = append(doc.Streams, streamDoc{Name: s.Name.Raw, Note: s.Note})
	}

	// Срез заполняется целиком заранее: указатели в него нельзя раздавать
	// до конца дописывания, иначе append переселит массив и часть ICOM
	// уедет в брошенную копию.
	doc.Functions = make([]functionDoc, len(m.Functions))
	sides := make(map[string]*functionDoc, len(m.Functions))
	for i, f := range m.Functions {
		doc.Functions[i] = functionDoc{
			Name: f.Name.Raw,
			Of:   f.Of.Raw,
			Kind: f.Kind.Raw,
			Type: f.Type.Raw,
			Note: f.Note,
		}
		sides[f.Name.Name] = &doc.Functions[i]
	}

	for _, l := range m.Links {
		if l.From.Set() && l.To.Set() {
			doc.Links = append(doc.Links, linkDoc{
				Flow: l.Flow.Raw, From: l.From.Raw, To: l.To.Raw,
				Side: l.Side, Note: l.Note,
			})
			continue
		}
		if l.From.Set() {
			if f := sides[l.From.Name]; f != nil {
				f.Out = append(f.Out, l.Flow.Raw)
			}
			continue
		}
		if f := sides[l.To.Name]; f != nil {
			switch l.SideName() {
			case ir.SideControl:
				f.Control = append(f.Control, l.Flow.Raw)
			case ir.SideMechanism:
				f.Mechanism = append(f.Mechanism, l.Flow.Raw)
			default:
				f.In = append(f.In, l.Flow.Raw)
			}
		}
	}

	if m.Layout != nil {
		doc.Layout = newLayout(m.Layout)
	}
	return doc, nil
}

func newLayout(l *ir.Layout) *layoutDoc {
	out := &layoutDoc{}
	for _, f := range l.Functions {
		entry := functionLayoutDoc{Function: f.Function.Raw, X: f.X.Val, Y: f.Y.Val}
		if f.Width.Set {
			w := f.Width.Val
			entry.Width = &w
		}
		if f.Height.Set {
			h := f.Height.Val
			entry.Height = &h
		}
		out.Functions = append(out.Functions, entry)
	}
	for _, a := range l.Arrows {
		entry := arrowLayoutDoc{
			Flow: a.Flow.Raw, On: a.On.Raw, Context: a.Context,
			From: a.From.Raw, To: a.To.Raw,
		}
		for _, p := range a.Points {
			entry.Points = append(entry.Points, [2]float64{p.X.Val, p.Y.Val})
		}
		out.Arrows = append(out.Arrows, entry)
	}
	if len(out.Functions) == 0 && len(out.Arrows) == 0 {
		return nil
	}
	return out
}

// yamlString печатает скаляр, беря его в кавычки там, где иначе YAML
// прочитает не строку. Правило намеренно осторожное: лишние кавычки безобидны,
// а недостающие меняют смысл документа.
func yamlString(s string) string {
	if s == "" || needsQuotes(s) {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return s
}

func needsQuotes(s string) bool {
	if strings.TrimSpace(s) != s {
		return true
	}
	switch strings.ToLower(s) {
	case "true", "false", "null", "~", "yes", "no", "on", "off":
		// yes/no/on/off по YAML 1.2 — строки, но кавычки снимают любые
		// сомнения о том, какую версию применит чужой разборщик.
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	if strings.ContainsAny(s, ":#\n\t") || strings.ContainsAny(s[:1], "-?,[]{}&*!|>'\"%@`") {
		return true
	}
	return false
}

func number(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func pad(s string, width int) string {
	if n := runeLen(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

func runeLen(s string) int { return len([]rune(s)) }

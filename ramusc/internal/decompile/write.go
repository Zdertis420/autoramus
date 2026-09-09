package decompile

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
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
			writeRawYAML(&b, "    ", s.Raw)
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
		writeRawYAML(&b, "    ", f.Raw)
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

	writeRawYAML(&b, "", doc.Raw)

	writeClassifiersYAML(&b, doc.Classifiers)

	if doc.Layout != nil {
		writeLayoutYAML(&b, doc.Layout)
	}

	_, err = io.WriteString(w, b.String())
	return err
}

// writeClassifiersYAML печатает справочники. Пустая секция не печатается
// вовсе: справочник без единой строки — обычное состояние модели, и заголовок
// без содержимого только сбивал бы с толку.
func writeClassifiersYAML(b *strings.Builder, classifiers []classifierDoc) {
	if len(classifiers) == 0 {
		return
	}

	b.WriteString("\nclassifiers:\n")
	for i, c := range classifiers {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  - name: " + yamlString(c.Name) + "\n")
		if c.Note != "" {
			b.WriteString("    note: " + yamlString(c.Note) + "\n")
		}
		writeRawYAML(b, "    ", c.Raw)

		b.WriteString("    columns:\n")
		for _, col := range c.Columns {
			b.WriteString("      - name: " + yamlString(col.Name) + "\n")
			b.WriteString("        type: " + yamlString(col.Type) + "\n")
			if col.Of != "" {
				b.WriteString("        of: " + yamlString(col.Of) + "\n")
			}
			if col.Note != "" {
				b.WriteString("        note: " + yamlString(col.Note) + "\n")
			}
			writeRawYAML(b, "        ", col.Raw)
		}

		if len(c.Rows) == 0 {
			continue
		}
		b.WriteString("    rows:\n")
		for _, row := range c.Rows {
			b.WriteString("      - cells:\n")
			for _, cell := range row.Cells {
				b.WriteString("          - column: " + yamlString(cell.Column) + "\n")
				b.WriteString("            value: " + cellYAML(cell.Value) + "\n")
			}
			if row.Note != "" {
				b.WriteString("        note: " + yamlString(row.Note) + "\n")
			}
			writeRawYAML(b, "        ", row.Raw)
		}
	}
}

// newRaw переводит люк из IR в документ.
func newRaw(source []ir.RawAttr) []rawDoc {
	var out []rawDoc
	for _, a := range source {
		out = append(out, rawDoc{Attribute: a.Attribute.Raw, Value: a.Value})
	}
	return out
}

// writeRawYAML печатает люк. Пустой не печатается вовсе: люк есть у каждого
// элемента (Р17), и у большинства ему нечего нести.
func writeRawYAML(b *strings.Builder, indent string, raw []rawDoc) {
	if len(raw) == 0 {
		return
	}
	b.WriteString(indent + "raw:\n")
	for _, a := range raw {
		b.WriteString(indent + "  - attribute: " + yamlString(a.Attribute) + "\n")
		b.WriteString(indent + "    value: " + rawValueYAML(a.Value) + "\n")
	}
}

// rawValueYAML печатает значение записи люка. Многоколоночное значение —
// потоковым отображением: оно короткое, а разворачивать его в блок значило бы
// утопить документ в отступах.
func rawValueYAML(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return cellYAML(v)
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)

	fields := make([]string, 0, len(names))
	for _, k := range names {
		fields = append(fields, k+": "+cellYAML(m[k]))
	}
	return "{" + strings.Join(fields, ", ") + "}"
}

// cellYAML печатает значение ячейки по его виду: строку в кавычках, число и
// признак как есть, список ссылок потоковым стилем. Печатать всё строками
// нельзя — валидатор сверяет вид значения с типом колонки.
func cellYAML(v any) string {
	switch value := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(value)
	case json.Number:
		return value.String()
	case []any:
		items := make([]string, len(value))
		for i, item := range value {
			items[i] = cellYAML(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case string:
		return yamlString(value)
	default:
		return yamlString(fmt.Sprint(value))
	}
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
		b.WriteString("      segments:\n")
		for j, s := range a.Segments {
			if j > 0 {
				b.WriteString("\n")
			}
			b.WriteString("        - on: " + yamlString(s.On) + "\n")
			if s.Context {
				b.WriteString("          context: true\n")
			}
			// from и to выравниваются друг под другом: концы читаются парой.
			for _, end := range []struct {
				key      string
				endpoint *endpointDoc
			}{{"from", s.From}, {"to", s.To}} {
				if end.endpoint == nil {
					continue // висящий конец: писать нечего
				}
				b.WriteString("          " + pad(end.key+":", len("from:")) + " " +
					endpointYAML(end.endpoint) + "\n")
			}
			points := make([]string, len(s.Points))
			for k, p := range s.Points {
				points[k] = "[" + number(p[0]) + ", " + number(p[1]) + "]"
			}
			writeList(b, "          ", "points:", points)
			// Люк у сегмента, а не у стрелки: свойства подписи и
			// альтернативный текст принадлежат сектору, и у разных сегментов
			// одной стрелки они разные (Р17).
			writeRawYAML(b, "          ", s.Raw)
		}
	}
}

// endpointYAML печатает конец сегмента потоковым отображением: он короткий,
// и разворачивать его в блок значило бы утопить геометрию в отступах.
func endpointYAML(e *endpointDoc) string {
	var fields []string
	if e.Function != "" {
		fields = append(fields, "function: "+yamlString(e.Function))
	}
	if e.Side != "" {
		fields = append(fields, "side: "+yamlString(e.Side))
	}
	if e.Border != "" {
		fields = append(fields, "border: "+yamlString(e.Border))
	}
	if e.Node != "" {
		fields = append(fields, "node: "+yamlString(e.Node))
	}
	if e.Tunnel {
		fields = append(fields, "tunnel: true")
	}
	return "{" + strings.Join(fields, ", ") + "}"
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

	// Классификаторы идут после работ и связей, но перед layout: сперва
	// содержание модели, потом геометрия.
	Classifiers []classifierDoc `json:"classifiers,omitempty"`
	Layout      *layoutDoc      `json:"layout,omitempty"`
	Raw         []rawDoc        `json:"raw,omitempty"`
}

// rawDoc — запись люка: атрибут Ramus, которому нет соответствия в полях
// языка, вместе со значением как есть (Р9, Р17).
type rawDoc struct {
	Attribute string `json:"attribute"`
	Value     any    `json:"value"`
}

type classifierDoc struct {
	Name    string      `json:"name"`
	Columns []columnDoc `json:"columns"`
	Rows    []rowDoc    `json:"rows,omitempty"`
	Note    string      `json:"note,omitempty"`
	Raw     []rawDoc    `json:"raw,omitempty"`
}

type columnDoc struct {
	Name string   `json:"name"`
	Type string   `json:"type"`
	Of   string   `json:"of,omitempty"`
	Note string   `json:"note,omitempty"`
	Raw  []rawDoc `json:"raw,omitempty"`
}

type rowDoc struct {
	Cells []cellDoc `json:"cells"`
	Note  string    `json:"note,omitempty"`
	Raw   []rawDoc  `json:"raw,omitempty"`
}

type cellDoc struct {
	Column string `json:"column"`
	Value  any    `json:"value"`
}

type streamDoc struct {
	Name string   `json:"name"`
	Note string   `json:"note,omitempty"`
	Raw  []rawDoc `json:"raw,omitempty"`
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
	Raw       []rawDoc `json:"raw,omitempty"`
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
	Flow     string       `json:"flow"`
	Segments []segmentDoc `json:"segments"`
}

type segmentDoc struct {
	On      string       `json:"on"`
	Context bool         `json:"context,omitempty"`
	From    *endpointDoc `json:"from,omitempty"`
	To      *endpointDoc `json:"to,omitempty"`
	Points  [][2]float64 `json:"points"`
	Raw     []rawDoc     `json:"raw,omitempty"`
}

type endpointDoc struct {
	Function string `json:"function,omitempty"`
	Side     string `json:"side,omitempty"`
	Border   string `json:"border,omitempty"`
	Node     string `json:"node,omitempty"`
	Tunnel   bool   `json:"tunnel,omitempty"`
}

// newDocument переводит IR в документ. Связь с обоими концами остаётся
// в секции links: списками ICOM её не записать, там у стрелки всегда один
// конец на блоке, а другой на границе листа.
// Проверки на непечатаемое здесь больше нет: она смотрела на незаполненность
// IR и потому срабатывала только на модели, собранной вручную. Из команды она
// была недостижима — разбор классификаторы и не читал, — и файл со
// справочниками уходил в вывод без них, с кодом успеха. Обнаружение переехало
// в rsf.Unsupported, который смотрит в сам файл (FR-002).
func newDocument(m *ir.Model) (*document, error) {
	doc := &document{Model: m.Name.Raw, Author: m.Author, Page: m.Page, Raw: newRaw(m.Raw)}
	for _, flow := range m.Flows {
		doc.Flows = append(doc.Flows, flow.Raw)
	}
	for _, s := range m.Streams {
		doc.Streams = append(doc.Streams, streamDoc{Name: s.Name.Raw, Note: s.Note, Raw: newRaw(s.Raw)})
	}

	// Срез заполняется целиком заранее: указатели в него нельзя раздавать
	// до конца дописывания, иначе append переселит массив и часть ICOM
	// уедет в брошенную копию.
	doc.Functions = make([]functionDoc, len(m.Functions))
	// Работы адресуются по идентичности, а не по имени. В .rsf имя работы
	// может повторяться и отсутствовать, и отображение по имени сводило такие
	// работы в одну: связи всех безымянных накапливались на последней.
	// Уникальность имён — требование к входному документу (Р4), а не свойство
	// файла, и опираться на неё при чтении файла нельзя.
	byPath := make(map[string]*functionDoc, len(m.Functions))
	byName := make(map[string]*functionDoc, len(m.Functions))
	for i, f := range m.Functions {
		doc.Functions[i] = functionDoc{
			Name: f.Name.Raw,
			Of:   f.Of.Raw,
			Kind: f.Kind.Raw,
			Type: f.Type.Raw,
			Note: f.Note,
			Raw:  newRaw(f.Raw),
		}
		if f.Name.Path != "" {
			byPath[f.Name.Path] = &doc.Functions[i]
		}
		// Имя остаётся запасным ключом: связь из секции links адресует работу
		// по имени, и её Ref указывает на саму связь, а не на работу.
		byName[f.Name.Name] = &doc.Functions[i]
	}

	owner := func(r ir.Ref) *functionDoc {
		if f, ok := byPath[r.Path]; ok {
			return f
		}
		return byName[r.Name]
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
			if f := owner(l.From); f != nil {
				f.Out = appendFlow(f.Out, l.Flow.Raw)
			}
			continue
		}
		if f := owner(l.To); f != nil {
			switch l.SideName() {
			case ir.SideControl:
				f.Control = appendFlow(f.Control, l.Flow.Raw)
			case ir.SideMechanism:
				f.Mechanism = appendFlow(f.Mechanism, l.Flow.Raw)
			default:
				f.In = appendFlow(f.In, l.Flow.Raw)
			}
		}
	}

	doc.Classifiers = newClassifiers(m.Classifiers)

	if m.Layout != nil {
		doc.Layout = newLayout(m.Layout)
	}
	return doc, nil
}

// appendFlow добавляет поток в список ICOM, не повторяясь. Один поток может
// прийти в работу несколькими секторами — это ветвление, а не несколько
// стрелок, и называть его дважды значило бы описать не ту модель.
func appendFlow(list []string, flow string) []string {
	for _, existing := range list {
		if existing == flow {
			return list
		}
	}
	return append(list, flow)
}

// newClassifiers переводит справочники IR в документ.
func newClassifiers(source []*ir.Classifier) []classifierDoc {
	var out []classifierDoc
	for _, c := range source {
		doc := classifierDoc{Name: c.Name.Raw, Note: c.Note, Raw: newRaw(c.Raw)}
		for _, col := range c.Columns {
			doc.Columns = append(doc.Columns, columnDoc{
				Name: col.Name.Raw, Type: col.Type.Raw, Of: col.Of.Raw, Note: col.Note,
				Raw: newRaw(col.Raw),
			})
		}
		for _, row := range c.Rows {
			r := rowDoc{Note: row.Note, Raw: newRaw(row.Raw)}
			for _, cell := range row.Cells {
				r.Cells = append(r.Cells, cellDoc{Column: cell.Column.Raw, Value: cell.Value})
			}
			doc.Rows = append(doc.Rows, r)
		}
		out = append(out, doc)
	}
	return out
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
		entry := arrowLayoutDoc{Flow: a.Flow.Raw}
		for _, s := range a.Segments {
			seg := segmentDoc{
				On:      s.On.Raw,
				Raw:     newRaw(s.Raw),
				Context: s.Context,
				From:    newEndpoint(s.From),
				To:      newEndpoint(s.To),
			}
			for _, p := range s.Points {
				seg.Points = append(seg.Points, [2]float64{p.X.Val, p.Y.Val})
			}
			entry.Segments = append(entry.Segments, seg)
		}
		out.Arrows = append(out.Arrows, entry)
	}
	if len(out.Functions) == 0 && len(out.Arrows) == 0 {
		return nil
	}
	return out
}

// newEndpoint возвращает nil для висящего конца: его в документе не должно
// быть вовсе, а не «пустым объектом».
func newEndpoint(e *ir.Endpoint) *endpointDoc {
	if e == nil {
		return nil
	}
	return &endpointDoc{
		Function: e.Function.Raw,
		Side:     e.Side,
		Border:   e.Border,
		Node:     e.Node.Raw,
		Tunnel:   e.Tunnel,
	}
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
	// Индикаторы потокового стиля значимы в любой позиции, а не только в
	// начале: списки ICOM и концы сегментов печатаются как [a, b] и {k: v},
	// и запятая внутри имени там разделяет элементы. Имя «Оформление,
	// нормконтроль и утверждение пояснительной записки» без кавычек
	// прочитается как два поля, и документ перестанет описывать ту модель,
	// из которой получен.
	if strings.ContainsAny(s, ":#\n\t,[]{}") {
		return true
	}
	// Остальные индикаторы значимы только в начале скаляра.
	if strings.ContainsAny(s[:1], "-?&*!|>'\"%@`") {
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

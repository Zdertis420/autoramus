package syntax

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

// ParseYAML разбирает YAML в то же дерево, что и ParseJSON.
//
// Строгость по Р15: якоря и алиасы запрещены, дублирующийся ключ — ошибка,
// а не «последний выигрывает». Булевы в yaml.v3 разрешаются по YAML 1.2,
// поэтому `Да`, `on` и `off` остаются строками сами собой — это закреплено
// тестом, а не оставлено на веру.
func ParseYAML(src []byte) (*Node, diag.List) {
	p := &yamlParser{li: newLineIndex(src)}

	dec := yaml.NewDecoder(bytes.NewReader(src))
	var root yaml.Node
	if err := dec.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			p.diags.Add(diag.New(diag.CodeEmptyDocument, p.li.pos(len(src)), "", "документ пуст"))
			return nil, p.diags
		}
		p.failure(err)
		return nil, p.diags
	}
	// Второй документ в потоке молча игнорировался бы: модель — ровно один документ.
	var extra yaml.Node
	if err := dec.Decode(&extra); err == nil {
		p.diags.Add(diag.New(diag.CodeSyntax, p.pos(&extra), "",
			"в файле больше одного документа YAML; модель должна быть одна"))
	}

	content := &root
	if content.Kind == yaml.DocumentNode {
		if len(content.Content) == 0 {
			p.diags.Add(diag.New(diag.CodeEmptyDocument, p.pos(content), "", "документ пуст"))
			return nil, p.diags
		}
		content = content.Content[0]
	}
	if content.Kind == 0 {
		p.diags.Add(diag.New(diag.CodeEmptyDocument, p.li.pos(len(src)), "", "документ пуст"))
		return nil, p.diags
	}

	node := p.convert(content)
	return node, p.diags
}

type yamlParser struct {
	li    *lineIndex
	diags diag.List
}

// pos переводит позицию узла yaml.v3 в нашу. Сканер yaml.v3 считает колонки
// в рунах — это закреплено тестом с кириллицей в yaml_test.go.
func (p *yamlParser) pos(n *yaml.Node) diag.Pos {
	return diag.Pos{Line: n.Line, Column: n.Column}
}

func (p *yamlParser) convert(n *yaml.Node) *Node {
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		p.diags.Add(diag.New(diag.CodeYAMLAnchor, p.pos(n), "",
			"якоря и алиасы YAML запрещены: выигрыша нет, а ошибиться легко"))
		if n.Kind == yaml.AliasNode {
			return &Node{Kind: Null, Pos: p.pos(n)}
		}
	}

	switch n.Kind {
	case yaml.MappingNode:
		return p.mapping(n)
	case yaml.SequenceNode:
		node := &Node{Kind: Array, Pos: p.pos(n)}
		for _, item := range n.Content {
			node.Items = append(node.Items, p.convert(item))
		}
		return node
	case yaml.ScalarNode:
		return p.scalar(n)
	default:
		p.diags.Add(diag.New(diag.CodeSyntax, p.pos(n), "", "неподдерживаемый узел YAML"))
		return &Node{Kind: Null, Pos: p.pos(n)}
	}
}

func (p *yamlParser) mapping(n *yaml.Node) *Node {
	node := &Node{Kind: Object, Pos: p.pos(n)}
	seen := make(map[string]diag.Pos, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		keyNode, valNode := n.Content[i], n.Content[i+1]
		keyPos := p.pos(keyNode)
		if keyNode.Kind != yaml.ScalarNode {
			p.diags.Add(diag.New(diag.CodeSyntax, keyPos, "",
				"ключ объекта должен быть строкой"))
			continue
		}
		key := keyNode.Value
		if first, dup := seen[key]; dup {
			p.diags.Add(diag.New(diag.CodeDuplicateKey, keyPos, "",
				"ключ «%s» уже объявлен в строке %d", key, first.Line))
		} else {
			seen[key] = keyPos
		}
		node.Fields = append(node.Fields, Field{
			Key:    key,
			KeyPos: keyPos,
			Value:  p.convert(valNode),
		})
	}
	return node
}

func (p *yamlParser) scalar(n *yaml.Node) *Node {
	pos := p.pos(n)
	switch n.Tag {
	case "!!str":
		return &Node{Kind: String, Pos: pos, Str: n.Value}
	case "!!int", "!!float":
		return &Node{Kind: Number, Pos: pos, Num: yamlNumber(n.Value)}
	case "!!bool":
		return &Node{Kind: Bool, Pos: pos, Bool: strings.EqualFold(n.Value, "true")}
	case "!!null":
		return &Node{Kind: Null, Pos: pos}
	case "!!timestamp":
		// Дата в модели — это текст: собственного типа дат у нас нет.
		return &Node{Kind: String, Pos: pos, Str: n.Value}
	default:
		p.diags.Add(diag.New(diag.CodeYAMLTag, pos, "",
			"тег YAML «%s» не поддерживается", n.Tag))
		return &Node{Kind: String, Pos: pos, Str: n.Value}
	}
}

// yamlNumber приводит запись числа YAML к виду, понятному JSON: шестнадцати-
// и восьмеричные литералы, подчёркивания и знак `+` в JSON недопустимы.
func yamlNumber(s string) json.Number {
	if i, err := strconv.ParseInt(s, 0, 64); err == nil {
		return json.Number(strconv.FormatInt(i, 10))
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return json.Number(strconv.FormatFloat(f, 'g', -1, 64))
	}
	return json.Number(s)
}

// failure переводит ошибку yaml.v3 в диагностики. Позицию приходится доставать
// из текста: структурированной ошибки разбора библиотека не отдаёт. Это
// единственное место, где мы читаем чужой текст, и наружу он не протекает —
// код диагностики остаётся типизированным.
func (p *yamlParser) failure(err error) {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	msg = strings.TrimPrefix(msg, "unmarshal errors:\n")
	for line := range strings.SplitSeq(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		text, pos := line, diag.Pos{}
		if n, rest, ok := cutLinePrefix(line); ok {
			pos = diag.Pos{Line: n, Column: 1}
			text = rest
		}
		p.diags.Add(diag.New(diag.CodeSyntax, pos, "", "ошибка синтаксиса YAML: %s", text))
	}
}

// cutLinePrefix снимает префикс вида «line 12: » и отдаёт номер строки.
func cutLinePrefix(s string) (int, string, bool) {
	const prefix = "line "
	if !strings.HasPrefix(s, prefix) {
		return 0, s, false
	}
	rest := s[len(prefix):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return 0, s, false
	}
	n, err := strconv.Atoi(rest[:colon])
	if err != nil {
		return 0, s, false
	}
	return n, strings.TrimSpace(rest[colon+1:]), true
}

// Пакет syntax — общее для JSON и YAML дерево документа с позицией у каждого
// узла. Позиции тянутся с самого разбора: восстанавливать их потом по JSON
// Pointer мучительно, а без них GUI не поставит маркеры на полях редактора.
package syntax

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

// Kind — вид узла. Набор совпадает с моделью данных JSON: YAML-фронтенд
// приводит свои скаляры к ней же.
type Kind uint8

const (
	Invalid Kind = iota
	Object
	Array
	String
	Number
	Bool
	Null
)

// String даёт русское имя вида для сообщений об ошибках.
func (k Kind) String() string {
	switch k {
	case Object:
		return "объект"
	case Array:
		return "массив"
	case String:
		return "строка"
	case Number:
		return "число"
	case Bool:
		return "логическое значение"
	case Null:
		return "null"
	default:
		return "неизвестно"
	}
}

// Field — пара «ключ-значение» объекта. Позиция ключа хранится отдельно:
// именно на ключ указывает сообщение о незнакомом поле.
type Field struct {
	Key    string
	KeyPos diag.Pos
	Value  *Node
}

// Node — узел документа.
type Node struct {
	Kind Kind
	Pos  diag.Pos // начало лексемы

	Str    string      // Kind == String
	Num    json.Number // Kind == Number
	Bool   bool        // Kind == Bool
	Items  []*Node     // Kind == Array
	Fields []Field     // Kind == Object, в порядке документа
}

// Field возвращает значение поля объекта или nil.
func (n *Node) Field(name string) *Node {
	if n == nil || n.Kind != Object {
		return nil
	}
	for i := range n.Fields {
		if n.Fields[i].Key == name {
			return n.Fields[i].Value
		}
	}
	return nil
}

// FieldKeyPos возвращает позицию ключа поля. Второй результат — false,
// если поля нет.
func (n *Node) FieldKeyPos(name string) (diag.Pos, bool) {
	if n == nil || n.Kind != Object {
		return diag.Pos{}, false
	}
	for i := range n.Fields {
		if n.Fields[i].Key == name {
			return n.Fields[i].KeyPos, true
		}
	}
	return diag.Pos{}, false
}

// Value разворачивает узел в обычное дерево Go (map, slice, string,
// json.Number, bool, nil). В таком виде документ уходит в валидатор схемы.
// При повторяющихся ключах побеждает последний, как в encoding/json;
// сам повтор ловится на разборе отдельной диагностикой.
func (n *Node) Value() any {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case Object:
		m := make(map[string]any, len(n.Fields))
		for _, f := range n.Fields {
			m[f.Key] = f.Value.Value()
		}
		return m
	case Array:
		s := make([]any, len(n.Items))
		for i, it := range n.Items {
			s[i] = it.Value()
		}
		return s
	case String:
		return n.Str
	case Number:
		return n.Num
	case Bool:
		return n.Bool
	default:
		return nil
	}
}

// Find находит узел по JSON Pointer (RFC 6901). Пустой указатель — корень.
// Возвращает nil, если пути в документе нет.
func (n *Node) Find(pointer string) *Node {
	if n == nil || pointer == "" {
		return n
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil
	}
	cur := n
	for _, raw := range strings.Split(pointer[1:], "/") {
		token := UnescapeToken(raw)
		switch {
		case cur == nil:
			return nil
		case cur.Kind == Object:
			cur = cur.Field(token)
		case cur.Kind == Array:
			i, err := strconv.Atoi(token)
			if err != nil || i < 0 || i >= len(cur.Items) {
				return nil
			}
			cur = cur.Items[i]
		default:
			return nil
		}
	}
	return cur
}

// FindKeyPos возвращает позицию самого точного места для указателя: для поля
// объекта — позицию его ключа, иначе — начало значения. Если пути нет,
// отдаётся позиция ближайшего существующего предка, чтобы маркер в редакторе
// оказался хотя бы рядом.
func (n *Node) FindKeyPos(pointer string) diag.Pos {
	if n == nil {
		return diag.Pos{}
	}
	if pointer == "" {
		return n.Pos
	}
	parent := n
	best := n.Pos
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for i, raw := range tokens {
		token := UnescapeToken(raw)
		last := i == len(tokens)-1
		switch parent.Kind {
		case Object:
			if last {
				if pos, ok := parent.FieldKeyPos(token); ok {
					return pos
				}
				return best
			}
			child := parent.Field(token)
			if child == nil {
				return best
			}
			best, parent = child.Pos, child
		case Array:
			idx, err := strconv.Atoi(token)
			if err != nil || idx < 0 || idx >= len(parent.Items) {
				return best
			}
			child := parent.Items[idx]
			if last {
				return child.Pos
			}
			best, parent = child.Pos, child
		default:
			return best
		}
	}
	return best
}

// EscapeToken экранирует сегмент JSON Pointer (RFC 6901).
func EscapeToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// UnescapeToken снимает экранирование сегмента JSON Pointer. Порядок замен
// обратный EscapeToken и по стандарту значим.
func UnescapeToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}

// Pointer собирает JSON Pointer из сегментов.
func Pointer(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range tokens {
		b.WriteByte('/')
		b.WriteString(EscapeToken(t))
	}
	return b.String()
}

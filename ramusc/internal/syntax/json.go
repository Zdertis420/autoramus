package syntax

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
)

// ParseJSON разбирает JSON в дерево с позициями.
//
// Позиция лексемы берётся так: json.Decoder сообщает смещение конца
// прочитанной лексемы (InputOffset), а начало следующей находится сдвигом
// вперёд по исходнику с пропуском пробелов и структурных символов. Документ
// целиком лежит в памяти, поэтому это дёшево и даёт точную колонку —
// в отличие от разбора в структуры, где позиция теряется.
func ParseJSON(src []byte) (*Node, diag.List) {
	p := &jsonParser{
		src: src,
		li:  newLineIndex(src),
		dec: json.NewDecoder(bytes.NewReader(src)),
	}
	p.dec.UseNumber()

	tok, pos, err := p.next()
	if err != nil {
		return nil, p.fail(err, pos)
	}
	node, err := p.value(tok, pos)
	if err != nil {
		return nil, p.fail(err, pos)
	}
	// После значения в документе не должно быть ничего, кроме пробелов.
	if _, tailPos, err := p.next(); !errors.Is(err, io.EOF) {
		if err == nil {
			p.diags.Add(diag.New(diag.CodeSyntax, tailPos, "",
				"после значения верхнего уровня идёт лишний текст"))
		} else {
			return node, p.fail(err, tailPos)
		}
	}
	return node, p.diags
}

type jsonParser struct {
	src   []byte
	li    *lineIndex
	dec   *json.Decoder
	diags diag.List
	// started — прочитана ли хоть одна лексема. Отличает пустой документ
	// от оборванного: и там и там декодер отдаёт io.EOF.
	started bool
}

// next читает следующую лексему и её позицию.
func (p *jsonParser) next() (json.Token, diag.Pos, error) {
	start := p.skip(int(p.dec.InputOffset()))
	tok, err := p.dec.Token()
	if err == nil {
		p.started = true
	}
	return tok, p.li.pos(start), err
}

// skip двигает смещение вперёд через пробелы и структурные символы,
// которые json.Decoder съедает, не сообщая о них лексемой.
func (p *jsonParser) skip(off int) int {
	for off < len(p.src) {
		switch p.src[off] {
		case ' ', '\t', '\r', '\n', ',', ':':
			off++
		default:
			return off
		}
	}
	return off
}

func (p *jsonParser) value(tok json.Token, pos diag.Pos) (*Node, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return p.object(pos)
		case '[':
			return p.array(pos)
		default:
			// Закрывающая скобка на месте значения — такого json.Decoder
			// не отдаёт, но пусть лучше будет внятная ошибка, чем паника.
			return nil, &json.SyntaxError{Offset: int64(p.dec.InputOffset())}
		}
	case string:
		return &Node{Kind: String, Pos: pos, Str: t}, nil
	case json.Number:
		return &Node{Kind: Number, Pos: pos, Num: t}, nil
	case bool:
		return &Node{Kind: Bool, Pos: pos, Bool: t}, nil
	case nil:
		return &Node{Kind: Null, Pos: pos}, nil
	default:
		return nil, &json.SyntaxError{Offset: int64(p.dec.InputOffset())}
	}
}

func (p *jsonParser) object(pos diag.Pos) (*Node, error) {
	node := &Node{Kind: Object, Pos: pos}
	seen := make(map[string]diag.Pos)
	for p.dec.More() {
		keyTok, keyPos, err := p.next()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, &json.SyntaxError{Offset: int64(p.dec.InputOffset())}
		}
		valTok, valPos, err := p.next()
		if err != nil {
			return nil, err
		}
		val, err := p.value(valTok, valPos)
		if err != nil {
			return nil, err
		}
		if first, dup := seen[key]; dup {
			p.diags.Add(diag.New(diag.CodeDuplicateKey, keyPos, "",
				"ключ «%s» уже объявлен в строке %d", key, first.Line))
		} else {
			seen[key] = keyPos
		}
		node.Fields = append(node.Fields, Field{Key: key, KeyPos: keyPos, Value: val})
	}
	if _, _, err := p.next(); err != nil { // закрывающая '}'
		return nil, err
	}
	return node, nil
}

func (p *jsonParser) array(pos diag.Pos) (*Node, error) {
	node := &Node{Kind: Array, Pos: pos}
	for p.dec.More() {
		tok, itemPos, err := p.next()
		if err != nil {
			return nil, err
		}
		item, err := p.value(tok, itemPos)
		if err != nil {
			return nil, err
		}
		node.Items = append(node.Items, item)
	}
	if _, _, err := p.next(); err != nil { // закрывающая ']'
		return nil, err
	}
	return node, nil
}

// fail переводит ошибку декодера в диагностику. У json.SyntaxError есть
// точное смещение, у прочих — только позиция последней лексемы.
func (p *jsonParser) fail(err error, pos diag.Pos) diag.List {
	var syn *json.SyntaxError
	switch {
	case errors.Is(err, io.EOF) && !p.started:
		p.diags.Add(diag.New(diag.CodeEmptyDocument, p.li.pos(len(p.src)), "",
			"документ пуст"))
	case errors.As(err, &syn):
		p.diags.Add(diag.New(diag.CodeSyntax, p.li.pos(int(syn.Offset)), "",
			"ошибка синтаксиса JSON: %s", syn.Error()))
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		// Конец файла посреди значения: скобка или кавычка не закрыта.
		p.diags.Add(diag.New(diag.CodeSyntax, p.li.pos(len(p.src)), "",
			"документ JSON оборван"))
	default:
		p.diags.Add(diag.New(diag.CodeSyntax, pos, "", "ошибка синтаксиса JSON: %s", err))
	}
	return p.diags
}

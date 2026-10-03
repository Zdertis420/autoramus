// Пакет validate проверяет документ по JSON Schema и переводит результат
// в диагностику компилятора. Позиции берутся из дерева разбора: валидатор
// схемы сообщает JSON Pointer, дерево отдаёт по нему строку и колонку.
package validate

import (
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
	"github.com/Zdertis420/autoramus/ramusc/internal/schema"
	"github.com/Zdertis420/autoramus/ramusc/internal/syntax"
)

// printer нужен только для запасного пути: у видов ошибок, которые мы не
// разобрали поимённо, текст берётся у самой библиотеки.
var printer = message.NewPrinter(language.English)

// Source проходит весь путь от текста до диагностики: разбор, проверка по
// схеме, а затем проверка смысла. Компиляция начинается с того же самого,
// поэтому двух наборов сообщений об одной и той же ошибке не возникает.
//
// Каждый следующий этап запускается только тогда, когда предыдущий не нашёл
// ошибок: валидировать нечего, а сыпать поверх синтаксической ошибки десятком
// наведённых бессмысленно. По той же причине смысловые проверки не видят
// документа с неверными типами полей — они вправе считать форму верной.
func Source(src []byte) (diag.List, error) {
	_, diags, err := Build(src)
	return diags, err
}

// Build — то же самое плюс построенная модель.
//
// Нужен компиляции: генератору требуется IR, а разбирать документ второй раз
// нельзя — получилось бы два прохода, которые однажды разойдутся. Отдельный
// вход, а не изменение Source: тот вызывается в четырёх местах, и менять его
// подпись ради одной новой ветки незачем.
//
// Модель возвращается только тогда, когда её построили: на синтаксической
// ошибке и на ошибке схемы строить нечего, и вернётся nil. Проверять надо
// диагностику, а не модель.
func Build(src []byte) (*ir.Model, diag.List, error) {
	root, diags := syntax.Load(src)
	if diags.HasErrors() {
		diags.Sort()
		return nil, diags, nil
	}
	schemaDiags, err := Document(root)
	if err != nil {
		return nil, nil, err
	}
	diags = append(diags, schemaDiags...)
	if diags.HasErrors() {
		diags.Sort()
		return nil, diags, nil
	}

	model := ir.Build(root)
	diags = append(diags, Semantic(model)...)
	if diags.HasErrors() {
		diags.Sort()
		return model, diags, nil
	}

	// Последняя ступень — раскладка и проверка геометрии
	// (specs/014-arrows-avoid-blocks). Предупреждение о стрелке сквозь блок
	// требует координат блоков, а часть из них расставляет раскладка; и оно
	// обязано быть тем же в validate и в компиляции и проверяться эталоном —
	// значит, рождается здесь, а не в команде после сборки. Лесенка
	// соблюдена: раскладка идёт только без ошибок, а ступень геометрии даёт
	// лишь предупреждения.
	//
	// Авторские сегменты запоминаются до раскладки: после неё всё, что она
	// дописала, неотличимо от написанного автором. Повторный layout.Apply в
	// компиляции ничего не меняет — раскладка идемпотентна.
	authored := make(map[*ir.Segment]bool)
	if model.Layout != nil {
		for _, a := range model.Layout.Arrows {
			for _, s := range a.Segments {
				authored[s] = true
			}
		}
	}
	layout.Apply(model)
	diags = append(diags, Geometry(model, authored)...)
	diags.Sort()
	return model, diags, nil
}

// Document проверяет документ по схеме модели.
//
// Возвращаемая ошибка означает поломку самого компилятора (не собирается
// встроенная схема) — вызывающему следует ответить кодом 2. Проблемы входного
// документа приходят списком диагностик.
func Document(root *syntax.Node) (diag.List, error) {
	if root == nil {
		return nil, nil
	}
	sch, err := schema.Model()
	if err != nil {
		return nil, err
	}
	err = sch.Validate(root.Value())
	if err == nil {
		return nil, nil
	}
	var verr *jsonschema.ValidationError
	if !asValidationError(err, &verr) {
		return nil, err
	}

	var out diag.List
	flatten(root, verr, &out)
	out.Sort()
	return out, nil
}

func asValidationError(err error, target **jsonschema.ValidationError) bool {
	v, ok := err.(*jsonschema.ValidationError)
	if ok {
		*target = v
	}
	return ok
}

// flatten разворачивает дерево ошибок валидатора в плоский список. Нас
// интересуют листья: у промежуточных узлов (allOf, $ref) текст лишь дублирует
// причину.
func flatten(root *syntax.Node, e *jsonschema.ValidationError, out *diag.List) {
	if len(e.Causes) > 0 {
		for _, cause := range e.Causes {
			flatten(root, cause, out)
		}
		return
	}
	emit(root, e, out)
}

func emit(root *syntax.Node, e *jsonschema.ValidationError, out *diag.List) {
	path := syntax.Pointer(e.InstanceLocation)
	pos := root.FindKeyPos(path)

	switch k := e.ErrorKind.(type) {
	case *kind.AdditionalProperties:
		// Ошибка отнесена к объекту целиком, но пользователю нужна позиция
		// самого поля — иначе маркер встанет на открывающую скобку.
		obj := root.Find(path)
		for _, prop := range k.Properties {
			propPath := path + "/" + syntax.EscapeToken(prop)
			propPos := pos
			if p, ok := obj.FieldKeyPos(prop); ok {
				propPos = p
			}
			out.Add(diag.New(diag.CodeSchemaUnknownField, propPos, propPath,
				"незнакомое поле «%s»", prop))
		}

	case *kind.Required:
		out.Add(diag.New(diag.CodeSchemaRequired, pos, path,
			"не хватает обязательных полей: %s", strings.Join(quoteAll(k.Missing), ", ")))

	case *kind.Type:
		out.Add(diag.New(diag.CodeSchemaType, pos, path,
			"ожидается %s, получено %s", typeNames(k.Want), typeName(k.Got)))

	case *kind.MinItems:
		if k.Want == 1 {
			out.Add(diag.New(diag.CodeSchemaMinItems, pos, path, "список не может быть пустым"))
			break
		}
		out.Add(diag.New(diag.CodeSchemaMinItems, pos, path,
			"элементов %d, а нужно не меньше %d", k.Got, k.Want))

	case *kind.MaxItems:
		out.Add(diag.New(diag.CodeSchemaMaxItems, pos, path,
			"элементов %d, а нужно не больше %d", k.Got, k.Want))

	case *kind.MinLength:
		if k.Want == 1 {
			out.Add(diag.New(diag.CodeSchemaMinLength, pos, path, "значение не может быть пустым"))
			break
		}
		out.Add(diag.New(diag.CodeSchemaMinLength, pos, path,
			"в строке %d символов, а нужно не меньше %d", k.Got, k.Want))

	case *kind.MaxLength:
		out.Add(diag.New(diag.CodeSchemaMaxLength, pos, path,
			"в строке %d символов, а нужно не больше %d", k.Got, k.Want))

	case *kind.Pattern:
		out.Add(diag.New(diag.CodeSchemaPattern, pos, path,
			"значение «%s» не подходит под образец %s", k.Got, k.Want))

	case *kind.UniqueItems:
		out.Add(diag.New(diag.CodeSchemaUniqueItems, pos, path,
			"элементы %d и %d совпадают, а повторы здесь запрещены",
			k.Duplicates[0], k.Duplicates[1]))

	case *kind.Enum:
		out.Add(diag.New(diag.CodeSchemaEnum, pos, path,
			"допустимы только значения: %s", strings.Join(quoteValues(k.Want), ", ")))

	case *kind.Const:
		out.Add(diag.New(diag.CodeSchemaConst, pos, path,
			"допустимо только значение %s", quoteValue(k.Want)))

	case *kind.MinProperties:
		out.Add(diag.New(diag.CodeSchemaMinProperties, pos, path,
			"полей %d, а нужно не меньше %d", k.Got, k.Want))

	case *kind.MaxProperties:
		out.Add(diag.New(diag.CodeSchemaMaxProperties, pos, path,
			"полей %d, а нужно не больше %d", k.Got, k.Want))

	case *kind.PropertyNames:
		out.Add(diag.New(diag.CodeSchemaPropertyNames, pos, path,
			"недопустимое имя поля «%s»", k.Property))

	case *kind.FalseSchema:
		out.Add(diag.New(diag.CodeSchemaNotAllowedHere, pos, path,
			"значение здесь недопустимо"))

	default:
		out.Add(diag.New(diag.CodeSchema, pos, path, "%s", e.ErrorKind.LocalizedString(printer)))
	}
}

// typeName переводит имя типа JSON Schema на русский.
func typeName(t string) string {
	switch t {
	case "object":
		return "объект"
	case "array":
		return "массив"
	case "string":
		return "строка"
	case "number":
		return "число"
	case "integer":
		return "целое число"
	case "boolean":
		return "логическое значение"
	case "null":
		return "null"
	default:
		return t
	}
}

func typeNames(ts []string) string {
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = typeName(t)
	}
	return strings.Join(names, " или ")
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = "«" + s + "»"
	}
	return out
}

func quoteValue(v any) string {
	if s, ok := v.(string); ok {
		return "«" + s + "»"
	}
	return printer.Sprintf("%v", v)
}

func quoteValues(vs []any) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = quoteValue(v)
	}
	return out
}

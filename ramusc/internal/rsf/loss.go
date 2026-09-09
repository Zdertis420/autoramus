package rsf

import (
	"fmt"
	"sort"
	"strings"
)

// Потери — то, что лежит в файле и не перенесено во входной язык.
//
// Вычисляются вычитанием: опись атрибутов минус перечень перенесённого. Прежний
// подход — перечислять «чего мы не умеем» — верен ровно до появления
// незнакомого атрибута: всё, о чём не догадались спросить, проходит молча.
// Именно так пропадали классификаторы.

// Loss — потеря: атрибут, чьё значение не доехало до документа.
type Loss struct {
	// Attribute — имя атрибута Ramus, напр. F_STATUS.
	Attribute string
	// Kind — вид элемента-владельца.
	Kind ElementKind
	// Columns — какие именно колонки не перенесены. У одноколоночного
	// атрибута — одна; у прочитанного частично — только непрочитанные.
	Columns []string
	// Count — сколько записей потеряно. Для большинства атрибутов запись одна
	// на элемент, но не для всех: у точек ломаной запись на каждую точку,
	// поэтому число здесь больше числа элементов и означает именно записи.
	Count int
}

func (l Loss) String() string {
	head := fmt.Sprintf("%s (%s)", l.Attribute, l.Kind)
	if len(l.Columns) > 0 {
		head += ": " + strings.Join(l.Columns, ", ")
	}
	return fmt.Sprintf("%s — %d", head, l.Count)
}

// transferred — что и в какой мере уже переносится во входной язык.
//
// Значение — список колонок, которые читаются. Пустой список означает, что
// атрибут переносится целиком, каким бы ни был состав его колонок.
//
// Список держится рядом с вычислением потерь намеренно: научившись читать новый
// атрибут, его вычёркивают из потерь здесь же, одной правкой. Разнеси эти два
// знания по разным файлам — и они разойдутся.
var transferred = map[string][]string{
	// Имя работы и имя потока.
	"Название":      nil,
	"Name":          nil,
	"F_STREAM_NAME": nil,
	// Иерархия работ: родитель.
	"HierarchicalAttribute": nil,
	// Геометрия: уезжает в секцию раскладки.
	"F_BOUNDS":              nil,
	"F_SECTOR_POINTS":       nil,
	"F_SECTOR_BORDER_START": nil,
	"F_SECTOR_BORDER_END":   nil,
	// Принадлежность сектора: чья диаграмма и какой поток.
	"F_FUNCTION_SECTOR": nil,
	"F_SECTOR_STREAM":   nil,
	// Вид элемента. Элементы DFD сняты с объёма решением по фиче 001, но сам
	// атрибут читается, и потерей он не является.
	"F_TYPE": nil,
	// Указатель на квалификатор работ: по нему находится сама модель
	// (findModel). Данными автора не является и в документе не нужен.
	"F_BASE_FUNCTION_QUALIFIER_ID": nil,
	// Запись о модели читается частично: отсюда берутся автор и лист, всё
	// остальное теряется. Поэтому колонки перечислены поимённо — иначе запись
	// целиком сошла бы за перенесённую и семь полей исчезли бы молча.
	"F_PROJECT_PREFERENCES": {"PROJECT_AUTOR", "DIAGRAM_SIZE"},
}

// Losses собирает перечень потерь по всей модели: что лежит в файле и не
// переносится полями языка.
func (m *Model) Losses() []Loss { return m.losses(nil, false) }

// LossesExcept — то, что не доехало до документа вовсе.
//
// Отличается от Losses не только вычитанием унесённого люком. У элемента,
// которого в документе нет совсем — например, у сектора без потока: язык не
// умеет описать стрелку, не привязанную к потоку, — теряются все атрибуты, а не
// только те, что не выражаются полями. Его геометрия перечислена в transferred,
// но переносить её оказалось некуда.
func (m *Model) LossesExcept(carried map[int64]bool) []Loss {
	return m.losses(carried, carried != nil)
}

func (m *Model) losses(carried map[int64]bool, whole bool) []Loss {
	// Свёртка идёт по паре «атрибут + вид элемента», а колонки объединяются.
	// Разный состав колонок у разных элементов — обычное дело: часть значений
	// пуста. Разводить такие случаи по отдельным записям значило бы вернуть
	// нечитаемый перечень, ради борьбы с которым свёртка и заведена.
	type key struct {
		attribute string
		kind      ElementKind
	}
	counts := make(map[key]int)
	columns := make(map[key]map[string]bool)

	for element, kind := range m.TransferableElements() {
		if carried[element] {
			continue
		}
		for _, a := range m.Attributes(element) {
			lost := lostColumns(a)
			if whole {
				// Элемента в документе нет — унести его атрибуты было некуда,
				// какими бы полями язык их ни выражал.
				lost = allColumns(a)
			}
			if len(lost) == 0 {
				continue
			}
			k := key{a.Name, kind}
			counts[k]++
			if columns[k] == nil {
				columns[k] = make(map[string]bool)
			}
			for _, c := range lost {
				columns[k][c] = true
			}
		}
	}

	out := make([]Loss, 0, len(counts))
	for k, n := range counts {
		names := make([]string, 0, len(columns[k]))
		for c := range columns[k] {
			names = append(names, c)
		}
		sort.Strings(names)
		out = append(out, Loss{Attribute: k.attribute, Kind: k.kind, Columns: names, Count: n})
	}

	// Свёртка уже произошла выше: одинаковые потери сложились в одну запись.
	// Здесь остаётся задать порядок — вывод обязан быть детерминированным.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Attribute != out[j].Attribute {
			return out[i].Attribute < out[j].Attribute
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// lostColumns отдаёт колонки атрибута, которые не переносятся. Пусто — атрибут
// перенесён целиком.
func lostColumns(a Attribute) []string {
	read, known := transferred[a.Name]
	if known && read == nil {
		return nil // переносится целиком
	}

	taken := make(map[string]bool, len(read))
	for _, c := range read {
		taken[strings.ToUpper(c)] = true
	}

	var out []string
	for name := range a.Columns {
		if !taken[strings.ToUpper(name)] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// LostColumns отдаёт колонки атрибута, которые не переносятся полями языка.
// Пусто — атрибут перенесён целиком, и в люк ему не место.
//
// Открыта наружу ради декомпилятора: он складывает в люк ровно то, что здесь
// признано потерянным. Иначе знание «что переносится» пришлось бы держать в
// двух местах, и они бы разошлись.
func LostColumns(a Attribute) []string { return lostColumns(a) }

// allColumns отдаёт все колонки значения: у элемента, не попавшего в документ,
// потеряно всё.
func allColumns(a Attribute) []string {
	out := make([]string, 0, len(a.Columns))
	for name := range a.Columns {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

package rsf

import (
	"fmt"
	"sort"
	"strings"
)

// Обнаружение того, что входным языком не выражается.
//
// Проход идёт по таблицам файла и не смотрит на результат разбора. Это
// принципиально: проверка вида «разбор ничего не положил в поле, значит в
// файле этого нет» верна ровно до тех пор, пока разбор что-то умеет, и молча
// пропускает всё, чего он не умеет вовсе. Именно так терялись классификаторы.

// UnsupportedKind — вид находки.
type UnsupportedKind int

const (
	// UnsupportedUnnamedFunction — работа без имени. Имя работы и есть её
	// идентификатор (Р4), пустых быть не может.
	UnsupportedUnnamedFunction UnsupportedKind = iota
	// UnsupportedDuplicateFunction — две работы с одинаковым именем. Р4
	// требует уникальности имён в пределах документа.
	UnsupportedDuplicateFunction
	// UnsupportedColumnType — тип колонки справочника, которому во входном
	// языке нет соответствия.
	UnsupportedColumnType
	// UnsupportedUnnamedArrow — стрелка без имени. Ramus позволяет нарисовать
	// стрелку и не дать ей имени, но идентификатор потока и есть его имя (Р4):
	// сослаться на такую стрелку нечем ни в flows, ни в ICOM-полях работ, ни
	// в layout (Р18). Виды только добавляются в конец: GUI ветвится по ним.
	UnsupportedUnnamedArrow
)

// Unsupported — находка: что именно не выражается, сколько таких случаев и,
// если применимо, где.
type Unsupported struct {
	Kind  UnsupportedKind
	Count int
	// Where — уточнение места: имя работы, справочника, колонки. Пусто, если
	// находка не привязана к одному месту.
	Where string
}

func (u Unsupported) String() string {
	var what string
	switch u.Kind {
	case UnsupportedUnnamedFunction:
		what = "работ без имени"
	case UnsupportedDuplicateFunction:
		what = "работ с повторяющимся именем"
	case UnsupportedColumnType:
		what = "колонок неизвестного типа"
	case UnsupportedUnnamedArrow:
		what = "стрелок без имени"
	default:
		what = "непонятных находок"
	}
	if u.Where != "" {
		return fmt.Sprintf("%s: %d (%s)", what, u.Count, u.Where)
	}
	return fmt.Sprintf("%s: %d", what, u.Count)
}

// Unsupported собирает всё, что файл содержит, а входной язык выразить не
// может. Проход не прекращается на первой находке: автор должен узнать обо
// всех сразу, иначе исправление превращается в перебор по одному за запуск.
func (m *Model) Unsupported() []Unsupported {
	var out []Unsupported

	if u, ok := m.unnamedFunctions(); ok {
		out = append(out, u)
	}
	out = append(out, m.duplicateFunctions()...)
	out = append(out, m.unknownColumnTypes()...)
	out = append(out, m.unnamedArrows()...)

	return out
}

// unnamedFunctions считает работы без имени.
func (m *Model) unnamedFunctions() (Unsupported, bool) {
	var n int
	for _, f := range m.Functions() {
		if strings.TrimSpace(f.Name) == "" {
			n++
		}
	}
	if n == 0 {
		return Unsupported{}, false
	}
	return Unsupported{Kind: UnsupportedUnnamedFunction, Count: n}, true
}

// duplicateFunctions находит повторяющиеся имена работ. Безымянные не
// считаются: о них уже сказано отдельно, и складывать их в дубли значило бы
// сообщить об одном и том же дважды.
func (m *Model) duplicateFunctions() []Unsupported {
	seen := make(map[string]int)
	for _, f := range m.Functions() {
		name := strings.Join(strings.Fields(f.Name), " ")
		if name == "" {
			continue
		}
		seen[name]++
	}

	var out []Unsupported
	for name, n := range seen {
		if n > 1 {
			out = append(out, Unsupported{
				Kind:  UnsupportedDuplicateFunction,
				Count: n,
				Where: fmt.Sprintf("«%s»", name),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Where < out[j].Where })
	return out
}

// unknownColumnTypes находит колонки справочников, значения которых лежат
// в таблице, о которой мы не знаем.
func (m *Model) unknownColumnTypes() []Unsupported {
	var out []Unsupported
	for _, c := range m.Classifiers() {
		for _, col := range c.Columns {
			if _, ok := valueTables[col.Type]; ok {
				continue
			}
			out = append(out, Unsupported{
				Kind:  UnsupportedColumnType,
				Count: 1,
				Where: fmt.Sprintf("справочник «%s», колонка «%s» типа %s", c.Name, col.Name, col.Type),
			})
		}
	}
	return out
}

// unnamedArrows находит стрелки, у которых нет имени.
//
// Считаются стрелки, а не сегменты: одна стрелка лежит в файле несколькими
// секторами — по одному на каждую диаграмму, где она нарисована, плюс по
// одному на каждую ветвь. Сообщить автору «шесть» вместо «одна» значило бы
// отправить его искать шесть мест там, где место одно.
func (m *Model) unnamedArrows() []Unsupported {
	streamless := m.streamlessSectors()
	if len(streamless) == 0 {
		return nil
	}

	names := make(map[int64]string)
	for _, f := range m.Functions() {
		names[f.ID] = f.Name
	}

	var out []Unsupported
	for _, arrow := range groupByCrosspoint(streamless) {
		out = append(out, Unsupported{
			Kind:  UnsupportedUnnamedArrow,
			Count: 1,
			Where: describeArrow(arrow, names),
		})
	}
	return out
}

// streamlessSectors отдаёт секторы, не связанные ни с одним потоком.
//
// Признак — отсутствие строки F_SECTOR_STREAM в таблице, а не Sector.Stream
// со значением -1: в поле -1 означает и «строки нет», и «строка есть, а
// значение не разобралось». Разница на сегодняшних файлах не видна, и именно
// поэтому её легко потерять — а с ней потерялась бы и стрелка на файле,
// которого мы ещё не видели.
func (m *Model) streamlessSectors() []Sector {
	attribute := m.attributes["F_SECTOR_STREAM"]

	var out []Sector
	for _, s := range m.Sectors() {
		if _, ok := m.others.First(Eq("ELEMENT_ID", fmt.Sprint(s.ID)), Eq("ATTRIBUTE_ID", attribute)); ok {
			continue
		}
		out = append(out, s)
	}
	return out
}

// groupByCrosspoint собирает секторы в стрелки: два сектора принадлежат одной
// стрелке, если делят кросспоинт. Кросспоинт связывает и ветви на одной
// диаграмме, и сегмент с его продолжением на диаграмме декомпозиции соседнего
// уровня — для сборки эти случаи неразличимы и различать их не нужно.
//
// Сектор без кросспоинтов образует стрелку из себя одного.
func groupByCrosspoint(sectors []Sector) [][]Sector {
	// Объединение непересекающихся множеств: связь уже записана в файле
	// явно, искать её геометрией нечего.
	parent := make([]int, len(sectors))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	union := func(a, b int) {
		if ra, rb := find(a), find(b); ra != rb {
			parent[rb] = ra
		}
	}

	seen := make(map[int64]int)
	for i, s := range sectors {
		for _, b := range []*Border{s.Start, s.End} {
			if b == nil || b.Crosspoint < 0 {
				continue
			}
			if j, ok := seen[b.Crosspoint]; ok {
				union(i, j)
			} else {
				seen[b.Crosspoint] = i
			}
		}
	}

	groups := make(map[int][]Sector)
	for i, s := range sectors {
		root := find(i)
		groups[root] = append(groups[root], s)
	}

	// Порядок задаётся явно: находки печатаются автору и сравниваются диффом,
	// а обход map в Go случаен от запуска к запуску.
	out := make([][]Sector, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g, func(i, j int) bool { return g[i].ID < g[j].ID })
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0].ID < out[j][0].ID })
	return out
}

// describeArrow называет места в терминах модели: работы и стороны блоков.
// Автор ищет стрелку глазами в Ramus, и идентификаторы секторов ему в этом не
// помогают — прежняя сводка потерь говорила ровно ими и была бесполезна.
//
// Концы, не прицепленные к работам, в описание не идут: у стрелки, перешедшей
// на диаграмму декомпозиции, таких концов больше, чем осмысленных, и они
// только зашумили бы строку. Если не прицеплен ни один — так и сказано.
func describeArrow(sectors []Sector, names map[int64]string) string {
	var sources, targets []string
	seen := make(map[string]bool)

	for _, s := range sectors {
		for _, b := range []*Border{s.Start, s.End} {
			if b == nil || b.Function < 0 {
				continue
			}
			// У работы может не быть имени — тогда она и сама непереносима,
			// и о ней уже сказано отдельной находкой. Пустые кавычки в этом
			// месте выглядели бы опечаткой, а не положением дел.
			name := names[b.Function]
			if strings.TrimSpace(name) == "" {
				name = "работа без имени"
			}
			text := fmt.Sprintf("«%s» (%s)", name, sideWord(b.FunctionType))
			if seen[text] {
				continue
			}
			seen[text] = true
			if b.FunctionType == SideRight {
				sources = append(sources, text)
			} else {
				targets = append(targets, text)
			}
		}
	}

	switch {
	case len(sources) == 0 && len(targets) == 0:
		return "оба конца не присоединены"
	case len(sources) == 0:
		return "→ " + strings.Join(targets, ", ")
	case len(targets) == 0:
		return strings.Join(sources, ", ") + " →"
	default:
		return strings.Join(sources, ", ") + " → " + strings.Join(targets, ", ")
	}
}

// sideWord — сторона блока словом. Side.String печатает «вход(L)» с буквой
// ICOM: она нужна в dump, где рядом стоят сырые числа файла, но в сообщении
// автору лишняя.
func sideWord(s Side) string {
	switch s {
	case SideLeft:
		return "вход"
	case SideTop:
		return "управление"
	case SideBottom:
		return "механизм"
	case SideRight:
		return "выход"
	default:
		return "неизвестная сторона"
	}
}

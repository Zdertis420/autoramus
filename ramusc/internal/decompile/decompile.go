// Пакет decompile переводит готовый файл Ramus в IR, а оттуда — в документ
// на входном языке (Р14). Любая существующая модель становится тест-кейсом
// для валидатора и образцом для нейросети, а заодно спецификацией для
// генератора: всё, что читается здесь, генератор обязан уметь записать.
package decompile

import (
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Model переводит модель из файла в IR.
//
// Пути (JSON Pointer) проставляются такими, какими они будут в напечатанном
// документе: позиций в исходнике у декомпилированной модели нет, но путь
// остаётся осмысленным, и диагностика валидатора на ней не слепая.
func Model(m *rsf.Model) (*ir.Model, error) {
	functions := m.Functions()
	if len(functions) == 0 {
		return nil, fmt.Errorf("в модели нет ни одной работы")
	}

	byID := make(map[int64]rsf.Function, len(functions))
	for _, f := range functions {
		byID[f.ID] = f
	}
	root, ok := findRoot(functions, m.Element)
	if !ok {
		return nil, fmt.Errorf("не нашлась корневая работа под элементом модели %d", m.Element)
	}

	out := &ir.Model{Name: ref(root.Name, "/model")}
	author, page := preferences(m)
	out.Author, out.Page = author, page

	for i, s := range m.Streams() {
		out.Flows = append(out.Flows, ref(s.Name, fmt.Sprintf("/flows/%d", i)))
	}

	ordered := treeOrder(functions, root)
	icom := m.ICOM()
	for i, f := range ordered {
		path := fmt.Sprintf("/functions/%d", i)
		fn := &ir.Function{
			Name: ref(f.Name, path+"/name"),
			Path: path,
		}
		if f.ID != root.ID {
			fn.Of = ref(byID[f.Parent].Name, path+"/of")
		}
		if t := elementType(f.Type); t != "" {
			fn.Type = ref(t, path+"/type")
		}
		out.Functions = append(out.Functions, fn)
		out.Links = append(out.Links, links(icom[f.ID], fn.Name, path)...)
	}

	out.Layout = layout(m, ordered, byID, root)
	out.Index()
	return out, nil
}

// findRoot ищет работу, лежащую прямо под элементом модели: это блок
// контекстной диаграммы.
func findRoot(functions []rsf.Function, modelElement int64) (rsf.Function, bool) {
	for _, f := range functions {
		if f.Parent == modelElement {
			return f, true
		}
	}
	return rsf.Function{}, false
}

// treeOrder раскладывает работы так, как они идут в дереве Ramus: корень,
// затем дети по цепочке PREVIOUS_ELEMENT_ID. Порядком строк в таблице
// пользоваться нельзя — он ничего не значит (RSF-FORMAT.md §3).
func treeOrder(functions []rsf.Function, root rsf.Function) []rsf.Function {
	children := make(map[int64][]rsf.Function)
	for _, f := range functions {
		children[f.Parent] = append(children[f.Parent], f)
	}

	var walk func(f rsf.Function) []rsf.Function
	walk = func(f rsf.Function) []rsf.Function {
		out := []rsf.Function{f}
		for _, child := range siblingOrder(children[f.ID]) {
			out = append(out, walk(child)...)
		}
		return out
	}
	return walk(root)
}

// siblingOrder выстраивает братьев по односвязному списку PREVIOUS. Если
// цепочка порвана, остаток дописывается как есть: терять работы нельзя.
func siblingOrder(siblings []rsf.Function) []rsf.Function {
	next := make(map[int64]rsf.Function, len(siblings))
	var first *rsf.Function
	for i, f := range siblings {
		if f.Previous == -1 {
			first = &siblings[i]
			continue
		}
		next[f.Previous] = f
	}

	var out []rsf.Function
	seen := make(map[int64]bool, len(siblings))
	for cur := first; cur != nil; {
		out = append(out, *cur)
		seen[cur.ID] = true
		following, ok := next[cur.ID]
		if !ok || seen[following.ID] {
			break
		}
		cur = &following
	}
	for _, f := range siblings {
		if !seen[f.ID] {
			out = append(out, f)
		}
	}
	return out
}

// links раскладывает ICOM работы в связи IR. Sugar означает, что печатать их
// надо списками in/control/out/mechanism, а не секцией links (Р7).
func links(sides *rsf.ICOM, name ir.Ref, path string) []*ir.Link {
	if sides == nil {
		return nil
	}
	var out []*ir.Link
	for _, side := range []struct {
		key   string
		flows []string
	}{
		{ir.SideIn, sides.In},
		{ir.SideControl, sides.Control},
		{ir.SideMechanism, sides.Mechanism},
	} {
		for i, flow := range side.flows {
			out = append(out, &ir.Link{
				Flow:  ref(flow, fmt.Sprintf("%s/%s/%d", path, side.key, i)),
				To:    name,
				Side:  side.key,
				Sugar: true,
				Path:  fmt.Sprintf("%s/%s/%d", path, side.key, i),
			})
		}
	}
	for i, flow := range sides.Out {
		out = append(out, &ir.Link{
			Flow:  ref(flow, fmt.Sprintf("%s/out/%d", path, i)),
			From:  name,
			Sugar: true,
			Path:  fmt.Sprintf("%s/out/%d", path, i),
		})
	}
	return out
}

// layout переносит геометрию: координаты всех блоков и те сегменты, чьи оба
// конца выразимы нынешним форматом. Ломаные, упирающиеся в узлы ветвления,
// записать нечем — сколько их, печатает шапка документа.
func layout(m *rsf.Model, ordered []rsf.Function, byID map[int64]rsf.Function, root rsf.Function) *ir.Layout {
	out := &ir.Layout{Path: "/layout"}

	for _, f := range ordered {
		if f.Bounds == nil {
			continue
		}
		path := fmt.Sprintf("/layout/functions/%d", len(out.Functions))
		out.Functions = append(out.Functions, &ir.FunctionLayout{
			Function: ref(f.Name, path+"/function"),
			X:        num(f.Bounds.X, path+"/x"),
			Y:        num(f.Bounds.Y, path+"/y"),
			Width:    num(f.Bounds.Width, path+"/width"),
			Height:   num(f.Bounds.Height, path+"/height"),
			Path:     path,
		})
	}

	streams := make(map[int64]string)
	for _, s := range m.Streams() {
		streams[s.ID] = s.Name
	}

	for _, sector := range m.Sectors() {
		if !expressible(sector) {
			continue
		}
		path := fmt.Sprintf("/layout/arrows/%d", len(out.Arrows))
		arrow := &ir.ArrowLayout{
			Flow: ref(streams[sector.Stream], path+"/flow"),
			On:   ref(root.Name, path+"/on"),
			Path: path,
		}
		if sector.Diagram == m.Element {
			// Контекстная диаграмма зовётся так же, как корневая работа,
			// и отличается только этой пометкой.
			arrow.Context = true
		} else {
			arrow.On = ref(byID[sector.Diagram].Name, path+"/on")
		}
		if sector.Start.OnFunction() {
			arrow.From = ref(byID[sector.Start.Function].Name, path+"/from")
		}
		if sector.End.OnFunction() {
			arrow.To = ref(byID[sector.End.Function].Name, path+"/to")
		}
		for i, p := range sector.Points {
			point := fmt.Sprintf("%s/points/%d", path, i)
			arrow.Points = append(arrow.Points, ir.Point{
				X:    num(p.X, point+"/0"),
				Y:    num(p.Y, point+"/1"),
				Path: point,
			})
		}
		out.Arrows = append(out.Arrows, arrow)
	}
	return out
}

// expressible сообщает, влезает ли сегмент в нынешний layout.arrows: концы
// в узлах ветвления и висящие концы записать нечем.
func expressible(s rsf.Sector) bool {
	for _, end := range []*rsf.Border{s.Start, s.End} {
		if !end.OnFunction() && !end.OnBorder() {
			return false
		}
	}
	return len(s.Points) > 0
}

// OrphanFlows перечисляет потоки, у которых в модели нет ни одного сегмента.
// Такие остаются в файле после переименования стрелок: Ramus заводит новый
// поток, а старый не удаляет, только отцепляет от него стрелки. В `flows` они
// попадают — вывод описывает файл как есть, — и валидатор честно даёт на них
// unused_flow.
func OrphanFlows(m *rsf.Model) []string {
	used := make(map[int64]bool)
	for _, s := range m.Sectors() {
		used[s.Stream] = true
	}
	var out []string
	for _, s := range m.Streams() {
		if !used[s.ID] {
			out = append(out, s.Name)
		}
	}
	return out
}

// Skipped считает сегменты, которые в документ не попали.
func Skipped(m *rsf.Model) (skipped, total int) {
	sectors := m.Sectors()
	for _, s := range sectors {
		if !expressible(s) {
			skipped++
		}
	}
	return skipped, len(sectors)
}

// elementType переводит F_TYPE в вид элемента входного языка. Всё, что не
// элемент DFD, — обычная работа, и поле не печатается вовсе.
func elementType(t int) string {
	switch t {
	case rsf.TypeExternalReference:
		return ir.TypeExternal
	case rsf.TypeDataStore:
		return ir.TypeStore
	case rsf.TypeDFDSRole:
		return ir.TypeRole
	default:
		return ""
	}
}

// preferences достаёт автора и размер листа из настроек модели.
func preferences(m *rsf.Model) (author, page string) {
	prefs, err := m.File.Table("attribute_model_preferences")
	if err != nil {
		return "", ""
	}
	row, ok := prefs.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(m.Element)))
	if !ok {
		return "", ""
	}
	return prefs.Str(row, "PROJECT_AUTOR"), prefs.Str(row, "DIAGRAM_SIZE")
}

func ref(name, path string) ir.Ref {
	return ir.Ref{Name: ir.Normalize(name), Raw: name, Path: path}
}

func num(v float64, path string) ir.Num {
	return ir.Num{Val: v, Set: true, Path: path}
}

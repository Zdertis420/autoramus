// Пакет layout раздаёт координаты работам, которым их не задал автор.
//
// На входе модель без геометрии, на выходе — она же с геометрией. Про ZIP, XML
// и формат .rsf не знает ничего: за счёт этого раскладку можно заменить, не
// трогая генератор, и наоборот (Р16).
//
// Секция layout остаётся оверрайдом (Р2): заданное автором не трогается ни в
// значении, ни в положении в списке. Раскладка дописывает только недостающее.
package layout

import (
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// Apply раздаёт координаты работам, у которых их нет.
//
// Модель приходит уже проверенной: конвейер идёт лесенкой, и раскладка
// запускается только после валидатора. Поэтому здесь не проверяется ни форма
// документа, ни осмысленность имён — ошибок автора раскладка не порождает.
func Apply(m *ir.Model) {
	if m == nil {
		return
	}
	if m.Layout == nil {
		m.Layout = &ir.Layout{Path: "/layout"}
	}

	known := make(map[string]bool, len(m.Layout.Functions))
	for _, box := range m.Layout.Functions {
		known[box.Function.Name] = true
	}

	for _, diagram := range diagrams(m) {
		ordered := sortByFlow(diagram.children, edges(m, diagram.siblings))
		for i, place := range place(len(ordered)) {
			name := ordered[i]
			if known[name] {
				// Автор задал сам — не трогаем ни значение, ни порядок записи.
				continue
			}
			path := fmt.Sprintf("/layout/functions/%d", len(m.Layout.Functions))
			m.Layout.Functions = append(m.Layout.Functions, &ir.FunctionLayout{
				Function: ir.Ref{Name: name, Path: path + "/function"},
				X:        ir.Num{Val: place.x, Set: true, Path: path + "/x"},
				Y:        ir.Num{Val: place.y, Set: true, Path: path + "/y"},
				Width:    ir.Num{Val: place.width, Set: true, Path: path + "/width"},
				Height:   ir.Num{Val: place.height, Set: true, Path: path + "/height"},
				Path:     path,
			})
		}
	}

	// Третьим проходом — стрелки. Он идёт после блоков, и не может иначе:
	// маршрут считается по фактическим прямоугольникам, а часть из них
	// появилась только что.
	placeArrows(m)
}

// diagram — единица раскладки: работа вместе с её непосредственными детьми.
// У каждой своя диагональ и свой порядок; дети разных родителей друг о друге
// не знают.
type diagram struct {
	// owner — чья это диаграмма. Пусто у контекстной A-0: её владелец —
	// сама модель, а корневая работа лежит на ней единственным блоком.
	owner string
	// children — имена детей в порядке объявления.
	children []string
	// siblings — те же имена отображением: связи между чужими работами рёбер
	// этой диаграммы не дают.
	siblings map[string]bool
}

// diagrams разбивает модель на диаграммы.
//
// Корневая работа лежит на своей диаграмме одна — это контекстная A-0. Работа
// без детей диаграммы не образует: раскладывать нечего.
//
// Порядок диаграмм — порядок объявления их владельцев: обход отображения дал бы
// разные результаты от запуска к запуску.
func diagrams(m *ir.Model) []diagram {
	children := make(map[string][]string)
	var owners []string
	seen := make(map[string]bool)

	for _, f := range m.Functions {
		owner := f.Of.Name
		if !seen[owner] {
			seen[owner] = true
			owners = append(owners, owner)
		}
		children[owner] = append(children[owner], f.Name.Name)
	}

	out := make([]diagram, 0, len(owners))
	for _, owner := range owners {
		names := children[owner]
		siblings := make(map[string]bool, len(names))
		for _, name := range names {
			siblings[name] = true
		}
		out = append(out, diagram{owner: owner, children: names, siblings: siblings})
	}
	return out
}

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

	known := make(map[string]*ir.FunctionLayout, len(m.Layout.Functions))
	for _, box := range m.Layout.Functions {
		known[box.Function.Name] = box
	}

	// Сколько стрелок у каждой стороны каждого блока — до того, как блоки
	// встали: размер блока от этого числа и зависит. Сами стрелки выводятся из
	// связей документа без единой координаты, поэтому считать их можно раньше,
	// чем прокладывать; прокладываются они третьим проходом, ниже.
	needs := demand(arrows(m))

	for _, diagram := range diagrams(m) {
		ordered := sortByFlow(diagram.children, edges(m, diagram.siblings))
		wants := make([]need, len(ordered))
		for i, name := range ordered {
			wants[i] = needs[blockKey{diagram.owner, name}]
		}
		placed := place(wants)

		// Авторские блоки — первыми: они не двигаются, и авто-блоки обязаны
		// их обойти, а для этого надо знать, где они стоят.
		var taken []box
		for i, spot := range placed {
			authored := known[ordered[i]]
			if authored == nil {
				continue
			}
			// Автор задал сам — не трогаем ни значение, ни порядок записи.
			//
			// Кроме размера, которого он не задавал: схема разрешает
			// положение без ширины и высоты, и такой блок уходил в файл
			// нулевым. Это не решение автора, а пропуск, и раскладка
			// дописывает недостающее — то, что сказано, остаётся как есть.
			if !authored.Width.Set {
				authored.Width = ir.Num{Val: spot.width, Set: true, Path: authored.Path + "/width"}
			}
			if !authored.Height.Set {
				authored.Height = ir.Num{Val: spot.height, Set: true, Path: authored.Path + "/height"}
			}
			taken = append(taken, box{x: authored.X.Val, y: authored.Y.Val,
				width: authored.Width.Val, height: authored.Height.Val})
		}

		for i, spot := range placed {
			name := ordered[i]
			if known[name] != nil {
				continue
			}
			spot = clear(spot, taken)
			taken = append(taken, spot)
			path := fmt.Sprintf("/layout/functions/%d", len(m.Layout.Functions))
			m.Layout.Functions = append(m.Layout.Functions, &ir.FunctionLayout{
				Function: ir.Ref{Name: name, Path: path + "/function"},
				X:        ir.Num{Val: spot.x, Set: true, Path: path + "/x"},
				Y:        ir.Num{Val: spot.y, Set: true, Path: path + "/y"},
				Width:    ir.Num{Val: spot.width, Set: true, Path: path + "/width"},
				Height:   ir.Num{Val: spot.height, Set: true, Path: path + "/height"},
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

// clear сдвигает авто-блок с занятых мест (specs/014-arrows-avoid-blocks,
// FR-010).
//
// Лестница расставляется так, будто авторских блоков нет, и ступень может
// прийтись на авторский блок: в skirt-full «Добавление фурнитуры» легло на
// «Сшивание деталей», и стрелкам между ними пройти было негде. Налезает —
// вправо за помеху на blockClearance; вправо не помещается на лист — вниз за
// неё. Блоку, на который ничто не налезает, путь не нужен: он остаётся на
// своей ступени, и документы без авторских блоков не меняются.
//
// Координаты только растут, поэтому сдвиги не зацикливаются; и всё же их
// число ограничено числом помех. Не нашлось места внутри листа — блок стоит
// на своей ступени: авторские блоки заняли всё, и это названная граница, как
// у пересекающихся авторских блоков (FR-005).
func clear(spot box, taken []box) box {
	moved := spot
	for range len(taken) + 1 {
		hit, found := firstOverlap(moved, taken)
		if !found {
			if moved.x+moved.width <= sheetRight && moved.y+moved.height <= sheetBottom {
				return moved
			}
			return spot
		}
		if right := hit.x + hit.width + blockClearance; right+moved.width <= sheetRight {
			moved.x = right
		} else {
			moved.y = hit.y + hit.height + blockClearance
		}
	}
	return spot
}

// firstOverlap — первая по списку помеха, на которую налезает блок. Касание
// сторонами налезанием не считается.
func firstOverlap(b box, taken []box) (box, bool) {
	for _, t := range taken {
		if b.x < t.x+t.width && t.x < b.x+b.width && b.y < t.y+t.height && t.y < b.y+b.height {
			return t, true
		}
	}
	return box{}, false
}

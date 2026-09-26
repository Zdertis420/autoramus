package generate

import (
	"fmt"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// Оформление стрелки: значения сняты с настоящих моделей, как и оформление
// работы. Во всех трёх файлах набора у всех секторов с потоком стоит одна и та
// же строка VISUAL_ATTRIBUTES — 140 знаков, ни одного исключения.
const (
	sectorVisual    = "8180808080808060BF82808080808080808080808080808080808080808080A4C07F7F7F7F808186808080C4E9E1ECEFE7888080808080808081808080808080808080808080"
	sectorAlignment = "0"
	sectorCreatePos = "0.0"
	sectorCreateSt  = "-1"
	sectorTilda     = "0"
	sectorTildaPos  = "0.0"
	sectorShowText  = "1"

	// Высота строки подписи при шрифте Dialog 8. В файлах она кратна числу
	// строк: 9.8 — одна, 19.6 — две, 39.2 — четыре.
	sectorLineHeight = 9.80078125
)

// Подпись несёт не всякий сектор, а один на всю стрелку.
//
// Прежде SHOW_TEXT=1 стояло у каждого, и у ветвящейся стрелки подпись
// печаталась столько раз, сколько у неё сегментов: пять копий «Правил
// изготовления» на одной диаграмме, одна поверх другой. Двигаешь стрелку —
// Ramus пересчитывает геометрию, и наверху оказывается то одна копия, то
// другая, каждая со своей серединой отрезка. Со стороны это выглядит так,
// будто подпись отлетает сама по себе, хотя линии стоят на месте.
//
// Правило снято с трёх файлов Ramus: 193 сектора, из них 189 с потоком —
// ноль нарушений. Подпись несёт сектор, который **начинается не в узле** и у
// которого оба конца на месте:
//
//	край → блок, край → узел, блок → блок, блок → край   подпись есть
//	узел → что угодно                                    подписи нет
//	конца нет вовсе (обрубок у края листа)               подписи нет
//
// То есть подписан ровно тот сегмент, с которого стрелка на этой диаграмме
// начинается; ветки, отходящие от узла, молчат. Четыре сектора, выпадающие из
// правила, — это секторы вовсе без потока, каких генератор не делает.
//
// Гасится подпись только в attribute_sector_properties. В attribute_sectors
// SHOW_TEXT=1 стоит у всех 193 секторов без исключения — там это поле про
// другое, и трогать его нельзя.
const (
	sectorHideText = "0"
	// Прозрачность подписи ходит вместе с её показом: у подписанных 1,
	// у молчащих 0. Проверено на всех трёх файлах, исключений нет.
	sectorTranspShown  = "1"
	sectorTranspHidden = "0"
)

// Ход конца: куда линия уходит из точки.
//
// Правило снято с трёх файлов Ramus — 84 конца, ноль нарушений. Заполняется
// только у концов, сидящих на узле; у концов на блоке, на краю листа и у всех
// точек внутри ломаной стоит «не определено».
//
// Прежде здесь стояло «правила для 0 и 1 из данных не видно», и это было верно
// ровно до тех пор, пока смотрели на все точки разом: среди них узловых концов
// меньше трети, и правило тонуло в остальных. Стоило разделить точки по тому,
// на чём сидит их конец, как оно проступило без единого исключения.
//
// Одного этого поля для соединённых линий мало — проверено в Ramus: с ходом,
// записанным у всех узловых концов, стык выглядел прежним. Узлом конец делают
// общие ординаты (см. writeSectors); ход пишется, потому что так пишет Ramus.
const (
	pointTypeNone       = "-1"
	pointTypeHorizontal = "0"
	pointTypeVertical   = "1"
)

// pointTypeOf отдаёт ход конца для точки i ломаной.
//
// Узел отличается от прочих концов **видом**, а не наличием номера кросспоинта:
// номер есть и у концов, которыми сшиты уровни, и таких вдвое больше. Написать
// им ход значило бы разойтись с Ramus, который оставляет их неопределёнными.
func pointTypeOf(seg *ir.Segment, i int) string {
	var end *ir.Endpoint
	var next int
	switch {
	case i == 0:
		end, next = seg.From, 1
	case i == len(seg.Points)-1:
		end, next = seg.To, i-1
	default:
		return pointTypeNone // точка внутри ломаной: у Ramus всегда -1
	}

	if end == nil || end.Kind() != ir.EndpointNode {
		return pointTypeNone
	}
	if next < 0 || next >= len(seg.Points) {
		return pointTypeNone // ломаная из одной точки: хода нет
	}

	at, to := seg.Points[i], seg.Points[next]
	switch {
	case at.X.Val == to.X.Val && at.Y.Val == to.Y.Val:
		// Вырожденный отрезок. Сказать про него «горизонталь» значило бы
		// соврать в поле, по которому Ramus рисует.
		return pointTypeNone
	case at.Y.Val == to.Y.Val:
		return pointTypeHorizontal
	case at.X.Val == to.X.Val:
		return pointTypeVertical
	default:
		return pointTypeNone // наискось маршруты не ходят, но правило полное
	}
}

// writeSectors записывает стрелки: по сектору на каждый нарисованный сегмент.
//
// Ошибка здесь — не ошибка автора: связи проверил валидатор, геометрию раздала
// раскладка. Если чего-то не хватает, это наш дефект, и он обязан кончиться
// отказом с кодом 2, а не файлом без стрелки.
func writeSectors(m *rsf.Model, source *ir.Model, functions, streams map[string]int64, c *counters) error {
	if source.Layout == nil || len(source.Layout.Arrows) == 0 {
		return nil
	}

	qualifier, ok := m.Qualifier("F_SECTORS")
	if !ok {
		return fmt.Errorf("в заготовке нет квалификатора F_SECTORS")
	}
	attributes := make(map[string]int64, 6)
	for _, name := range []string{
		"F_FUNCTION_SECTOR", "F_SECTOR_STREAM",
		"F_SECTOR_BORDER_START", "F_SECTOR_BORDER_END",
		"F_SECTOR_POINTS", "F_SECTOR_PROPERTIES", "F_SECTOR_ATTRIBUTE",
	} {
		id, found := m.Attribute(name)
		if !found {
			return fmt.Errorf("в заготовке нет атрибута %s", name)
		}
		attributes[name] = id
	}

	next, err := m.File.NextID("elements", "ELEMENT_ID")
	if err != nil {
		return err
	}

	// Ординаты делятся на пару «диаграмма + поток», а не на сектор. Узел
	// у Ramus — не координата, а пара ординат: точки он сравнивает по их
	// тождеству, и концы трёх секторов с равными X/Y, но разными номерами,
	// для него — три посторонние точки, лежащие рядом. Ствол с ветками он тогда
	// рисует отдельными линиями без скругления в стыке. Мера — три файла Ramus:
	// 28 узлов, у всех общие ординаты; номер ординаты при этом ни разу не
	// выходит за пределы одного потока на одной диаграмме.
	type owner struct{ diagram, stream int64 }
	ordinates := make(map[owner]*lines)

	// Подписи — до записи: место одной зависит от мест всех остальных на
	// той же диаграмме.
	labels := placeLabels(source)

	for _, arrow := range source.Layout.Arrows {
		stream, ok := streams[arrow.Flow.Name]
		if !ok {
			return fmt.Errorf("поток «%s»: стрелка есть, а самого потока в файле нет", arrow.Flow.Name)
		}
		nodes := newNodeIDs(c)
		// Линии этой стрелки, по диаграммам. Точки одной стрелки, лежащие на
		// общей прямой, обязаны нести один номер линии: на нём держится и
		// выравнивание при перетаскивании блока, и само понимание Ramus, что
		// перед ним одна стрелка, а не стопка совпадающих обрывков.
		//
		// По диаграммам, а не на всю стрелку: в «тесте» вертикаль «контроля»
		// на родительской диаграмме несёт номер 169, а тот же x на диаграмме
		// ребёнка — 166. Разным стрелкам общих линий не заводится и здесь.
		// byDiagram := make(map[int64]*lines)
		for _, seg := range arrow.Segments {
			diagram, err := diagramID(m, seg, functions)
			if err != nil {
				return fmt.Errorf("поток «%s»: %w", arrow.Flow.Name, err)
			}
			shared, ok := ordinates[owner{diagram, stream}]
			if !ok {
				shared = newLines(c)
				ordinates[owner{diagram, stream}] = shared
			}

			id := next
			next++
			if err := writeSector(m, sector{
				id:         id,
				qualifier:  qualifier,
				attributes: attributes,
				diagram:    diagram,
				stream:     stream,
				flow:       arrow.Flow.Name,
				segment:    seg,
				functions:  functions,
				counters:   c,
				nodes:      nodes,
				ordinates:  shared,
				label:      labels[seg],
			}); err != nil {
				return fmt.Errorf("поток «%s»: %w", arrow.Flow.Name, err)
			}
		}
	}
	return nil
}

// sector — всё, что нужно для записи одного сегмента.
type sector struct {
	id         int64
	qualifier  int64
	attributes map[string]int64
	diagram    int64
	stream     int64
	flow       string
	segment    *ir.Segment
	functions  map[string]int64
	counters   *counters
	// nodes — узлы ветвления этой стрелки. Область видимости — одна стрелка:
	// кросспоинт в файле всегда принадлежит ровно одному потоку, и имена
	// узлов разных стрелок пересечься не могут.
	nodes *nodeIDs
	// ordinates — координатные линии этого потока на этой диаграмме: общие у
	// всех его секторов, чтобы концы, сходящиеся в узле, были для Ramus одной
	// точкой.
	ordinates *lines
	// label — рамка подписи, если сегмент её несёт; поставлена заранее,
	// вместе со всеми подписями диаграммы.
	label label
}

// diagramID отвечает, на чьей диаграмме нарисован сегмент.
//
// У контекстной диаграммы владелец — элемент модели, а не корневая работа:
// корневая работа лежит на ней блоком, и перепутать их значило бы нарисовать
// стрелку внутри декомпозиции вместо A-0.
func diagramID(m *rsf.Model, seg *ir.Segment, functions map[string]int64) (int64, error) {
	if seg.Context {
		return m.Element, nil
	}
	id, ok := functions[seg.On.Name]
	if !ok {
		return 0, fmt.Errorf("сегмент нарисован на диаграмме работы «%s», которой нет в файле", seg.On.Name)
	}
	return id, nil
}

// writeSector кладёт один сегмент во все его таблицы.
func writeSector(m *rsf.Model, s sector) error {
	element := fmt.Sprint(s.id)

	rows := []struct {
		table  string
		values map[string]string
	}{
		{"elements", map[string]string{
			"ELEMENT_ID":        element,
			"ELEMENT_NAME":      "",
			"QUALIFIER_ID":      fmt.Sprint(s.qualifier),
			"CREATED_BRANCH_ID": "0",
			"REMOVED_BRANCH_ID": rsf.AliveBranch,
		}},
		{"attribute_other_elements", map[string]string{
			"ATTRIBUTE_ID":    fmt.Sprint(s.attributes["F_FUNCTION_SECTOR"]),
			"ELEMENT_ID":      element,
			"OTHER_ELEMENT":   fmt.Sprint(s.diagram),
			"VALUE_BRANCH_ID": "0",
		}},
		{"attribute_other_elements", map[string]string{
			"ATTRIBUTE_ID":    fmt.Sprint(s.attributes["F_SECTOR_STREAM"]),
			"ELEMENT_ID":      element,
			"OTHER_ELEMENT":   fmt.Sprint(s.stream),
			"VALUE_BRANCH_ID": "0",
		}},
	}

	for _, border := range []struct {
		attribute string
		end       *ir.Endpoint
	}{
		{"F_SECTOR_BORDER_START", s.segment.From},
		{"F_SECTOR_BORDER_END", s.segment.To},
	} {
		if border.end == nil {
			// Висящий конец: строки границы у него в файле нет вовсе.
			// Написать её со значениями «ничего» значило бы сказать не то,
			// что сказано в оригинале.
			continue
		}
		values, err := borderRow(s, border.end)
		if err != nil {
			return err
		}
		values["ATTRIBUTE_ID"] = fmt.Sprint(s.attributes[border.attribute])
		values["ELEMENT_ID"] = element
		rows = append(rows, struct {
			table  string
			values map[string]string
		}{"attribute_sector_borders", values})
	}

	ordinates := s.ordinates
	for i, p := range s.segment.Points {
		rows = append(rows, struct {
			table  string
			values map[string]string
		}{"attribute_sector_points", map[string]string{
			"ATTRIBUTE_ID":    fmt.Sprint(s.attributes["F_SECTOR_POINTS"]),
			"ELEMENT_ID":      element,
			"POSITION":        fmt.Sprint(i),
			"POINT_TYPE":      pointTypeOf(s.segment, i),
			"X_POSITION":      number(p.X.Val),
			"Y_POSITION":      number(p.Y.Val),
			"X_ORDINATE_ID":   fmt.Sprint(ordinates.ordinateX(p.X.Val)),
			"Y_ORDINATE_ID":   fmt.Sprint(ordinates.ordinateY(p.Y.Val)),
			"VALUE_BRANCH_ID": "0",
		}})
	}

	// Строка свойств есть у каждого сектора, но у молчащего она пустая:
	// рамка в нулях, показ и прозрачность сняты. Так и в файлах Ramus —
	// ни одного молчащего сектора с ненулевой рамкой на 135 проверенных.
	show, transparent := sectorHideText, sectorTranspHidden
	var box label
	if labeled(s.segment) {
		show, transparent = sectorShowText, sectorTranspShown
		box = s.label
	}
	rows = append(rows, struct {
		table  string
		values map[string]string
	}{"attribute_sector_properties", map[string]string{
		"ATTRIBUTE_ID":    fmt.Sprint(s.attributes["F_SECTOR_PROPERTIES"]),
		"ELEMENT_ID":      element,
		"SHOW_TEXT":       show,
		"SHOW_TILDA":      sectorTilda,
		"TEXT_HIEGHT":     number(box.height),
		"TEXT_WIDTH":      number(box.width),
		"TEXT_X":          number(box.x),
		"TEXT_Y":          number(box.y),
		"TILDA_POS":       sectorTildaPos,
		"TRANSPARENT":     transparent,
		"VALUE_BRANCH_ID": "0",
	}})

	rows = append(rows, struct {
		table  string
		values map[string]string
	}{"attribute_sectors", map[string]string{
		"ATTRIBUTE_ID":      fmt.Sprint(s.attributes["F_SECTOR_ATTRIBUTE"]),
		"ELEMENT_ID":        element,
		"ALTERNATIVE_TEXT":  "",
		"CREATE_POS":        sectorCreatePos,
		"CREATE_STATE":      sectorCreateSt,
		"SHOW_TEXT":         sectorShowText,
		"TEXT_ALIGMENT":     sectorAlignment,
		"VISUAL_ATTRIBUTES": sectorVisual,
		"VALUE_BRANCH_ID":   "0",
	}})

	for _, r := range rows {
		table, err := m.File.Table(r.table)
		if err != nil {
			return err
		}
		if _, err := table.Add(r.values); err != nil {
			return fmt.Errorf("%s: %w", r.table, err)
		}
	}
	return nil
}

// borderRow собирает строку конца сегмента.
//
// Вид конца называет ровно одно поле: FUNCTION у прицепленного к блоку,
// BORDER_TYPE у лежащего на краю листа. Прочие остаются -1, как в настоящих
// файлах. Номер узла стоит у обоих: им сшиваются уровни.
func borderRow(s sector, e *ir.Endpoint) (map[string]string, error) {
	values := map[string]string{
		"FUNCTION":        "-1",
		"FUNCTION_TYPE":   "-1",
		"BORDER_TYPE":     "-1",
		"TUNNEL_SOFT":     "0",
		"VALUE_BRANCH_ID": "0",
	}

	switch e.Kind() {
	case ir.EndpointFunction:
		id, ok := s.functions[e.Function.Name]
		if !ok {
			return nil, fmt.Errorf("конец прицеплен к работе «%s», которой нет в файле", e.Function.Name)
		}
		values["FUNCTION"] = fmt.Sprint(id)
		values["FUNCTION_TYPE"] = fmt.Sprint(int(sideOf(e.Side)))
	case ir.EndpointBorder:
		values["BORDER_TYPE"] = fmt.Sprint(int(borderCodeOf(e.Border)))
	case ir.EndpointNode:
		// Узел ветвления. Сама раскладка их не заводит, но автор вправе
		// описать геометрию с ветвлением руками, и декомпилятор такие концы
		// читает. Чего читает декомпилятор, то генератор обязан записать.
		values["CROSSPOINT"] = fmt.Sprint(s.nodes.id(e.Node.Name))
		return values, nil
	default:
		return nil, fmt.Errorf("конец сегмента не назван: ни блок, ни край листа, ни узел")
	}

	values["CROSSPOINT"] = fmt.Sprint(s.counters.junctionOf(s.flow, s.segment, e))
	return values, nil
}

// labeled отвечает, несёт ли сегмент подпись стрелки.
//
// Правило и мера — в комментарии к sectorHideText.
func labeled(seg *ir.Segment) bool {
	if seg.From == nil || seg.To == nil {
		return false // обрубок: конца нет, подписывать нечего
	}
	return seg.From.Kind() != ir.EndpointNode
}

// label — рамка подписи стрелки. Место и размер ей даёт placeLabels
// (label.go): подпись не видит соседей, пока её ставят по одной, и потому
// ставится проходом по всем подписям диаграммы разом.
type label struct{ x, y, width, height float64 }

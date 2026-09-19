package generate

import (
	"fmt"
	"math"

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
	sectorTransp    = "1"
	sectorShowText  = "1"

	// Высота строки подписи при шрифте Dialog 10. В файлах она кратна числу
	// строк: 9.8 — одна, 19.6 — две, 39.2 — четыре.
	sectorLineHeight = 9.80078125
	// Ширина знака. Не метрика шрифта, а оценка по настоящим подписям:
	// «вход2» 5 рун → 21, «контроль» 8 → 33, «Детали изделия» 14 → 58,
	// «Швеёно-вышивальгая машинка» 26 → 112. Выходит 4.1…4.6 на руну.
	// Скорее всего Ramus пересчитает её при отрисовке — значение выглядит
	// кэшем, — но оставлять ноль нельзя: у подписанных стрелок рамка непустая.
	sectorRuneWidth = 4.3
)

// Точки ломаной. POINT_TYPE в настоящих файлах чаще всего -1 (76 точек из 101
// в «тесте»), правила для 0 и 1 из данных не видно, и -1 — значение, с которым
// Ramus заведомо работает.
const pointType = "-1"

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
		byDiagram := make(map[int64]*lines)
		for _, seg := range arrow.Segments {
			diagram, err := diagramID(m, seg, functions)
			if err != nil {
				return fmt.Errorf("поток «%s»: %w", arrow.Flow.Name, err)
			}
			ordinates, drawn := byDiagram[diagram]
			if !drawn {
				ordinates = newLines(c)
				byDiagram[diagram] = ordinates
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
				ordinates:  ordinates,
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
	// ordinates — координатные линии этой стрелки на этой диаграмме. Общие для
	// всех её секторов: номер линии и есть то, чем Ramus сшивает сегменты в
	// одну стрелку.
	ordinates *lines
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

	// Ординаты считаются на сектор: точки, лежащие на одной прямой, делят
	// номер линии. Так это устроено в настоящих файлах.
	ordinates := s.ordinates
	for i, p := range s.segment.Points {
		rows = append(rows, struct {
			table  string
			values map[string]string
		}{"attribute_sector_points", map[string]string{
			"ATTRIBUTE_ID":    fmt.Sprint(s.attributes["F_SECTOR_POINTS"]),
			"ELEMENT_ID":      element,
			"POSITION":        fmt.Sprint(i),
			"POINT_TYPE":      pointType,
			"X_POSITION":      number(p.X.Val),
			"Y_POSITION":      number(p.Y.Val),
			"X_ORDINATE_ID":   fmt.Sprint(ordinates.ordinateX(p.X.Val)),
			"Y_ORDINATE_ID":   fmt.Sprint(ordinates.ordinateY(p.Y.Val)),
			"VALUE_BRANCH_ID": "0",
		}})
	}

	label := labelBox(s.flow, s.segment)
	rows = append(rows, struct {
		table  string
		values map[string]string
	}{"attribute_sector_properties", map[string]string{
		"ATTRIBUTE_ID":    fmt.Sprint(s.attributes["F_SECTOR_PROPERTIES"]),
		"ELEMENT_ID":      element,
		"SHOW_TEXT":       sectorShowText,
		"SHOW_TILDA":      sectorTilda,
		"TEXT_HIEGHT":     number(label.height),
		"TEXT_WIDTH":      number(label.width),
		"TEXT_X":          number(label.x),
		"TEXT_Y":          number(label.y),
		"TILDA_POS":       sectorTildaPos,
		"TRANSPARENT":     sectorTransp,
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

// label — рамка подписи стрелки.
type label struct{ x, y, width, height float64 }

// labelBox считает, где и какой величины стоять подписи.
//
// Ставится она на середине самого длинного отрезка: там для неё больше всего
// места, и она реже налезает на соседнюю стрелку. Размер — оценка: настоящей
// метрики шрифта у компилятора нет, а нулевая рамка в файлах означает «подпись
// не показывать».
func labelBox(flow string, seg *ir.Segment) label {
	width := float64(len([]rune(flow))) * sectorRuneWidth
	at := middlePoint(seg)
	return label{
		x:      at.x,
		y:      at.y,
		width:  width,
		height: sectorLineHeight,
	}
}

type xy struct{ x, y float64 }

// middlePoint — середина самого длинного отрезка ломаной.
func middlePoint(seg *ir.Segment) xy {
	if len(seg.Points) == 0 {
		return xy{}
	}
	if len(seg.Points) == 1 {
		return xy{x: seg.Points[0].X.Val, y: seg.Points[0].Y.Val}
	}

	best, bestLength := 1, -1.0
	for i := 1; i < len(seg.Points); i++ {
		a, b := seg.Points[i-1], seg.Points[i]
		length := math.Abs(b.X.Val-a.X.Val) + math.Abs(b.Y.Val-a.Y.Val)
		if length > bestLength {
			best, bestLength = i, length
		}
	}
	a, b := seg.Points[best-1], seg.Points[best]
	return xy{x: (a.X.Val + b.X.Val) / 2, y: (a.Y.Val + b.Y.Val) / 2}
}

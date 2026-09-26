package generate_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
	"github.com/Zdertis420/autoramus/ramusc/internal/syntax"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// Стрелки в файле. Проверяется не картинка, а то, что в таблицах лежит ровно
// то, что лежит в настоящих моделях Ramus: состав строк, коды сторон и общий
// номер узла между уровнями.

// documentModel проводит документ через тот же конвейер, что и компиляция:
// разбор, проверку и раскладку. Генератор получает модель именно такой.
func documentModel(t *testing.T, path string) *ir.Model {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	model, diags, internal := validate.Build(src)
	if internal != nil {
		t.Fatal(internal)
	}
	if diags.HasErrors() {
		t.Fatalf("документ %s невалиден", path)
	}
	layout.Apply(model)
	return model
}

func documentPath(name string) string {
	return filepath.Join("..", "..", "testdata", "documents", name)
}

func examplePath(name string) string {
	return filepath.Join("..", "..", "..", "examples", name)
}

// buildFrom собирает файл из документа и открывает его обратно.
func buildFrom(t *testing.T, path string) (*rsf.File, *rsf.Model) {
	t.Helper()

	file, err := generate.File(documentModel(t, path))
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(file)
	if err != nil {
		t.Fatalf("собранный файл не читается: %v", err)
	}
	return file, m
}

// TestSectorRows — состав строк сектора.
//
// Восемь строк в шести таблицах: элемент, диаграмма, поток, два конца, точки,
// подпись и оформление. Иерархической строки у сектора нет — проверено на всех
// трёх настоящих моделях, и лишняя строка означала бы, что мы пишем не то,
// что пишет Ramus.
func TestSectorRows(t *testing.T) {
	file, m := buildFrom(t, examplePath("skirt.yaml"))

	sectors := m.Sectors()
	if len(sectors) == 0 {
		t.Fatal("в собранном файле нет ни одного сектора")
	}

	for _, table := range []string{"attribute_hierarchicals"} {
		rows, err := file.Table(table)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range sectors {
			if _, ok := rows.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(s.ID))); ok {
				t.Errorf("у сектора %d есть строка в %s, а у настоящих её нет", s.ID, table)
			}
		}
	}

	for _, s := range sectors {
		if s.Diagram < 0 {
			t.Errorf("сектор %d не назван диаграммой", s.ID)
		}
		if s.Stream < 0 {
			t.Errorf("сектор %d не назван потоком", s.ID)
		}
		if len(s.Points) < 2 {
			t.Errorf("сектор %d: точек %d", s.ID, len(s.Points))
		}
		for _, end := range []*rsf.Border{s.Start, s.End} {
			if end == nil {
				t.Errorf("сектор %d: конец не записан", s.ID)
				continue
			}
			if end.TunnelSoft {
				t.Errorf("сектор %d: туннель, которого мы не ставим", s.ID)
			}
			if end.Crosspoint < 0 {
				t.Errorf("сектор %d: конец без номера узла", s.ID)
			}
		}
	}

	// Оформление и подпись — по строке на сектор.
	for _, table := range []string{"attribute_sector_properties", "attribute_sectors"} {
		rows, err := file.Table(table)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows.Rows) != len(sectors) {
			t.Errorf("%s: строк %d, секторов %d", table, len(rows.Rows), len(sectors))
		}
	}

	// Ветка версионирования у всех значений одна: строка, попавшая в другую,
	// для Ramus не существует.
	for _, table := range []string{
		"attribute_sector_borders", "attribute_sector_points",
		"attribute_sector_properties", "attribute_sectors",
	} {
		rows, err := file.Table(table)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows.Rows {
			if got := rows.Str(row, "VALUE_BRANCH_ID"); got != "0" {
				t.Errorf("%s: VALUE_BRANCH_ID = %q", table, got)
			}
		}
	}
}

// TestSectorSidesMatchDocument — стрелка приходит на ту сторону, которая
// названа в документе (FR-003, SC-003).
//
// Сверяется ICOM собранного файла с ICOM документа: у каждой работы те же
// потоки на тех же сторонах.
func TestSectorSidesMatchDocument(t *testing.T) {
	for _, name := range []string{"skirt.yaml", "skirt.json"} {
		t.Run(name, func(t *testing.T) {
			source := documentModel(t, examplePath(name))
			file, err := generate.File(source)
			if err != nil {
				t.Fatal(err)
			}
			m, err := rsf.NewModel(file)
			if err != nil {
				t.Fatal(err)
			}

			names := make(map[int64]string)
			for _, f := range m.Functions() {
				names[f.ID] = f.Name
			}

			got := make(map[string]bool)
			for id, icom := range m.ICOM() {
				for side, flows := range map[string][]string{
					ir.SideIn:        icom.In,
					ir.SideControl:   icom.Control,
					ir.SideMechanism: icom.Mechanism,
					ir.SideOut:       icom.Out,
				} {
					for _, flow := range flows {
						got[fmt.Sprintf("%s|%s|%s", names[id], side, flow)] = true
					}
				}
			}

			for _, l := range source.Links {
				if l.To.Name != "" {
					key := fmt.Sprintf("%s|%s|%s", l.To.Name, l.SideName(), l.Flow.Name)
					if !got[key] {
						t.Errorf("работа «%s» не принимает «%s» стороной %s",
							l.To.Name, l.Flow.Name, l.SideName())
					}
				}
				if l.From.Name != "" {
					key := fmt.Sprintf("%s|%s|%s", l.From.Name, ir.SideOut, l.Flow.Name)
					if !got[key] {
						t.Errorf("работа «%s» не отдаёт «%s» выходом", l.From.Name, l.Flow.Name)
					}
				}
			}
		})
	}
}

// TestJunctionJoinsLevels — конец на блоке родителя и конец на краю листа
// дочерней диаграммы несут один номер узла.
//
// Так согласование уровней выражено в настоящих файлах: в `тест.rsf` у
// «контроля» это cp=2, в «Изготовлении юбки» — cp=2, 12 и 15. Во входном языке
// такой сшивки нет, вычисляет её генератор, и проверить её больше негде.
func TestJunctionJoinsLevels(t *testing.T) {
	_, m := buildFrom(t, examplePath("skirt.yaml"))

	names := make(map[int64]string)
	for _, f := range m.Functions() {
		names[f.ID] = f.Name
	}
	streams := make(map[int64]string)
	for _, s := range m.Streams() {
		streams[s.ID] = s.Name
	}

	// Ключ — работа, сторона и поток; собираем номера узлов с обеих сторон
	// перехода: с блока на диаграмме родителя и с края на диаграмме самой
	// работы.
	onBlock := make(map[string]int64)
	onBorder := make(map[string]int64)

	for _, s := range m.Sectors() {
		flow := streams[s.Stream]
		for _, end := range []*rsf.Border{s.Start, s.End} {
			switch {
			case end.OnFunction():
				onBlock[fmt.Sprintf("%s|%s|%s", names[end.Function], end.FunctionType.ICOM(), flow)] = end.Crosspoint
			case end.OnBorder():
				// Владелец диаграммы — работа, внутри которой нарисован
				// сегмент. У контекстной диаграммы владелец не работа, и
				// такие концы в сшивке не участвуют.
				if owner, ok := names[s.Diagram]; ok {
					onBorder[fmt.Sprintf("%s|%s|%s", owner, rsf.Side(borderCode(end)).ICOM(), flow)] = end.Crosspoint
				}
			}
		}
	}

	checked := 0
	for key, border := range onBorder {
		block, ok := onBlock[key]
		if !ok {
			t.Errorf("граничная стрелка %s не нашла своей половины на диаграмме родителя", key)
			continue
		}
		if block != border {
			t.Errorf("%s: узел на блоке %d, на краю %d — стрелка распадётся надвое",
				key, block, border)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("ни одного перехода между уровнями не проверено")
	}
}

// borderCode отдаёт код края листа: в rsf.Border он лежит отдельным полем.
func borderCode(b *rsf.Border) int { return b.BorderType }

// TestContextDiagramSectors — ICOM корневой работы нарисован на A-0.
//
// Владелец сегментов там — элемент модели, а не корневая работа: перепутать их
// значило бы нарисовать контекстные стрелки внутри декомпозиции.
func TestContextDiagramSectors(t *testing.T) {
	_, m := buildFrom(t, examplePath("skirt.yaml"))

	streams := make(map[int64]string)
	for _, s := range m.Streams() {
		streams[s.ID] = s.Name
	}

	sides := make(map[string]rsf.Side)
	for _, s := range m.Sectors() {
		if s.Diagram != m.Element {
			continue
		}
		for _, end := range []*rsf.Border{s.Start, s.End} {
			if end.OnBorder() {
				sides[streams[s.Stream]] = rsf.Side(end.BorderType)
			}
		}
	}

	want := map[string]rsf.Side{
		"Ткань":     rsf.SideLeft,
		"Фурнитура": rsf.SideLeft,
		"Правила изготовления": rsf.SideTop,
		"Швея-закройщица":      rsf.SideBottom,
		"Юбка":                 rsf.SideRight,
	}
	for flow, side := range want {
		got, ok := sides[flow]
		if !ok {
			t.Errorf("на A-0 нет стрелки «%s»", flow)
			continue
		}
		if got != side {
			t.Errorf("«%s» упирается в край %s, ожидался %s", flow, got, side)
		}
	}
}

// TestNoLinksWritesNothing — модель без связей не добавляет ни строки
// (FR-014).
//
// Через документ такая модель не проходит: валидатор требует у работы все
// четыре стороны ICOM. Поэтому она собирается руками — требование адресовано
// генератору, а не автору.
func TestNoLinksWritesNothing(t *testing.T) {
	empty, err := generate.Template()
	if err != nil {
		t.Fatal(err)
	}

	model := &ir.Model{
		Name: ir.Ref{Name: "Пустая", Path: "/model"},
		Functions: []*ir.Function{
			{Name: ir.Ref{Name: "Пустая", Path: "/functions/0/name"}},
		},
	}
	model.Index()
	layout.Apply(model)

	file, err := generate.File(model)
	if err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{
		"attribute_sector_borders", "attribute_sector_points",
		"attribute_sector_properties", "attribute_sectors",
	} {
		rows, err := file.Table(table)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows.Rows) != 0 {
			t.Errorf("%s: без связей записано %d строк", table, len(rows.Rows))
		}
	}

	// Потоков тоже нет: элементов в файле должно стать ровно на одну работу
	// больше, чем в заготовке.
	before, err := empty.Table("elements")
	if err != nil {
		t.Fatal(err)
	}
	after, err := file.Table("elements")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(after.Rows)-len(before.Rows), 1; got != want {
		t.Errorf("элементов добавилось %d, ожидалась одна работа", got)
	}

	// И последовательности остались нетронутыми: номеров никто не выдавал.
	seq, err := file.Sequences()
	if err != nil {
		t.Fatal(err)
	}
	base, err := empty.Sequences()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{rsf.SequenceOrdinates, rsf.SequenceCrosspoints} {
		if seq[name] != base[name] {
			t.Errorf("%s = %d, а без стрелок должна остаться %d", name, seq[name], base[name])
		}
	}
}

// TestSequencesGrow — выданные номера возвращаются в файл.
//
// Ramus эти две последовательности сам не перематывает: оставить их прежними
// значило бы, что следующая правка в Ramus выдаст уже занятые номера.
func TestSequencesGrow(t *testing.T) {
	file, m := buildFrom(t, examplePath("skirt.yaml"))

	seq, err := file.Sequences()
	if err != nil {
		t.Fatal(err)
	}

	points := 0
	for _, s := range m.Sectors() {
		points += len(s.Points)
	}
	if seq[rsf.SequenceOrdinates] <= 1 {
		t.Errorf("%s = %d, а точек записано %d", rsf.SequenceOrdinates, seq[rsf.SequenceOrdinates], points)
	}
	if seq[rsf.SequenceCrosspoints] <= 1 {
		t.Errorf("%s = %d, а концов записано %d", rsf.SequenceCrosspoints,
			seq[rsf.SequenceCrosspoints], 2*len(m.Sectors()))
	}
}

// TestOrdinatesSharedWithinSector — точки на одной прямой делят номер линии.
//
// Так это устроено в настоящих файлах: у сектора столько номеров, сколько у
// него различных координат, а не сколько точек.
func TestOrdinatesSharedWithinSector(t *testing.T) {
	_, m := buildFrom(t, examplePath("skirt.yaml"))

	for _, s := range m.Sectors() {
		xs := make(map[float64]int64)
		ys := make(map[float64]int64)
		for _, p := range s.Points {
			if known, ok := xs[p.X]; ok && known != p.XOrdinate {
				t.Errorf("сектор %d: x = %g названо линиями %d и %d", s.ID, p.X, known, p.XOrdinate)
			}
			if known, ok := ys[p.Y]; ok && known != p.YOrdinate {
				t.Errorf("сектор %d: y = %g названо линиями %d и %d", s.ID, p.Y, known, p.YOrdinate)
			}
			xs[p.X] = p.XOrdinate
			ys[p.Y] = p.YOrdinate
		}
	}
}

// Узел в Ramus — не координата, а пара ординат. Точки сравниваются по
// тождеству ординат (Point.equals), направление отрезка — по тому, общая ли
// ордината у его концов (Pin.getType), а соседи в узле ищутся перебором точек
// той же ординаты (Point.getPins). Три конца с одинаковыми X/Y, но разными
// номерами ординат для Ramus — три посторонние точки, лежащие рядом: он не
// видит ни ствола, ни ответвления и не скругляет стык (ArrowPainter.paintPin).

// ordinateViolation — нарушение правила общих ординат.
type ordinateViolation struct {
	what string
}

// ordinateViolations проверяет два правила, снятых с файлов Ramus:
//
//  1. концы, сидящие на одном узле одной диаграммы, делят обе ординаты;
//  2. номер ординаты не выходит за пределы одной стрелки: у всех его точек
//     одна диаграмма и один поток.
//
// Второе нужно не для отрисовки, а для правки в Ramus: ордината — общая
// направляющая, и точки чужой стрелки на ней поехали бы вслед за перетаскиванием.
func ordinateViolations(m *rsf.Model) []ordinateViolation {
	var out []ordinateViolation

	type node struct{ diagram, crosspoint int64 }
	type pair struct{ x, y int64 }
	nodes := make(map[node]pair)
	var nodeOrder []node

	type owner struct{ diagram, stream int64 }
	type line struct {
		axis byte
		id   int64
	}
	owners := make(map[line]owner)
	var lineOrder []line
	reported := make(map[line]bool)

	for _, s := range m.Sectors() {
		if len(s.Points) < 2 {
			continue
		}
		last := len(s.Points) - 1
		for _, e := range []struct {
			border *rsf.Border
			point  rsf.Point
		}{{s.Start, s.Points[0]}, {s.End, s.Points[last]}} {
			if !junctionEnd(e.border) {
				continue
			}
			k := node{s.Diagram, e.border.Crosspoint}
			got := pair{e.point.XOrdinate, e.point.YOrdinate}
			known, ok := nodes[k]
			if !ok {
				nodes[k] = got
				nodeOrder = append(nodeOrder, k)
				continue
			}
			if known != got {
				out = append(out, ordinateViolation{fmt.Sprintf(
					"диаграмма %d, узел %d: сектор %d сидит на ординатах (%d, %d), а соседний — на (%d, %d)",
					k.diagram, k.crosspoint, s.ID, got.x, got.y, known.x, known.y)})
			}
		}

		if s.Stream < 0 {
			// Стрелка без потока: у Ramus две такие в «тесте» делят ординату,
			// и различать их нечем — потока «никакой» у обеих.
			continue
		}
		for _, p := range s.Points {
			for _, l := range []line{{'x', p.XOrdinate}, {'y', p.YOrdinate}} {
				o := owner{s.Diagram, s.Stream}
				known, ok := owners[l]
				if !ok {
					owners[l] = o
					lineOrder = append(lineOrder, l)
					continue
				}
				if known != o && !reported[l] {
					reported[l] = true
					out = append(out, ordinateViolation{fmt.Sprintf(
						"ордината %c%d общая у двух стрелок: диаграмма %d поток %d и диаграмма %d поток %d",
						l.axis, l.id, known.diagram, known.stream, o.diagram, o.stream)})
				}
			}
		}
	}
	return out
}

// TestJunctionOrdinatesInRamusFiles — мера правила на файлах самого Ramus.
//
// 28 узлов на трёх файлах, у всех концы делят обе ординаты; номер ординаты ни
// разу не выходит за пределы одного потока на одной диаграмме.
func TestJunctionOrdinatesInRamusFiles(t *testing.T) {
	for _, name := range []string{"тест.rsf", "ИзготовлениеЮбки.rsf", "ФормированиеТП.rsf"} {
		t.Run(name, func(t *testing.T) {
			file, err := rsf.Open(examplePath(name))
			if err != nil {
				t.Fatal(err)
			}
			m, err := rsf.NewModel(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range ordinateViolations(m) {
				t.Errorf("%s — значит правило не правило", v.what)
			}
		})
	}
}

// TestJunctionEndsShareOrdinates — то же правило на наших файлах.
//
// Прежде ординаты раздавались на сектор, и ни один из 22 узлов «чахохбили» не
// делил их: Ramus рисовал ствол и ветки отдельными линиями, лежащими рядом.
func TestJunctionEndsShareOrdinates(t *testing.T) {
	for _, path := range []string{
		examplePath("chakhokhbili.yaml"),
		examplePath("skirt.yaml"),
		examplePath("skirt-full.yaml"),
		documentPath("staircase.yaml"),
		documentPath("nested.yaml"),
		documentPath("feedback.yaml"),
		documentPath("channels-trunk.yaml"),
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, m := buildFrom(t, path)
			for _, v := range ordinateViolations(m) {
				t.Error(v.what)
			}
		})
	}
}

// TestNestedLevels — модель с тремя уровнями: сегменты чужих диаграмм не
// смешиваются.
func TestNestedLevels(t *testing.T) {
	_, m := buildFrom(t, documentPath("nested.yaml"))

	diagrams := make(map[string]int)
	names := make(map[int64]string)
	for _, f := range m.Functions() {
		names[f.ID] = f.Name
	}
	for _, s := range m.Sectors() {
		name := names[s.Diagram]
		if s.Diagram == m.Element {
			name = "A-0"
		}
		diagrams[name]++
	}

	for _, want := range []string{"A-0", "Производство", "Обработка"} {
		if diagrams[want] == 0 {
			t.Errorf("на диаграмме «%s» не нарисовано ни одной стрелки", want)
		}
	}
	if n := diagrams["Контроль"]; n != 0 {
		t.Errorf("у работы без декомпозиции «Контроль» нарисовано %d стрелок", n)
	}
}

// TestFlowNamesSurvive — имя потока доезжает посимвольно (FR-015).
//
// Запятая, кавычка и возврат каретки уже ломали вывод раньше: из-за них
// заводили правила кавычек в писателе и сохранение «\r» при разборе.
func TestFlowNamesSurvive(t *testing.T) {
	source := documentModel(t, examplePath("skirt.yaml"))

	tricky := []string{
		`Раздел "основные решения"`,
		"Ткань, фурнитура и нитки",
		"Строка\r\nс возвратом",
	}
	for i, name := range tricky {
		if i >= len(source.Flows) {
			t.Fatal("в документе меньше потоков, чем нужно проверке")
		}
		// Имя потока правится и в словаре, и в связях: идентификатор потока —
		// это его имя (Р4), и разъехаться они не должны.
		old := source.Flows[i].Name
		source.Flows[i].Name = name
		for _, l := range source.Links {
			if l.Flow.Name == old {
				l.Flow.Name = name
			}
		}
		for _, a := range source.Layout.Arrows {
			if a.Flow.Name == old {
				a.Flow.Name = name
			}
		}
	}

	file, err := generate.File(source)
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(file)
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]bool)
	for _, s := range m.Streams() {
		got[s.Name] = true
	}
	for _, name := range tricky {
		if !got[name] {
			t.Errorf("имя потока %q не доехало до файла; есть: %s",
				name, strings.Join(keys(got), ", "))
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestDiagramOwnersHaveVisualData — у владельца каждой диаграммы, на которой
// есть сегменты, стоит строка F_VISUAL_DATA.
//
// Это условие отрисовки, а не оформление: SectorRefactor.loadFromFunction
// выходит по `return`, не дойдя до секторов, если строки нет, — и диаграмма
// остаётся пустой при полностью верных таблицах секторов. Так и пропадала A-0:
// работам строку писали, элементу модели (владельцу сегментов контекстной
// диаграммы) — нет.
//
// Проверяются все проверочные документы: владельцем бывает и работа, и элемент
// модели, и упустить можно любого.
func TestDiagramOwnersHaveVisualData(t *testing.T) {
	for _, name := range []string{"staircase.yaml", "feedback.yaml", "nested.yaml"} {
		t.Run(name, func(t *testing.T) {
			file, m := buildFrom(t, documentPath(name))

			visual, err := file.Table("attribute_visual_datas")
			if err != nil {
				t.Fatal(err)
			}

			owners := make(map[int64]bool)
			for _, s := range m.Sectors() {
				owners[s.Diagram] = true
			}
			if len(owners) == 0 {
				t.Fatal("в собранном файле нет ни одной диаграммы с сегментами")
			}

			for owner := range owners {
				if _, ok := visual.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(owner))); !ok {
					t.Errorf("владелец диаграммы %d рисует сегменты, но строки F_VISUAL_DATA у него нет: Ramus не покажет ни одного", owner)
				}
			}
		})
	}
}

// crosspointViolation — номер узла, доставшийся геометрически разным точкам
// одной диаграммы.
type crosspointViolation struct {
	diagram, crosspoint int64
	points              [][2]float64
}

// crosspointViolations ищет нарушения инварианта «узел — одна точка».
//
// Узел у Ramus — именно точка, а не метка: два сегмента, делящие номер, обязаны
// сходиться в одной координате. Номер, стоящий у двух разных мест диаграммы,
// противоречив, и рисунок по нему не строится.
//
// Номер, общий у точек **разных** диаграмм, нарушением не является: так сшиты
// уровни (фича 006), и в «Юбке» таких узлов большинство. Поэтому ключ —
// диаграмма плюс номер.
func crosspointViolations(m *rsf.Model) []crosspointViolation {
	type key struct{ diagram, crosspoint int64 }

	// Конец сегмента лежит в крайней точке ломаной: начало — в первой,
	// конец — в последней.
	seen := make(map[key][][2]float64)
	var order []key
	for _, s := range m.Sectors() {
		if len(s.Points) < 2 {
			continue
		}
		ends := []struct {
			border *rsf.Border
			point  rsf.Point
		}{
			{s.Start, s.Points[0]},
			{s.End, s.Points[len(s.Points)-1]},
		}
		for _, e := range ends {
			if e.border == nil || e.border.Crosspoint < 0 {
				continue
			}
			k := key{s.Diagram, e.border.Crosspoint}
			if _, known := seen[k]; !known {
				order = append(order, k)
			}
			seen[k] = append(seen[k], [2]float64{e.point.X, e.point.Y})
		}
	}

	var out []crosspointViolation
	for _, k := range order {
		points := seen[k]
		for _, p := range points[1:] {
			if p != points[0] {
				out = append(out, crosspointViolation{k.diagram, k.crosspoint, points})
				break
			}
		}
	}
	return out
}

// TestCrosspointIsOnePointInRamusFiles — мера инварианта.
//
// Проверяется не наш код, а утверждение о формате: если бы Ramus допускал
// номер узла у двух разных точек диаграммы, требовать этого от генератора было
// бы нечем. Три модели, 255 узлов, ноль нарушений — утверждение измерено.
func TestCrosspointIsOnePointInRamusFiles(t *testing.T) {
	for _, name := range []string{"тест.rsf", "ИзготовлениеЮбки.rsf", "ФормированиеТП.rsf"} {
		t.Run(name, func(t *testing.T) {
			file, err := rsf.Open(examplePath(name))
			if err != nil {
				t.Fatal(err)
			}
			m, err := rsf.NewModel(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range crosspointViolations(m) {
				t.Errorf("диаграмма %d, узел %d стоит у разных точек: %v — значит инвариант не инвариант",
					v.diagram, v.crosspoint, v.points)
			}
		})
	}
}

// TestCrosspointIsOnePoint — тот же инвариант на наших файлах.
//
// Нарушают его ветвящиеся потоки: поток, приходящий с края листа к трём
// работам, мы рисуем тремя отдельными линиями, а номер узла даём один — он
// вычисляется по потоку, работе и стороне и потому у всех трёх совпадает.
// Три точки под одним номером противоречивы (FR-004, FR-005).
func TestCrosspointIsOnePoint(t *testing.T) {
	documents := []string{
		examplePath("skirt.yaml"),
		documentPath("staircase.yaml"),
		documentPath("feedback.yaml"),
		documentPath("nested.yaml"),
	}
	for _, path := range documents {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, m := buildFrom(t, path)
			for _, v := range crosspointViolations(m) {
				t.Errorf("диаграмма %d, узел %d стоит у %d разных точек: %v",
					v.diagram, v.crosspoint, len(v.points), v.points)
			}
		})
	}
}

// Ход конца в узле. У конца сегмента, сидящего на узле одной диаграммы, Ramus
// записывает, куда линия уходит из этой точки: 0 — горизонталью, 1 —
// вертикалью. Мера снята с трёх его файлов, 84 конца, ноль нарушений
// (specs/011-junction-orientation/research.md).
//
// Причиной «стрелки не выглядят соединёнными» поле не было: его заполнение
// картинку в Ramus не изменило. Причина — ординаты, см. ordinateViolations.

// junctionEnd сообщает, что конец сидит на узле.
//
// Узел — конец, не прицепленный ни к работе, ни к краю листа. Различать по
// одному лишь номеру кросспоинта нельзя: номер есть и у концов, которыми сшиты
// уровни, и таких вдвое больше.
func junctionEnd(b *rsf.Border) bool {
	return b != nil && b.Crosspoint >= 0 && !b.OnFunction() && b.BorderType < 0
}

// wayOut отдаёт ход конца по соседней точке: 0 — горизонталь, 1 — вертикаль,
// -1 — хода нет (отрезок вырожден или идёт наискось).
func wayOut(at, next rsf.Point) int64 {
	switch {
	case at.X == next.X && at.Y == next.Y:
		return -1
	case at.Y == next.Y:
		return 0
	case at.X == next.X:
		return 1
	default:
		return -1
	}
}

// TestJunctionEndsCarryOrientation — у каждого конца-узла записан ход.
//
// Проверяется по собранному файлу, а не по раскладке: поле пишет генератор, и
// спрос с того, что доехало до .rsf.
func TestJunctionEndsCarryOrientation(t *testing.T) {
	for _, path := range []string{
		examplePath("chakhokhbili.yaml"),
		examplePath("skirt.yaml"),
		examplePath("skirt-full.yaml"),
		documentPath("staircase.yaml"),
		documentPath("nested.yaml"),
		documentPath("channels-trunk.yaml"),
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, m := buildFrom(t, path)

			junctions := 0
			for _, s := range m.Sectors() {
				if len(s.Points) < 2 {
					continue
				}
				last := len(s.Points) - 1
				for _, end := range []struct {
					border *rsf.Border
					at     rsf.Point
					next   rsf.Point
					where  string
				}{
					{s.Start, s.Points[0], s.Points[1], "начало"},
					{s.End, s.Points[last], s.Points[last-1], "конец"},
				} {
					want := wayOut(end.at, end.next)
					if !junctionEnd(end.border) {
						// Не узел: блок, край листа или сшивка уровней. Ramus
						// оставляет ход неопределённым.
						if end.at.Type != -1 {
							t.Errorf("сектор %d, %s: ход %d у конца, который не сидит на узле",
								s.ID, end.where, end.at.Type)
						}
						continue
					}
					junctions++
					if want == -1 {
						continue // вырожденный отрезок: хода нет
					}
					if end.at.Type != want {
						t.Errorf("сектор %d, %s (%g, %g): ход %d, а отрезок уходит %s",
							s.ID, end.where, end.at.X, end.at.Y, end.at.Type,
							map[int64]string{0: "горизонталью (0)", 1: "вертикалью (1)"}[want])
					}
				}
			}
			if junctions == 0 {
				t.Logf("узлов нет — проверять нечего")
			} else {
				t.Logf("узловых концов проверено: %d", junctions)
			}
		})
	}
}

// TestSectorLabelOnce — подпись у стрелки одна на диаграмму.
//
// Прежде SHOW_TEXT=1 стояло у каждого сектора, и ветвящаяся стрелка несла
// столько подписей, сколько у неё сегментов: пять «Правил изготовления» на
// одной диаграмме, одна поверх другой. Стоило подвинуть стрелку, как Ramus
// пересчитывал геометрию и наверху оказывалась другая копия — со стороны это
// выглядело так, будто подпись отлетает сама по себе.
//
// Мера — три настоящие модели: 189 секторов с потоком, у каждой пары
// «диаграмма + поток» ровно одна подпись, ноль исключений.
func TestSectorLabelOnce(t *testing.T) {
	file, m := buildFrom(t, examplePath("skirt.yaml"))

	props, err := file.Table("attribute_sector_properties")
	if err != nil {
		t.Fatal(err)
	}

	type owner struct{ diagram, stream int64 }
	labels := make(map[owner]int)
	seen := make(map[owner]bool)

	for _, s := range m.Sectors() {
		key := owner{s.Diagram, s.Stream}
		seen[key] = true

		row, ok := props.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(s.ID)))
		if !ok {
			t.Errorf("сектор %d: нет строки подписи", s.ID)
			continue
		}
		shown := props.Value(row, "SHOW_TEXT").Text == "1"
		if shown {
			labels[key]++
		}

		// У молчащего сектора рамка пустая: в настоящих файлах нет ни одного
		// с SHOW_TEXT=0 и ненулевой рамкой.
		for _, field := range []string{"TEXT_X", "TEXT_Y", "TEXT_WIDTH", "TEXT_HIEGHT"} {
			v, err := strconv.ParseFloat(props.Value(row, field).Text, 64)
			if err != nil {
				t.Errorf("сектор %d: %s не число: %q", s.ID, field, props.Value(row, field).Text)
				continue
			}
			zero := v == 0
			if !shown && !zero {
				t.Errorf("сектор %d: подписи нет, а %s не ноль", s.ID, field)
			}
			if shown && field == "TEXT_WIDTH" && zero {
				t.Errorf("сектор %d: подпись показана, а рамка пустая", s.ID)
			}
		}

		// Прозрачность ходит вместе с показом.
		want := "0"
		if shown {
			want = "1"
		}
		if got := props.Value(row, "TRANSPARENT").Text; got != want {
			t.Errorf("сектор %d: SHOW_TEXT=%v, а TRANSPARENT=%s", s.ID, shown, got)
		}
	}

	for key := range seen {
		if labels[key] != 1 {
			t.Errorf("диаграмма %d, поток %d: подписей %d, а должна быть одна",
				key.diagram, key.stream, labels[key])
		}
	}
}

// TestSectorAttributeShowText — в attribute_sectors подпись не гасится.
//
// Поле SHOW_TEXT есть в двух таблицах, и это разные поля. В
// attribute_sector_properties Ramus его гасит у веток, а в attribute_sectors
// оно равно 1 у всех 193 секторов всех трёх моделей без единого исключения.
// Погасить его заодно значило бы починить одно и сломать другое.
func TestSectorAttributeShowText(t *testing.T) {
	file, m := buildFrom(t, examplePath("skirt.yaml"))

	rows, err := file.Table("attribute_sectors")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range m.Sectors() {
		row, ok := rows.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(s.ID)))
		if !ok {
			t.Errorf("сектор %d: нет строки оформления", s.ID)
			continue
		}
		if got := rows.Value(row, "SHOW_TEXT").Text; got != "1" {
			t.Errorf("сектор %d: в attribute_sectors SHOW_TEXT=%s, у Ramus всегда 1", s.ID, got)
		}
	}
}

// Промежутки у блока. Стрелки одной стороны делят её на n+1 частей, и при
// постоянном размере блока они сходились до неразличимого: двенадцать входов
// на высоте 50.4 — по 3.9 единицы. Раскладка растит блок под его стрелки;
// здесь проверяется, что до файла доехало именно это
// (specs/012-arrow-port-spacing).

// minPortGap — порог из раскладки (layout.portGap). Повторён, а не
// экспортирован: тест меряет файл снаружи и не должен верить числу, которое
// проверяет.
const minPortGap = 15.0

// portGap — промежуток между соседними стрелками одной стороны блока.
type portGap struct {
	function int64
	side     rsf.Side
	arrows   int
	gap      float64
}

// portGaps собирает промежутки по файлу.
//
// Точка крепления — крайняя точка сектора, чей конец прицеплен к блоку. Одна
// точка может принадлежать нескольким секторам (поток, уходящий из порта к
// нескольким получателям), поэтому точки собираются множеством.
func portGaps(m *rsf.Model) []portGap {
	type side struct {
		function int64
		side     rsf.Side
	}
	along := make(map[side]map[float64]bool)
	var order []side
	for _, s := range m.Sectors() {
		if len(s.Points) == 0 {
			continue
		}
		for _, e := range []struct {
			border *rsf.Border
			point  rsf.Point
		}{{s.Start, s.Points[0]}, {s.End, s.Points[len(s.Points)-1]}} {
			if !e.border.OnFunction() {
				continue
			}
			k := side{e.border.Function, e.border.FunctionType}
			v := e.point.X // верх и низ блока: стрелки идут вдоль x
			if k.side == rsf.SideLeft || k.side == rsf.SideRight {
				v = e.point.Y
			}
			if along[k] == nil {
				along[k] = make(map[float64]bool)
				order = append(order, k)
			}
			along[k][v] = true
		}
	}

	var out []portGap
	for _, k := range order {
		values := make([]float64, 0, len(along[k]))
		for v := range along[k] {
			values = append(values, v)
		}
		sort.Float64s(values)
		for i := 1; i < len(values); i++ {
			out = append(out, portGap{k.function, k.side, len(values), values[i] - values[i-1]})
		}
	}
	return out
}

// TestPortSpacingInRamusFiles — мера порога.
//
// Авто-раскладка Ramus (`тест.rsf`, модель, которую никто не двигал) под порог
// не попадает: два входа на высоте 50.4 стоят в 16.8. Только этот файл, а не
// все три: в `ФормированиеТП.rsf` блок 72 × 50.4 с тремя входами даёт 12.6 —
// Ramus блоков не растит, и при трёх стрелках на стороне порог нарушил бы и он.
// Утверждение, стало быть, узкое: при двух стрелках Ramus не теснее 15.
func TestPortSpacingInRamusFiles(t *testing.T) {
	file, err := rsf.Open(examplePath("тест.rsf"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := rsf.NewModel(file)
	if err != nil {
		t.Fatal(err)
	}
	gaps := portGaps(m)
	if len(gaps) == 0 {
		t.Fatal("в «тесте» не нашлось ни одной стороны с двумя стрелками — мерить нечего")
	}
	for _, g := range gaps {
		if g.gap < minPortGap-1e-9 {
			t.Errorf("работа %d, сторона %s: промежуток %.1f — порог выше того, что делает сам Ramus",
				g.function, g.side.ICOM(), g.gap)
		}
	}
}

// pinnedSides отмечает стороны блоков, геометрию которых задал автор: размер
// блока по этой оси или хоть одну стрелку, прицепленную к этой стороне. Там
// точки крепления ставил не раскладчик, и требовать от них порога нельзя —
// авторское главнее (Р2).
func pinnedSides(t *testing.T, path string) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	model, _, internal := validate.Build(src)
	if internal != nil {
		t.Fatal(internal)
	}
	out := make(map[string]bool)
	if model.Layout == nil {
		return out
	}
	for _, f := range model.Layout.Functions {
		if f.Height.Set {
			out[f.Function.Name+"|"+ir.SideIn] = true
			out[f.Function.Name+"|"+ir.SideOut] = true
		}
		if f.Width.Set {
			out[f.Function.Name+"|"+ir.SideControl] = true
			out[f.Function.Name+"|"+ir.SideMechanism] = true
		}
	}
	for _, a := range model.Layout.Arrows {
		for _, s := range a.Segments {
			for _, e := range []*ir.Endpoint{s.From, s.To} {
				if e != nil && e.Function.Name != "" {
					out[e.Function.Name+"|"+e.Side] = true
				}
			}
		}
	}
	return out
}

// TestPortSpacing — ни на одной стороне ни одного блока набора стрелки не
// стоят теснее порога (FR-001, SC-001).
//
// Набор берётся перечнем каталога: новый документ проверяется без правки теста.
func TestPortSpacing(t *testing.T) {
	documents, err := filepath.Glob(documentPath("*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	documents = append(documents,
		examplePath("chakhokhbili.yaml"),
		examplePath("skirt.yaml"),
		examplePath("skirt-full.yaml"),
	)
	for _, path := range documents {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, m := buildFrom(t, path)
			pinned := pinnedSides(t, path)
			names := make(map[int64]string)
			for _, f := range m.Functions() {
				names[f.ID] = f.Name
			}
			for _, g := range portGaps(m) {
				if pinned[names[g.function]+"|"+g.side.ICOM()] {
					continue
				}
				if g.gap < minPortGap-1e-9 {
					t.Errorf("«%s», сторона %s: %d стрелок, промежуток %.1f < %.0f",
						names[g.function], g.side.ICOM(), g.arrows, g.gap, minPortGap)
				}
			}
		})
	}
}

// Подписи (specs/013-arrow-label-overlap).

// labelRow — показанная подпись одного сектора, как она лежит в файле.
type labelRow struct {
	sector     int64
	diagram    int64
	stream     int64
	flow       string
	x, y, w, h float64
}

// shownLabels собирает показанные подписи файла.
func shownLabels(t *testing.T, file *rsf.File, m *rsf.Model) []labelRow {
	t.Helper()
	props, err := file.Table("attribute_sector_properties")
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[int64]string)
	for _, s := range m.Streams() {
		names[s.ID] = s.Name
	}
	num := func(row rsf.Row, field string) float64 {
		v, err := strconv.ParseFloat(props.Value(row, field).Text, 64)
		if err != nil {
			t.Fatalf("%s не число: %q", field, props.Value(row, field).Text)
		}
		return v
	}
	var out []labelRow
	for _, s := range m.Sectors() {
		row, ok := props.First(rsf.Eq("ELEMENT_ID", fmt.Sprint(s.ID)))
		if !ok || props.Value(row, "SHOW_TEXT").Text != "1" {
			continue
		}
		out = append(out, labelRow{
			sector: s.ID, diagram: s.Diagram, stream: s.Stream, flow: names[s.Stream],
			x: num(row, "TEXT_X"), y: num(row, "TEXT_Y"),
			w: num(row, "TEXT_WIDTH"), h: num(row, "TEXT_HIEGHT"),
		})
	}
	return out
}

// lineHeight — высота строки подписи в файлах Ramus (Dialog 8).
const lineHeight = 9.80078125

// TestGlyphWidthsMatchRamus — мера таблицы ширин.
//
// Проверяется не наш код, а утверждение о шрифте: однострочная подпись в файле
// Ramus шириной ровно в своё имя, посчитанное по таблице. Если таблица с
// файлами Ramus не сходится, всё, что меряется по ней дальше, меряет не то.
func TestGlyphWidthsMatchRamus(t *testing.T) {
	checked := 0
	for _, name := range []string{"ИзготовлениеЮбки.rsf", "ФормированиеТП.rsf"} {
		file, err := rsf.Open(examplePath(name))
		if err != nil {
			t.Fatal(err)
		}
		m, err := rsf.NewModel(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range shownLabels(t, file, m) {
			if l.flow == "" || math.Abs(l.h-lineHeight) > 1e-6 {
				continue // многострочные: их ширину автор мог выбрать сам
			}
			checked++
			if got := generate.TextWidth(l.flow); math.Abs(got-l.w) > 0.5 {
				t.Errorf("%s: «%s» по таблице %v, в файле %v", name, l.flow, got, l.w)
			}
		}
	}
	if checked == 0 {
		t.Fatal("однострочных подписей не нашлось — мерить было нечего")
	}
	t.Logf("однострочных подписей сверено: %d", checked)
}

// labelDocuments — набор, на котором проверяются подписи: все документы
// каталога и примеры поставки. Перечень каталога — чтобы новый документ
// проверялся без правки теста.
func labelDocuments(t *testing.T) []string {
	t.Helper()
	documents, err := filepath.Glob(documentPath("*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return append(documents,
		examplePath("chakhokhbili.yaml"),
		examplePath("skirt.yaml"),
		examplePath("skirt-full.yaml"),
		examplePath("diamond-production.yaml"),
	)
}

// wordsOf режет имя на слова так же, как Ramus переносит строки
// (PStringBounder, BreakIterator): по пробелам и после дефиса, дефис остаётся
// со словом. Своя реализация, а не генератора: тест не должен верить тому,
// что проверяет.
func wordsOf(name string) []string {
	var out []string
	for _, field := range strings.Fields(name) {
		for {
			i := strings.Index(field, "-")
			if i < 0 || i == len(field)-1 {
				break
			}
			out = append(out, field[:i+1])
			field = field[i+1:]
		}
		out = append(out, field)
	}
	return out
}

// TestLabelFitsWords — рамка подписи вмещает каждое слово имени (FR-005).
//
// Ramus переносит текст по ширине рамки, и слово, которое в неё не влезло,
// режет по буквам: «Помидор|ы». Рамка не уже самого длинного слова — и
// резать ему нечего.
func TestLabelFitsWords(t *testing.T) {
	for _, path := range labelDocuments(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, m := buildFrom(t, path)
			for _, l := range shownLabels(t, file, m) {
				for _, word := range wordsOf(l.flow) {
					if need := generate.TextWidth(word); l.w < need-1e-9 {
						t.Errorf("«%s»: слово «%s» шириной %v не влезает в рамку %v",
							l.flow, word, need, l.w)
					}
				}
			}
		})
	}
}

// labelBoundaryDocuments — документы, где подписи законно налезают: у стрелки
// в пределах 34 единиц нет места ни без подписей, ни без блоков (FR-012,
// названная граница). Отдельным перечнем, чтобы на остальных проверка
// оставалась безусловной.
var labelBoundaryDocuments = map[string]bool{
	"labels-crowded.yaml": true,
}

// overlaps — площадь пересечения двух прямоугольников больше нуля.
func overlaps(ax, ay, aw, ah, bx, by, bw, bh float64) bool {
	const eps = 1e-6
	return math.Min(ax+aw, bx+bw)-math.Max(ax, bx) > eps &&
		math.Min(ay+ah, by+bh)-math.Max(ay, by) > eps
}

// TestLabelsDoNotCollide — подписи не лежат друг на друге, на блоках и за
// листом (FR-001–FR-003, SC-001, SC-002).
//
// Проверяется рамка, записанная в файл. Ramus её пересчитывает, но если она
// не меньше текста — а ширина посчитана метрикой, — его рамка ложится внутрь
// нашей (specs/013-arrow-label-overlap, research И3), и проверка по нашей
// честная.
func TestLabelsDoNotCollide(t *testing.T) {
	for _, path := range labelDocuments(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, m := buildFrom(t, path)
			names := make(map[int64]string)
			blocks := make(map[int64][]rsf.Function)
			for _, f := range m.Functions() {
				names[f.ID] = f.Name
				if f.Bounds != nil {
					blocks[f.Parent] = append(blocks[f.Parent], f)
				}
			}

			var problems []string
			labels := shownLabels(t, file, m)
			for i, a := range labels {
				if a.x < 7-1e-9 || a.y < 7-1e-9 || a.x+a.w > 793+1e-9 || a.y+a.h > 437+1e-9 {
					problems = append(problems, fmt.Sprintf("«%s» выходит за лист: (%v, %v) %v × %v",
						a.flow, a.x, a.y, a.w, a.h))
				}
				for _, b := range labels[i+1:] {
					if a.diagram == b.diagram && overlaps(a.x, a.y, a.w, a.h, b.x, b.y, b.w, b.h) {
						problems = append(problems, fmt.Sprintf("диаграмма «%s»: «%s» налезает на «%s»",
							names[a.diagram], a.flow, b.flow))
					}
				}
				for _, f := range blocks[a.diagram] {
					r := f.Bounds
					if overlaps(a.x, a.y, a.w, a.h, r.X, r.Y, r.Width, r.Height) {
						problems = append(problems, fmt.Sprintf("диаграмма «%s»: «%s» налезает на работу «%s»",
							names[a.diagram], a.flow, f.Name))
					}
				}
			}

			if labelBoundaryDocuments[filepath.Base(path)] {
				// Названная граница: наложения ожидаемы, но видны поимённо.
				for _, p := range problems {
					t.Log(p)
				}
				return
			}
			for _, p := range problems {
				t.Error(p)
			}
		})
	}
}

// labelReach — дальше этого подпись от своей линии не уходит: самая далёкая
// подпись в моделях Ramus набора стоит в 34 единицах.
const labelReach = 34.0

// distanceToOwnLine — от рамки подписи до ближайшего отрезка секторов её
// потока на её диаграмме.
func distanceToOwnLine(m *rsf.Model, l labelRow) float64 {
	best := math.Inf(1)
	for _, s := range m.Sectors() {
		if s.Diagram != l.diagram || s.Stream != l.stream {
			continue
		}
		for i := 1; i < len(s.Points); i++ {
			a, b := s.Points[i-1], s.Points[i]
			x1, x2 := math.Min(a.X, b.X), math.Max(a.X, b.X)
			y1, y2 := math.Min(a.Y, b.Y), math.Max(a.Y, b.Y)
			dx := math.Max(0, math.Max(l.x-x2, x1-(l.x+l.w)))
			dy := math.Max(0, math.Max(l.y-y2, y1-(l.y+l.h)))
			best = math.Min(best, math.Hypot(dx, dy))
		}
	}
	return best
}

// TestLabelNearOwnLine — подпись понятно чья: не дальше 34 единиц от своей
// линии (FR-004, SC-003), в том числе там, где места нет.
func TestLabelNearOwnLine(t *testing.T) {
	for _, path := range labelDocuments(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, m := buildFrom(t, path)
			for _, l := range shownLabels(t, file, m) {
				if d := distanceToOwnLine(m, l); d > labelReach+1e-9 {
					t.Errorf("«%s» в %.1f от своей линии — дальше %v", l.flow, d, labelReach)
				}
			}
		})
	}
}

// authoredArrows отмечает пары «поток + диаграмма», которые автор нарисовал
// сам: их точки не прокладывала раскладка, и спрашивать с них «не сквозь
// блок» — значит спрашивать с автора; для этого есть предупреждение
// arrow_through_block. Диаграмма зовётся работой-владельцем, контекстная —
// пустой строкой.
//
// IR строится прямо из разбора, без validate.Build: тот раскладывает модель, и
// после него все стрелки выглядят авторскими.
func authoredArrows(t *testing.T, path string) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := syntax.Load(src)
	model := ir.Build(root)
	out := make(map[string]bool)
	if model == nil || model.Layout == nil {
		return out
	}
	for _, a := range model.Layout.Arrows {
		for _, s := range a.Segments {
			diagram := s.On.Name
			if s.Context {
				diagram = ""
			}
			out[a.Flow.Name+"|"+diagram] = true
		}
	}
	return out
}

// TestNoArrowThroughBlock — ни один отрезок стрелки, проложенной раскладкой,
// не проходит внутри блока своей диаграммы (FR-001, FR-002, SC-002).
//
// Мерится собранный файл, после разведения по каналам и подписей: спрос с
// того, что увидит автор. Касание стороны блока — не пересечение.
func TestNoArrowThroughBlock(t *testing.T) {
	for _, path := range labelDocuments(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, m := buildFrom(t, path)
			authored := authoredArrows(t, path)

			names := make(map[int64]string)
			blocks := make(map[int64][]rsf.Function)
			for _, f := range m.Functions() {
				names[f.ID] = f.Name
				if f.Bounds != nil {
					blocks[f.Parent] = append(blocks[f.Parent], f)
				}
			}
			streams := make(map[int64]string)
			for _, s := range m.Streams() {
				streams[s.ID] = s.Name
			}

			const eps = 1e-6
			for _, s := range m.Sectors() {
				flow := streams[s.Stream]
				diagram := names[s.Diagram] // у контекстной владельца-работы нет — пусто
				if authored[flow+"|"+diagram] {
					continue
				}
				for i := 1; i < len(s.Points); i++ {
					a, b := s.Points[i-1], s.Points[i]
					x1, x2 := math.Min(a.X, b.X), math.Max(a.X, b.X)
					y1, y2 := math.Min(a.Y, b.Y), math.Max(a.Y, b.Y)
					for _, f := range blocks[s.Diagram] {
						r := f.Bounds
						if x2 > r.X+eps && x1 < r.X+r.Width-eps && y2 > r.Y+eps && y1 < r.Y+r.Height-eps {
							t.Errorf("«%s» проходит сквозь работу «%s»: (%.1f, %.1f) → (%.1f, %.1f)",
								flow, strings.TrimSpace(f.Name), a.X, a.Y, b.X, b.Y)
						}
					}
				}
			}
		})
	}
}

// Пересечения стрелок (specs/015-remove-double-crossings). Считаются ровно так,
// как evidence/double.py, по которому сняты потолки: иначе тест сторожил бы не
// те числа.

// arrowLines — ломаные секторов по стрелкам: «диаграмма + поток». Все сегменты
// одного потока на диаграмме, включая ветки дерева, — одна стрелка: отвод,
// пересекающий собственную магистраль, пересечением не считается. Сектор без
// потока — стрелка сам по себе.
func arrowLines(m *rsf.Model) (map[[2]int64][][]rsf.Point, [][2]int64) {
	out := make(map[[2]int64][][]rsf.Point)
	var order [][2]int64
	for _, s := range m.Sectors() {
		k := [2]int64{s.Diagram, s.Stream}
		if s.Stream < 0 {
			k[1] = -s.ID - 1
		}
		if _, seen := out[k]; !seen {
			order = append(order, k)
		}
		out[k] = append(out[k], s.Points)
	}
	return out, order
}

// crossAt — точка, где горизонталь одного отрезка проходит строго внутри
// вертикали другого. Касание концами — не пересечение.
func crossAt(a, b, c, d rsf.Point) (rsf.Point, bool) {
	const eps = 1e-6
	ah, ch := math.Abs(a.Y-b.Y) < eps, math.Abs(c.Y-d.Y) < eps
	if ah == ch {
		return rsf.Point{}, false
	}
	if !ah {
		a, b, c, d = c, d, a, b
	}
	x1, x2 := math.Min(a.X, b.X), math.Max(a.X, b.X)
	y1, y2 := math.Min(c.Y, d.Y), math.Max(c.Y, d.Y)
	if x1+eps < c.X && c.X < x2-eps && y1+eps < a.Y && a.Y < y2-eps {
		return rsf.Point{X: math.Round(c.X*10) / 10, Y: math.Round(a.Y*10) / 10}, true
	}
	return rsf.Point{}, false
}

// crossings — сколько различных точек пересечения у двух стрелок.
func crossings(p, q [][]rsf.Point) int {
	seen := make(map[[2]float64]bool)
	for _, pl := range p {
		for _, ql := range q {
			for i := 1; i < len(pl); i++ {
				for j := 1; j < len(ql); j++ {
					if at, ok := crossAt(pl[i-1], pl[i], ql[j-1], ql[j]); ok {
						seen[[2]float64{at.X, at.Y}] = true
					}
				}
			}
		}
	}
	return len(seen)
}

// totalCrossings — пересечения всех пар разных стрелок одной диаграммы.
func totalCrossings(m *rsf.Model) int {
	lines, order := arrowLines(m)
	n := 0
	for i, a := range order {
		for _, b := range order[i+1:] {
			if a[0] == b[0] {
				n += crossings(lines[a], lines[b])
			}
		}
	}
	return n
}

// crossingCeilings — потолки пересечений по документам: замер
// evidence/double.py. Храповик: снижается, когда раскладка становится лучше, и
// не поднимается молча. Новый документ обязан получить свой.
//
// Сняты до фичи 015 и опущены после неё там, где она убрала лишние
// пересечения: «чахохбили» 158 → 156, channels-pair 2 → 0, channels-trunk 5 → 4.
var crossingCeilings = map[string]int{
	"branch-border.yaml":      2,
	"channels-feedback.yaml":  16,
	"channels-pair.yaml":      0,
	"channels-trunk.yaml":     4,
	"dfd-tunnel.yaml":         0,
	"feedback.yaml":           1,
	"labels-crowded.yaml":     0,
	"nested.yaml":             2,
	"staircase.yaml":          0,
	"two-sides.yaml":          1,
	"chakhokhbili.yaml":       156,
	"diamond-production.yaml": 10,
	"skirt-full.yaml":         16,
	"skirt.yaml":              8,
}

// TestCrossingsDoNotGrow — число пересечений ни на одном документе не растёт
// (FR-003): убрав лишнее пересечение в одном месте, раскладка не вправе
// добавить новое в другом.
func TestCrossingsDoNotGrow(t *testing.T) {
	for _, path := range labelDocuments(t) {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			ceiling, ok := crossingCeilings[name]
			if !ok {
				t.Fatalf("у документа нет потолка пересечений: снимите его evidence/double.py и впишите в crossingCeilings")
			}
			_, m := buildFrom(t, path)
			got := totalCrossings(m)
			if got > ceiling {
				t.Errorf("пересечений %d, потолок %d", got, ceiling)
			}
			t.Logf("пересечений %d, потолок %d", got, ceiling)
		})
	}
}

// withLine отдаёт копию стрелки, где отрезок i..i+1 ломаной lines[li] сдвинут
// поперёк себя на координату v: у вертикали меняется x, у горизонтали — y.
func withLine(lines [][]rsf.Point, li, i int, vertical bool, v float64) [][]rsf.Point {
	out := make([][]rsf.Point, len(lines))
	copy(out, lines)
	moved := append([]rsf.Point(nil), lines[li]...)
	for _, k := range []int{i, i + 1} {
		if vertical {
			moved[k].X = v
		} else {
			moved[k].Y = v
		}
	}
	out[li] = moved
	return out
}

// TestNoAvoidableDoubleCrossing — ни одна пара стрелок не пересекает друг друга
// лишний раз (FR-002, SC-001).
//
// Лишнее — то, что исчезает, если поменять местами соседние параллельные
// отрезки двух стрелок: одну и ту же линию разделили, но разошлись не в ту
// сторону. У Ramus таких ноль на всех трёх файлах; пересечения, которые обменом
// не снимаются, — входы поперёк отводов дерева механизмов — есть и у него, и
// сюда не относятся. Порт evidence/swap.py.
//
// Крайние отрезки ломаной не трогаются: они прицеплены к блоку, краю листа или
// узлу, и двигать их значит отцепить стрелку.
func TestNoAvoidableDoubleCrossing(t *testing.T) {
	const near = 2 * 6.0 // два шага канала
	const eps = 1e-6
	for _, path := range labelDocuments(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, m := buildFrom(t, path)
			streams := make(map[int64]string)
			for _, s := range m.Streams() {
				streams[s.ID] = s.Name
			}
			lines, order := arrowLines(m)
			for x, pk := range order {
				for _, qk := range order[x+1:] {
					if pk[0] != qk[0] {
						continue
					}
					P, Q := lines[pk], lines[qk]
					was := crossings(P, Q)
					if was < 2 {
						continue
					}
					best := was
					for pi, pl := range P {
						for a := 1; a+2 < len(pl); a++ {
							for qi, ql := range Q {
								for b := 1; b+2 < len(ql); b++ {
									p1, p2, q1, q2 := pl[a], pl[a+1], ql[b], ql[b+1]
									for _, vertical := range []bool{true, false} {
										at := func(p rsf.Point) (float64, float64) {
											if vertical {
												return p.X, p.Y
											}
											return p.Y, p.X
										}
										pc, plo := at(p1)
										pc2, phi := at(p2)
										qc, qlo := at(q1)
										qc2, qhi := at(q2)
										if math.Abs(pc-pc2) > eps || math.Abs(qc-qc2) > eps || math.Abs(pc-qc) > near {
											continue
										}
										plo, phi = math.Min(plo, phi), math.Max(plo, phi)
										qlo, qhi = math.Min(qlo, qhi), math.Max(qlo, qhi)
										if math.Min(phi, qhi) <= math.Max(plo, qlo) {
											continue // вдоль линии не соседствуют
										}
										now := crossings(withLine(P, pi, a, vertical, qc), withLine(Q, qi, b, vertical, pc))
										best = min(best, now)
									}
								}
							}
						}
					}
					if best < was {
						t.Errorf("«%s» и «%s»: пересечений %d, а после обмена соседних полос — %d",
							streams[pk[1]], streams[qk[1]], was, best)
					}
				}
			}
		})
	}
}

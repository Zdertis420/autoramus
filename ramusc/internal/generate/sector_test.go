package generate_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
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

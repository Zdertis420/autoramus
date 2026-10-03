package layout_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
)

// Маршруты сверяются с `examples/тест.rsf` — моделью, которую Ramus разложил
// сам. Документ `testdata/documents/staircase.yaml` повторяет её структуру:
// четыре подработы цепочкой.

// Границы листа берутся из place_test.go: они там уже объявлены, и второй
// набор тех же чисел однажды разошёлся бы с первым.

// TestForwardRouteMatchesRamus — вертикаль прямой связи стоит ровно посередине
// промежутка между блоками.
//
// Числа взяты из `тест.rsf`: 212⅔, 406 и 599⅓ на трёх связях подряд. Совпадение
// там точное, и это единственное место маршрута, которое Ramus считает сам:
// точки крепления внутренних стрелок в той модели расставлены мышью.
func TestForwardRouteMatchesRamus(t *testing.T) {
	m := modelOf(t, document("staircase.yaml"))
	layout.Apply(m)

	tests := []struct {
		flow string
		want float64
	}{
		{"между12", 212 + 2.0/3.0},
		{"между23", 406},
		{"между34", 599 + 1.0/3.0},
	}

	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			seg := segmentOn(t, arrowOf(t, m, tt.flow), "Работа")
			if len(seg.Points) != 4 {
				t.Fatalf("точек %d, у связи «вправо, вниз, вправо» их четыре: %s",
					len(seg.Points), format(seg))
			}
			// Вертикаль — вторая и третья точки, у них общий x.
			got := seg.Points[1].X.Val
			if !near(got, seg.Points[2].X.Val) {
				t.Errorf("вертикаль разъехалась: %g и %g", got, seg.Points[2].X.Val)
			}
			if !near(got, tt.want) {
				t.Errorf("вертикаль на x = %g, Ramus ставит её на %g", got, tt.want)
			}
		})
	}
}

// TestStaircaseMatchesRamus — блоки встали на диагональ `тест.rsf`.
//
// Проверка не про стрелки, но без неё сверка вертикалей ничего не значит:
// середина промежутка считается по краям блоков.
func TestStaircaseMatchesRamus(t *testing.T) {
	m := modelOf(t, document("staircase.yaml"))
	layout.Apply(m)

	want := []struct {
		name string
		x, y float64
	}{
		{"Первая", 80, 80},
		{"Вторая", 273 + 1.0/3.0, 160 + 2.0/3.0},
		{"Третья", 466 + 2.0/3.0, 241 + 1.0/3.0},
		{"Четвёртая", 660, 322},
	}
	placed := boxes(m)
	for _, w := range want {
		b := placed[w.name]
		if b == nil {
			t.Fatalf("работа «%s» не размещена", w.name)
		}
		if !near(b.X.Val, w.x) || !near(b.Y.Val, w.y) {
			t.Errorf("«%s» в (%g, %g), Ramus кладёт в (%g, %g)", w.name, b.X.Val, b.Y.Val, w.x, w.y)
		}
	}
}

// TestGeometryIsLawful — ни одна стрелка не выходит за лист, не пересекает
// блок и не идёт наискось (FR-005, FR-008).
//
// Проверка «не пересекает блок» спрашивается не со всякой диаграммы. Автор
// вправе расставить работы руками, и `skirt-full.yaml` показывает, чем это
// кончается: заданное вручную «Сшивание деталей» перекрывается с «Добавлением
// фурнитуры», которое досталось раскладке. Когда блоки налезают друг на друга
// сами, обойти их нечем, и требовать от стрелки чистого пути значило бы
// требовать невозможного — ровно это и оговорено в FR-005.
func TestGeometryIsLawful(t *testing.T) {
	for _, path := range append(documents(), document("staircase.yaml")) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m := modelOf(t, path)
			written := authored(m)
			layout.Apply(m)

			blocks := blocksByDiagram(m)
			for _, a := range m.Layout.Arrows {
				for _, s := range a.Segments {
					if written[authoredKey(a.Flow.Name, s)] {
						continue // геометрия автора: она его и ответственность
					}
					own := blocks[diagramOf(s)]
					checkSegment(t, a.Flow.Name, s, own, !blocksOverlap(own))
				}
			}
		})
	}
}

func checkSegment(t *testing.T, flow string, s *ir.Segment, blocks []*ir.FunctionLayout, clean bool) {
	t.Helper()

	for i, p := range s.Points {
		if p.X.Val < sheetLeft || p.X.Val > sheetRight || p.Y.Val < sheetTop || p.Y.Val > sheetBottom {
			t.Errorf("«%s»: точка %d (%g, %g) вне листа", flow, i, p.X.Val, p.Y.Val)
		}
	}

	for i := 1; i < len(s.Points); i++ {
		a, b := s.Points[i-1], s.Points[i]
		if !near(a.X.Val, b.X.Val) && !near(a.Y.Val, b.Y.Val) {
			t.Errorf("«%s»: отрезок %d идёт наискось: (%g, %g) → (%g, %g)",
				flow, i, a.X.Val, a.Y.Val, b.X.Val, b.Y.Val)
		}
		if !clean {
			continue
		}
		for _, block := range blocks {
			if intersects(a, b, block) {
				t.Errorf("«%s»: отрезок %d (%g, %g) → (%g, %g) идёт сквозь работу «%s» %s",
					flow, i, a.X.Val, a.Y.Val, b.X.Val, b.Y.Val, block.Function.Name, format(s))
			}
		}
	}
}

// blocksOverlap сообщает, что блоки диаграммы налезают друг на друга. Такое
// бывает только от ручной геометрии: своя раскладка разносит их по диагонали и
// обходит авторские блоки (clear). Остаётся один случай — автор сам положил
// свои блоки внахлёст; тогда обойти их нечем, и проверка пересечений на такой
// диаграмме отключается — названная граница FR-005 фичи 014. На наборе такой
// диаграммы нет.
func blocksOverlap(blocks []*ir.FunctionLayout) bool {
	for i, a := range blocks {
		for _, b := range blocks[i+1:] {
			if a.X.Val < b.X.Val+b.Width.Val && b.X.Val < a.X.Val+a.Width.Val &&
				a.Y.Val < b.Y.Val+b.Height.Val && b.Y.Val < a.Y.Val+a.Height.Val {
				return true
			}
		}
	}
	return false
}

// intersects — отрезок проходит внутри блока. Касание не считается: точка
// крепления лежит ровно на стороне.
func intersects(a, b ir.Point, block *ir.FunctionLayout) bool {
	left, right := minMax(a.X.Val, b.X.Val)
	top, bottom := minMax(a.Y.Val, b.Y.Val)
	const eps = 1e-9
	return right > block.X.Val+eps && left < block.X.Val+block.Width.Val-eps &&
		bottom > block.Y.Val+eps && top < block.Y.Val+block.Height.Val-eps
}

func minMax(a, b float64) (float64, float64) {
	if a > b {
		return b, a
	}
	return a, b
}

// blocksByDiagram раскладывает блоки по диаграммам: сегмент вправе пересекать
// только то, чего на его диаграмме нет.
func blocksByDiagram(m *ir.Model) map[string][]*ir.FunctionLayout {
	placed := boxes(m)
	out := make(map[string][]*ir.FunctionLayout)
	for _, f := range m.Functions {
		if b := placed[f.Name.Name]; b != nil {
			out[f.Of.Name] = append(out[f.Of.Name], b)
		}
	}
	return out
}

// diagramOf отдаёт владельца диаграммы сегмента; у контекстной он пуст.
func diagramOf(s *ir.Segment) string {
	if s.Context {
		return ""
	}
	return s.On.Name
}

// TestFeedbackGoesAround — обратная связь обходит блоки снаружи.
//
// «Замечания» идут от «Контроля» назад в управление «Обработки». Прямого пути
// нет: приёмник стоит левее источника, и стрелка обязана выйти за лестницу.
func TestFeedbackGoesAround(t *testing.T) {
	m := modelOf(t, document("feedback.yaml"))
	layout.Apply(m)

	seg := segmentOn(t, arrowOf(t, m, "Замечания"), "Производство")
	if seg.From.Function.Name != "Контроль" || seg.To.Function.Name != "Обработка" {
		t.Fatalf("стрелка идёт из «%s» в «%s», ожидалось из «Контроль» в «Обработка»",
			seg.From.Function.Name, seg.To.Function.Name)
	}
	if seg.To.Side != ir.SideControl {
		t.Errorf("приходит стороной %q, ожидалось %q", seg.To.Side, ir.SideControl)
	}

	// Коридор: стрелка обязана подняться выше самого верхнего блока, иначе
	// обхода не вышло.
	placed := boxes(m)
	top := placed["Обработка"].Y.Val
	if placed["Контроль"].Y.Val < top {
		top = placed["Контроль"].Y.Val
	}
	highest := seg.Points[0].Y.Val
	for _, p := range seg.Points {
		if p.Y.Val < highest {
			highest = p.Y.Val
		}
	}
	if highest >= top {
		t.Errorf("стрелка не поднялась выше блоков: верх коридора %g, верх блоков %g\n%s",
			highest, top, format(seg))
	}
}

// TestRoutesAreStable — два прогона дают одни и те же числа.
//
// Раскладка обходит связи документа, а не отображения; проверка ловит возврат
// обхода map, от которого файл поехал бы от запуска к запуску (принцип III).
func TestRoutesAreStable(t *testing.T) {
	first := modelOf(t, example("skirt.yaml"))
	layout.Apply(first)
	second := modelOf(t, example("skirt.yaml"))
	layout.Apply(second)

	if len(first.Layout.Arrows) != len(second.Layout.Arrows) {
		t.Fatalf("стрелок %d и %d", len(first.Layout.Arrows), len(second.Layout.Arrows))
	}
	for i := range first.Layout.Arrows {
		a, b := first.Layout.Arrows[i], second.Layout.Arrows[i]
		if a.Flow.Name != b.Flow.Name {
			t.Fatalf("стрелка %d: «%s» и «%s»", i, a.Flow.Name, b.Flow.Name)
		}
		if format(a.Segments[0]) != format(b.Segments[0]) {
			t.Errorf("«%s»: маршруты разошлись:\n%s\n%s", a.Flow.Name,
				format(a.Segments[0]), format(b.Segments[0]))
		}
	}
}

// format печатает ломаную: без неё сообщение об ошибке нечитаемо.
func format(s *ir.Segment) string {
	out := ""
	for _, p := range s.Points {
		if out != "" {
			out += " → "
		}
		out += fmt.Sprintf("(%g, %g)", p.X.Val, p.Y.Val)
	}
	return out
}

// TestControlRouteClimbsOver — связь «выход → управление» не идёт сквозь
// приёмник, когда тот поднялся выше точки выхода (research Р-6).
//
// Прежний шаблон — вправо до вертикали порта и вниз — считал, что лестница
// ставит приёмник ниже источника. Выросший блок может встать выше: сумма
// высот съедает вертикальный размах, и соседи перекрываются по высоте. Тогда
// горизонталь шла бы сквозь приёмник, и стрелка обходит его сверху.
func TestControlRouteClimbsOver(t *testing.T) {
	control := func(m *ir.Model) {
		m.Flows = append(m.Flows, ir.Ref{Name: "указание"})
		m.Links = append(m.Links,
			produces("источник", "указание"),
			&ir.Link{Flow: ir.Ref{Name: "указание"}, To: ir.Ref{Name: "приёмник"},
				Side: ir.SideControl, Sugar: true},
		)
	}
	segment := func(t *testing.T, m *ir.Model) *ir.Segment {
		t.Helper()
		for _, a := range m.Layout.Arrows {
			if a.Flow.Name == "указание" && len(a.Segments) == 1 {
				return a.Segments[0]
			}
		}
		t.Fatal("стрелка «указание» не нарисована одним сегментом")
		return nil
	}

	t.Run("приёмник выше выхода", func(t *testing.T) {
		m := model("корень", "источник", "приёмник")
		control(m)
		crowd(m, "приёмник", ir.SideIn, 19)
		layout.Apply(m)

		s := segment(t, m)
		source, target := boxes(m)["источник"], boxes(m)["приёмник"]
		if exit := s.Points[0].Y.Val; exit <= target.Y.Val {
			t.Fatalf("выход на y=%v уже выше приёмника (верх %v) — случай не тот, что проверяется",
				exit, target.Y.Val)
		}
		checkSegment(t, "указание", s, []*ir.FunctionLayout{source, target}, true)

		n := len(s.Points)
		last, before := s.Points[n-1], s.Points[n-2]
		if !near(last.Y.Val, target.Y.Val) || before.Y.Val >= last.Y.Val {
			t.Errorf("стрелка приходит в управление не сверху: %s", format(s))
		}
	})

	t.Run("приёмник ниже выхода", func(t *testing.T) {
		m := model("корень", "источник", "приёмник")
		control(m)
		layout.Apply(m)
		if s := segment(t, m); len(s.Points) != 3 {
			t.Errorf("без перекрытия маршрут прежний, из трёх точек, а вышло %s", format(s))
		}
	})
}

// TestDetourIsNearest — обход берёт ближайшую чистую полосу, а не коридор
// (specs/014-arrows-avoid-blocks, US2).
//
// «Курица с луком» идёт от «Обжарки лука» к «Тушению» через одну ступень
// лестницы. Середина промежутка — центр пропущенной «Подготовки томатной
// массы». Чистая полоса есть в соседнем промежутке, между «Подготовкой» и
// «Тушением»: горизонталь выхода проходит над «Подготовкой». Маршрут тот же по
// форме — четыре точки, изломов не прибавилось, — сдвинута только вертикаль.
func TestDetourIsNearest(t *testing.T) {
	m := modelOf(t, example("chakhokhbili.yaml"))
	layout.Apply(m)

	const diagram = "Приготовление чахохбили"
	var seg *ir.Segment
	for _, a := range m.Layout.Arrows {
		if a.Flow.Name != "Курица с луком" {
			continue
		}
		for _, s := range a.Segments {
			if !s.Context && s.On.Name == diagram {
				seg = s
			}
		}
	}
	if seg == nil {
		t.Fatal("сегмент «Курицы с луком» на A0 не найден")
	}

	if len(seg.Points) != 4 {
		t.Errorf("точек %d, ожидалось 4 — как у прежнего шаблона: %s", len(seg.Points), format(seg))
	}
	placed := boxes(m)
	tomato, stew := placed["Подготовка томатной массы"], placed["Тушение"]
	lane := seg.Points[1].X.Val
	if lane <= tomato.X.Val+tomato.Width.Val || lane >= stew.X.Val {
		t.Errorf("вертикаль x=%v не в промежутке между «Подготовкой томатной массы» (правый край %v) и «Тушением» (левый край %v)",
			lane, tomato.X.Val+tomato.Width.Val, stew.X.Val)
	}
	checkSegment(t, "Курица с луком", seg, blocksByDiagram(m)[diagram], true)
}

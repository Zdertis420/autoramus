package layout_test

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
)

// Стрелки проверяются на настоящих документах проекта: skirt.yaml написан без
// единой координаты, а feedback.yaml и nested.yaml добавлены этой фичей —
// обратной связи и третьего уровня в наборе не было.

// document отдаёт путь к документу-фикстуре.
func document(name string) string {
	return filepath.Join("..", "..", "testdata", "documents", name)
}

// documents — набор, по которому идут проверки, верные для любой модели.
//
// Три документа `channels-*` добавлены фичей каналов. Прежний набор наложений
// не содержал вовсе — ни одного на шести документах, — и проверка на нём
// подтверждала бы не работу разведения, а собственную слепоту.
//
// `branch-border.yaml` добавлен фичей ветки на край листа по той же причине:
// потока, который и потребляется внутри диаграммы, и уходит за её край, не
// было ни в одном документе набора, и пропажу граничного сегмента не ловил
// никто.
//
// `examples/chakhokhbili.yaml` в этот перечень не входит, хотя наложений в нём
// больше, чем во всех пробах вместе. Причина названа и временна: в нём есть
// **другой** дефект — связь, перешагивающая один блок лестницы, идёт сквозь
// него, — и `TestGeometryIsLawful` на нём краснеет. Дефект к наложениям
// отношения не имеет и чинится не здесь, поэтому модель подключена к проверке
// наложений отдельным перечнем (`overlapDocuments` в channel_test.go), а в
// общий вернётся, когда прямые связи научатся обходить блоки.
func documents() []string {
	return []string{
		example("skirt.yaml"),
		example("skirt.json"),
		example("skirt-full.yaml"),
		document("feedback.yaml"),
		document("nested.yaml"),
		document("two-sides.yaml"),
		document("dfd-tunnel.yaml"),
		document("channels-pair.yaml"),
		document("channels-feedback.yaml"),
		document("channels-trunk.yaml"),
		document("branch-border.yaml"),
	}
}

// authored отмечает пары «поток + диаграмма», геометрию которых автор задал
// сам. Их раскладка не трогает, и требовать от них своих правил нельзя.
//
// Пара, а не поток целиком: единица оверрайда — пара, и сегменты того же
// потока на других диаграммах раскладка пишет сама. Отмечать весь поток
// значило бы снимать проверку и с них.
func authored(m *ir.Model) map[string]bool {
	out := make(map[string]bool)
	if m.Layout == nil {
		return out
	}
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			out[authoredKey(a.Flow.Name, s)] = true
		}
	}
	return out
}

// authoredKey — ключ пары. Контекстная диаграмма зовётся пустой строкой.
func authoredKey(flow string, s *ir.Segment) string {
	diagram := s.On.Name
	if s.Context {
		diagram = ""
	}
	return flow + "\x00" + diagram
}

// arrowOf находит стрелку потока.
func arrowOf(t *testing.T, m *ir.Model, flow string) *ir.ArrowLayout {
	t.Helper()
	for _, a := range m.Layout.Arrows {
		if a.Flow.Name == flow {
			return a
		}
	}
	t.Fatalf("в раскладке нет стрелки потока «%s»", flow)
	return nil
}

// segmentOn находит сегмент стрелки на названной диаграмме. Пустое имя — это
// контекстная A-0.
func segmentOn(t *testing.T, a *ir.ArrowLayout, diagram string) *ir.Segment {
	t.Helper()
	for _, s := range a.Segments {
		if (diagram == "" && s.Context) || (diagram != "" && !s.Context && s.On.Name == diagram) {
			return s
		}
	}
	t.Fatalf("у стрелки «%s» нет сегмента на диаграмме «%s»", a.Flow.Name, diagram)
	return nil
}

// TestEveryLinkDrawn — у каждой связи документа есть стрелка.
//
// Считается не число стрелок, а покрытие: одна связь может дать несколько
// сегментов (ветвление), но ни один поток, у которого есть связь, не вправе
// остаться без линии. Молчаливая потеря связи — то, ради чего фича и затевалась.
func TestEveryLinkDrawn(t *testing.T) {
	for _, path := range documents() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m := modelOf(t, path)
			layout.Apply(m)

			drawn := make(map[string]bool, len(m.Layout.Arrows))
			for _, a := range m.Layout.Arrows {
				drawn[a.Flow.Name] = true
				if len(a.Segments) == 0 {
					t.Errorf("у стрелки «%s» нет ни одного сегмента", a.Flow.Name)
				}
			}
			for _, l := range m.Links {
				if !drawn[l.Flow.Name] {
					t.Errorf("связь с потоком «%s» не нарисована", l.Flow.Name)
				}
			}
		})
	}
}

// TestSegmentsAreWellFormed — у каждого сегмента есть диаграмма, оба конца и
// ломаная, а концы — блок или край листа.
//
// Концом бывает блок, край листа или узел ветвления — больше ничем. Иной вид
// означал бы, что в раскладку заехало то, чего генератор записать не умеет.
func TestSegmentsAreWellFormed(t *testing.T) {
	for _, path := range documents() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m := modelOf(t, path)
			written := authored(m)
			layout.Apply(m)

			for _, a := range m.Layout.Arrows {
				for i, s := range a.Segments {
					if written[authoredKey(a.Flow.Name, s)] {
						// Этот сегмент написал автор. Узлы ветвления в языке
						// есть, и требовать от его записи нашей формы нельзя:
						// она и не наша. Сегменты того же потока на других
						// диаграммах писала раскладка, и с них спрос прежний.
						continue
					}
					where := fmt.Sprintf("«%s», сегмент %d", a.Flow.Name, i)
					if s.On.Name == "" {
						t.Errorf("%s: не назван диаграммой", where)
					}
					if s.From == nil || s.To == nil {
						t.Errorf("%s: конец висит неприсоединённым", where)
						continue
					}
					for _, e := range []*ir.Endpoint{s.From, s.To} {
						switch e.Kind() {
						case ir.EndpointFunction, ir.EndpointBorder, ir.EndpointNode:
						default:
							t.Errorf("%s: конец не блок, не край листа и не узел (%s)", where, e.Kind())
						}
					}
					if len(s.Points) < 2 {
						t.Errorf("%s: точек %d, ломаной не выходит", where, len(s.Points))
					}
				}
			}
		})
	}
}

// TestAttachPoints — крепление делит сторону на n+1.
//
// Числа снял с контекстной диаграммы `examples/тест.rsf`: два входа стоят в
// 1/3 и 2/3 высоты блока, одиночные управление, механизм и выход — в середине
// стороны. У «Изготовления юбки» на A-0 ровно такой же случай: два входа,
// одно управление, один выход и пять механизмов.
func TestAttachPoints(t *testing.T) {
	m := modelOf(t, example("skirt.yaml"))
	layout.Apply(m)

	root := boxes(m)["Изготовление юбки"]
	x, y := root.X.Val, root.Y.Val
	w, h := root.Width.Val, root.Height.Val

	tests := []struct {
		flow   string
		wantX  float64
		wantY  float64
		reason string
	}{
		{"Ткань", x, y + h/3, "первый из двух входов"},
		{"Фурнитура", x, y + 2*h/3, "второй из двух входов"},
		{"Правила изготовления", x + w/2, y, "единственное управление"},
		{"Юбка", x + w, y + h/2, "единственный выход"},
		{"Швея-закройщица", x + w/6, y + h, "первый из пяти механизмов"},
		{"Швейная машинка", x + 4*w/6, y + h, "четвёртый из пяти механизмов"},
	}

	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			seg := segmentOn(t, arrowOf(t, m, tt.flow), "")
			// Конец на блоке — тот, что не лежит на краю листа.
			at := seg.Points[0]
			if seg.From.Kind() == ir.EndpointBorder {
				at = seg.Points[len(seg.Points)-1]
			}
			if !near(at.X.Val, tt.wantX) || !near(at.Y.Val, tt.wantY) {
				t.Errorf("%s: крепление (%g, %g), ожидалось (%g, %g)",
					tt.reason, at.X.Val, at.Y.Val, tt.wantX, tt.wantY)
			}
		})
	}
}

// TestBranchingDrawsEveryConsumer — поток к нескольким потребителям доходит до
// каждого, и делает это одним деревом (FR-002, FR-003).
//
// «Правила изготовления» приходят ко всем трём работам A0 плюс к корневой на
// A-0. На A0 это дерево: один вход с края, два узла и три отвода — 2n−1 = 5
// сегментов при трёх потребителях, как у «контроля» в `тест.rsf`. На A-0
// потребитель один, ветвить нечего, и сегмент там ровно один.
func TestBranchingDrawsEveryConsumer(t *testing.T) {
	m := modelOf(t, example("skirt.yaml"))
	layout.Apply(m)

	arrow := arrowOf(t, m, "Правила изготовления")
	consumers := make(map[string]bool)
	for _, s := range arrow.Segments {
		if s.To != nil && s.To.Function.Name != "" {
			consumers[s.To.Function.Name] = true
		}
	}

	want := []string{"Изготовление юбки", "Раскрой материала", "Сшивание деталей", "Добавление фурнитуры"}
	for _, name := range want {
		if !consumers[name] {
			t.Errorf("«Правила изготовления» не доходят до работы «%s»", name)
		}
	}
	if got, want := len(arrow.Segments), 5+1; got != want {
		t.Errorf("сегментов %d, ожидалось %d (дерево на A0 плюс одиночный на A-0)", got, want)
	}

	// Край листа поток пересекает по разу на диаграмму: дубли, ради которых
	// дерево и заведено, ушли.
	// Диаграмма опознаётся именем вместе с пометкой Context: контекстная A-0 и
	// декомпозиция A0 зовутся одинаково и различаются только ею.
	entries := make(map[string]int)
	for _, s := range arrow.Segments {
		if s.From != nil && s.From.Kind() == ir.EndpointBorder {
			entries[fmt.Sprintf("%s/context=%v", s.On.Name, s.Context)]++
		}
	}
	for diagram, n := range entries {
		if n != 1 {
			t.Errorf("на диаграмме «%s» поток входит на лист %d раз, а должен один", diagram, n)
		}
	}
	if len(entries) != 2 {
		t.Errorf("диаграмм со входом с края %d, ожидалось 2", len(entries))
	}
}

// TestContextDiagram — ICOM корневой работы нарисован на A-0 от краёв листа.
func TestContextDiagram(t *testing.T) {
	m := modelOf(t, example("skirt.yaml"))
	layout.Apply(m)

	sides := map[string]string{
		"Ткань": ir.BorderLeft,
		"Правила изготовления": ir.BorderTop,
		"Швея-закройщица":      ir.BorderBottom,
		"Юбка":                 ir.BorderRight,
	}
	for flow, border := range sides {
		seg := segmentOn(t, arrowOf(t, m, flow), "")
		got := seg.From.Border
		if border == ir.BorderRight {
			got = seg.To.Border
		}
		if got != border {
			t.Errorf("«%s» на A-0 упирается в край %q, ожидался %q", flow, got, border)
		}
	}
}

// near сравнивает координаты с допуском: они считаются в плавающей точке, и
// требовать побитового равенства от суммы долей нельзя.
func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestDuplicateLinkCollapses — связь, записанная и списками ICOM, и в links,
// даёт одну стрелку, а не две (FR-005).
//
// Проверяется заодно и место на стороне блока: схлопывать надо до раздачи
// мест, иначе сторона делится на n+1 частей при завышенном n и одиночная
// стрелка садится не в середину, а в треть.
func TestDuplicateLinkCollapses(t *testing.T) {
	m := modelOf(t, filepath.Join("..", "..", "..", "specs", "008-arrow-tunnels-duplicates",
		"evidence", "duplicate-link.yaml"))
	layout.Apply(m)

	arrow := arrowOf(t, m, "Заготовка")
	if len(arrow.Segments) != 1 {
		t.Fatalf("сегментов %d, связь одна: %v", len(arrow.Segments), arrow.Segments)
	}

	second := boxes(m)["Вторая"]
	at := arrow.Segments[0].Points[len(arrow.Segments[0].Points)-1]
	if want := second.Y.Val + second.Height.Val/2; !near(at.Y.Val, want) {
		t.Errorf("вход «Второй» на y=%g, ожидалась середина стороны y=%g: "+
			"место считалось на две стрелки вместо одной", at.Y.Val, want)
	}
}

// TestDifferentSidesAreNotDuplicates — один поток к той же работе двумя
// сторонами даёт две стрелки (FR-007).
//
// Сторона приёмника входит в ключ схлопывания именно ради этого случая:
// «Заготовка» и подаётся на вход, и управляет — это две разные стрелки, и
// склеить их значило бы потерять управление.
func TestDifferentSidesAreNotDuplicates(t *testing.T) {
	m := modelOf(t, document("two-sides.yaml"))
	layout.Apply(m)

	sides := make(map[string]bool)
	for _, s := range arrowOf(t, m, "Заготовка").Segments {
		if s.To != nil && s.To.Function.Name == "Вторая" {
			sides[s.To.Side] = true
		}
	}
	for _, want := range []string{ir.SideIn, ir.SideControl} {
		if !sides[want] {
			t.Errorf("нет стрелки «Заготовки» к «Второй» стороной %s", want)
		}
	}
}

// borderSegments отдаёт сегменты стрелки, кончающиеся на краю листа названной
// диаграммы-декомпозиции. Контекстная A-0 не в счёт: там край листа — норма, и
// наличие её сегмента ничего не говорит о том, сшиты ли уровни ниже.
func borderSegments(a *ir.ArrowLayout, diagram, border string) []*ir.Segment {
	var out []*ir.Segment
	for _, s := range branchSegments(a, diagram) {
		if s.To != nil && s.To.Kind() == ir.EndpointBorder && s.To.Border == border {
			out = append(out, s)
		}
	}
	return out
}

// TestExportedFlowReachesBorder — поток, объявленный выходом диаграммы, доходит
// до её края, даже если внутри его кто-то потребляет.
//
// «Журнал» производит «Обработка», потребляет «Сборка», и он же стоит в `out` у
// «Пробы». На A-0 он уходит из блока «Пробы» вправо, и внутри декомпозиции ему
// обязано быть во что перейти. Прежде половинка «работа → наружу»
// отбрасывалась, едва у потока находился потребитель, и уровни расходились
// молча: в файле стрелка есть, а связи между уровнями нет.
func TestExportedFlowReachesBorder(t *testing.T) {
	m := modelOf(t, document("branch-border.yaml"))
	layout.Apply(m)

	journal := arrowOf(t, m, "журнал")
	if got := borderSegments(journal, "Проба", ir.BorderRight); len(got) != 1 {
		t.Errorf("сегментов «журнала» до правого края %d, ожидался один: %s",
			len(got), formatAll(branchSegments(journal, "Проба")))
	}

	// Потребитель никуда не делся: ветка на край добавляется, а не подменяет.
	var toBlock int
	for _, s := range branchSegments(journal, "Проба") {
		if s.To != nil && s.To.Kind() == ir.EndpointFunction {
			toBlock++
		}
	}
	if toBlock != 1 {
		t.Errorf("отводов «журнала» в блоки %d, потребитель один", toBlock)
	}
}

// TestTunnelledFlowStaysInside — туннель у владельца диаграммы отменяет ветку
// на край.
//
// Автор сказал списком `tunnel`, что поток намеренно не переходит на уровень
// выше; дорисовывать ему выход значило бы спорить с автором. Тест охраняет, а
// не требует: он зелёный и до фичи, и обязан остаться зелёным после.
func TestTunnelledFlowStaysInside(t *testing.T) {
	cases := []struct{ path, flow, diagram string }{
		{document("dfd-tunnel.yaml"), "Принятая заявка", "Приём заявок"},
		{example("skirt-full.yaml"), "Заявка в работе", "Приём заказа"},
	}
	for _, c := range cases {
		t.Run(filepath.Base(c.path), func(t *testing.T) {
			m := modelOf(t, c.path)
			layout.Apply(m)

			arrow := arrowOf(t, m, c.flow)
			if got := borderSegments(arrow, c.diagram, ir.BorderRight); len(got) != 0 {
				t.Errorf("туннелированный поток «%s» дошёл до края листа: %s",
					c.flow, formatAll(got))
			}
		})
	}
}

// formatAll печатает ломаные сегментов — без них сообщение об ошибке говорит
// «не сошлось» и молчит о том, что именно.
func formatAll(segments []*ir.Segment) string {
	out := ""
	for _, s := range segments {
		if out != "" {
			out += "; "
		}
		out += format(s)
	}
	if out == "" {
		return "(ни одного)"
	}
	return out
}

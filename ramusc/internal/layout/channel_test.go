package layout_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
)

// Каналы: две стрелки не вправе лечь на одну линию так, чтобы слиться в одну.
//
// Пересечение в точке допустимо и неизбежно в любой нетривиальной модели —
// проверяется только наложение отрезком. Разница между тем и другим измерима:
// общая часть двух сегментов имеет нулевую длину или ненулевую.

// overlapDocuments — набор для проверки наложений.
//
// Шире общего: сюда входит `examples/chakhokhbili.yaml`, в общий перечень не
// попавшая из-за постороннего дефекта (см. комментарий к documents()). Наложений
// в ней больше, чем во всех пробах вместе, и отказываться от неё из-за чужой
// беды было бы расточительством.
func overlapDocuments() []string {
	return append(documents(), example("chakhokhbili.yaml"))
}

// piece — прямолинейный участок ломаной.
type piece struct {
	flow     string
	diagram  string
	vertical bool
	coord    float64 // общая координата: x у вертикали, y у горизонтали
	lo, hi   float64 // протяжённость вдоль линии
}

// pieces разбирает раскладку на участки.
//
// Диагональных участков не бывает: маршруты строятся из горизонталей и
// вертикалей. Наискось идущий отрезок — сам по себе ошибка, и тест обязан
// сказать о ней, а не молча пропустить участок, который не смог разобрать.
func pieces(t *testing.T, m *ir.Model) []piece {
	t.Helper()

	var out []piece
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			for i := 1; i < len(s.Points); i++ {
				p, q := s.Points[i-1], s.Points[i]
				x1, y1 := p.X.Val, p.Y.Val
				x2, y2 := q.X.Val, q.Y.Val

				switch {
				case near(x1, x2) && near(y1, y2):
					// Вырожденный отрезок нулевой длины: его убирает clean().
				case near(x1, x2):
					lo, hi := minMax(y1, y2)
					out = append(out, piece{a.Flow.Name, diagramOf(s), true, x1, lo, hi})
				case near(y1, y2):
					lo, hi := minMax(x1, x2)
					out = append(out, piece{a.Flow.Name, diagramOf(s), false, y1, lo, hi})
				default:
					t.Errorf("«%s»: отрезок %d идёт наискось: (%g, %g) → (%g, %g)",
						a.Flow.Name, i, x1, y1, x2, y2)
				}
			}
		}
	}
	return out
}

// sharedLength — длина общей части двух участков одной линии.
func sharedLength(a, b piece) float64 {
	lo, hi := a.lo, a.hi
	if b.lo > lo {
		lo = b.lo
	}
	if b.hi < hi {
		hi = b.hi
	}
	return hi - lo
}

// sameLine сообщает, что участки лежат на одной прямой.
//
// Координаты сравниваются точно. Совпадение рождается из одной и той же
// формулы, применённой дважды, и совпадает побитово; порог лечил бы болезнь,
// которой нет, и заводил бы вопрос «а какой порог».
func sameLine(a, b piece) bool {
	return a.diagram == b.diagram && a.vertical == b.vertical && a.coord == b.coord
}

// TestNoSegmentOverlap — сегменты разных стрелок не накладываются.
//
// Стрелка — пара «поток + диаграмма». Участки одной стрелки пропускаются: их
// общая магистраль намеренна, она и есть ветвление.
func TestNoSegmentOverlap(t *testing.T) {
	for _, path := range overlapDocuments() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m := modelOf(t, path)
			layout.Apply(m)

			all := pieces(t, m)
			for i, a := range all {
				for _, b := range all[i+1:] {
					if a.flow == b.flow || !sameLine(a, b) {
						continue
					}
					if length := sharedLength(a, b); length > 0 {
						t.Errorf("диаграмма «%s»: «%s» и «%s» лежат на %s и делят отрезок длиной %g",
							a.diagram, a.flow, b.flow, lineName(a), length)
					}
				}
			}
		})
	}
}

// lineName называет линию так, как её увидит человек.
func lineName(p piece) string {
	if p.vertical {
		return fmt.Sprintf("вертикали x=%g", p.coord)
	}
	return fmt.Sprintf("горизонтали y=%g", p.coord)
}

// TestSharedLineDisjointSpansUnmoved — общая линия ещё не наложение.
//
// На A0 «чахохбили» три пары стрелок делят горизонталь, не перекрываясь по
// длине: вход, стоящий на середине высоты блока, и единственный выход того же
// блока — тоже на середине. «Курица» и «Куски курицы» у «Нарезки курицы»,
// «Соль» (четвёртая из семи) и «Тушёная курица» у «Тушения», «Чеснок» (второй
// из трёх) и «Чахохбили» у «Финальной приправки». Они различимы прекрасно, и
// разводить их незачем — иначе рисунок разъезжался бы без нужды (FR-008).
//
// Мера снята с настоящей модели: правило «сравниваем протяжённость, а не
// координату» без неё выглядело бы придиркой. Линия берётся серединой блока,
// а не числом: прежде здесь стояли y=298.8 и y=347.2, и они уехали, когда
// блоки начали расти под свои стрелки (specs/012-arrow-port-spacing), — пары
// же остались парами.
func TestSharedLineDisjointSpansUnmoved(t *testing.T) {
	m := modelOf(t, example("chakhokhbili.yaml"))
	layout.Apply(m)

	const diagram = "Приготовление чахохбили"
	middle := func(name string) float64 {
		b := boxes(m)[name]
		return b.Y.Val + b.Height.Val/2
	}
	want := []struct {
		y     float64
		flows [2]string
	}{
		{middle("Нарезка курицы"), [2]string{"Курица", "Куски курицы"}},
		{middle("Тушение"), [2]string{"Соль", "Тушёная курица"}},
		{middle("Финальная приправка"), [2]string{"Чеснок", "Чахохбили"}},
	}

	on := make(map[float64]map[string]bool)
	for _, p := range pieces(t, m) {
		if p.vertical || p.diagram != diagram {
			continue
		}
		if on[p.coord] == nil {
			on[p.coord] = make(map[string]bool)
		}
		on[p.coord][p.flow] = true
	}

	for _, w := range want {
		for _, flow := range w.flows {
			if !on[w.y][flow] {
				t.Errorf("«%s» ушёл с горизонтали y=%g, хотя делил её мирно: там %s",
					flow, w.y, flowsOn(on[w.y]))
			}
		}
	}
}

// flowsOn перечисляет потоки линии в устойчивом порядке.
func flowsOn(set map[string]bool) string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return fmt.Sprint(out)
}

// TestChannelsAreStable — разведение зависит только от текста документа.
//
// Одна и та же модель раскладывается дважды; координаты обязаны совпасть
// побитово. Обход отображения, попавший в раздачу каналов, всплывёт здесь, а не
// в чужом диффе через три месяца (принцип III конституции).
func TestChannelsAreStable(t *testing.T) {
	for _, path := range overlapDocuments() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			first, second := modelOf(t, path), modelOf(t, path)
			layout.Apply(first)
			layout.Apply(second)

			was, now := pieces(t, first), pieces(t, second)
			if len(was) != len(now) {
				t.Fatalf("участков стало %d вместо %d", len(now), len(was))
			}
			for i := range was {
				if was[i] != now[i] {
					t.Errorf("участок %d разошёлся: %+v против %+v", i, was[i], now[i])
				}
			}
		})
	}
}

// TestCanonicalLaneKept — с канонической линии уходит только тот, кому пришлось.
//
// На `channels-pair.yaml` два потока идут между одной парой работ и просят одну
// вертикаль. Первый по порядку документа обязан остаться ровно там, где его
// поставила формула, — посередине промежутка, — а подвинуться обязан второй, и
// ровно на один шаг канала.
//
// Это и есть FR-008 в самом узком месте: разведение не вправе двигать рисунок
// шире необходимого. Что документ, где разводить нечего вовсе, не меняется ни на
// йоту, проверено иначе — побайтовым сравнением файлов (quickstart.md, шаг 4).
func TestCanonicalLaneKept(t *testing.T) {
	m := modelOf(t, document("channels-pair.yaml"))
	layout.Apply(m)

	// Середина промежутка между «Раскроем» и «Сборкой»: блоки лестницы стоят
	// с шагом 193⅓ при ширине 72, и правый край первого — 152, левый второго —
	// 370.
	const (
		canonical = 261.0
		// Шаг канала. Повторён здесь числом намеренно: тест живёт во внешнем
		// пакете и обязан ловить молчаливое изменение константы, а не следовать
		// за ним. Так же поступает place_test.go с границами листа.
		step = 6.0
	)

	lanes := make(map[string]float64)
	for _, p := range pieces(t, m) {
		if p.vertical && p.diagram == "Проба" && (p.flow == "первый" || p.flow == "второй") {
			lanes[p.flow] = p.coord
		}
	}

	if lanes["первый"] != canonical {
		t.Errorf("«первый» съехал на x=%g, хотя пришёл за линией первым: канон %g",
			lanes["первый"], canonical)
	}
	if got, want := lanes["второй"], canonical+step; got != want {
		t.Errorf("«второй» встал на x=%g, ожидалось %g — один шаг канала от канона",
			got, want)
	}
}

// TestAuthoredGeometryIsObstacle — автоматическая стрелка обходит авторскую.
//
// Геометрию из секции layout раскладка не трогает (Р2), но и не имеет права не
// видеть: иначе автомат ляжет поверх авторской линии и не заметит. Проверяется
// на `skirt-full.yaml` — единственном документе набора, где автор рисовал
// стрелки руками.
func TestAuthoredGeometryIsObstacle(t *testing.T) {
	m := modelOf(t, example("skirt-full.yaml"))

	// Что автор написал — запоминаем до раскладки: после неё в списке лежит и
	// дописанное.
	type line struct {
		diagram  string
		flow     string
		vertical bool
		coord    float64
		lo, hi   float64
	}
	var byAuthor []line
	for _, a := range m.Layout.Arrows {
		for _, s := range a.Segments {
			for i := 1; i < len(s.Points); i++ {
				p, q := s.Points[i-1], s.Points[i]
				switch {
				case p.X.Val == q.X.Val && p.Y.Val == q.Y.Val:
				case p.X.Val == q.X.Val:
					lo, hi := minMax(p.Y.Val, q.Y.Val)
					byAuthor = append(byAuthor, line{diagramOf(s), a.Flow.Name, true, p.X.Val, lo, hi})
				case p.Y.Val == q.Y.Val:
					lo, hi := minMax(p.X.Val, q.X.Val)
					byAuthor = append(byAuthor, line{diagramOf(s), a.Flow.Name, false, p.Y.Val, lo, hi})
				}
			}
		}
	}
	if len(byAuthor) == 0 {
		t.Fatal("в документе нет авторской геометрии — проверять нечего")
	}

	layout.Apply(m)

	for _, p := range pieces(t, m) {
		for _, w := range byAuthor {
			if w.diagram != p.diagram || w.vertical != p.vertical || w.coord != p.coord {
				continue
			}
			if w.flow == p.flow {
				continue // это и есть авторская линия, она на месте
			}
			if minf(p.hi, w.hi)-maxf(p.lo, w.lo) > 0 {
				t.Errorf("диаграмма «%s»: «%s» легла на авторскую линию «%s» (%s)",
					p.diagram, p.flow, w.flow, lineName(p))
			}
		}
	}
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

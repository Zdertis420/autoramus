package layout_test

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
)

// model собирает модель из имён работ вручную: проверять размещение на
// настоящих документах неудобно — их работ ровно столько, сколько есть, а
// поведение надо знать при любом числе.
func model(parent string, children ...string) *ir.Model {
	m := &ir.Model{Name: ir.Ref{Name: parent}}
	m.Functions = append(m.Functions, &ir.Function{Name: ir.Ref{Name: parent}})
	for _, name := range children {
		m.Functions = append(m.Functions, &ir.Function{
			Name: ir.Ref{Name: name},
			Of:   ir.Ref{Name: parent},
		})
	}
	return m
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('A'+i%26)) + string(rune('0'+i/26))
	}
	return out
}

// TestDiagonalForThree — три блока на диаграмме.
//
// Числа посчитаны по формуле, снятой с тест.rsf: шаг 580/2 и 242/2. Ровно так
// должен выглядеть examples/skirt.yaml, ради которого фича и делается.
func TestDiagonalForThree(t *testing.T) {
	m := model("корень", "первая", "вторая", "третья")
	layout.Apply(m)

	want := []struct {
		name string
		x, y float64
		w, h float64
	}{
		{"первая", 80, 80, 72, 50.4},
		{"вторая", 370, 201, 72, 50.4},
		{"третья", 660, 322, 72, 50.4},
	}
	placed := boxes(m)
	for _, w := range want {
		got, ok := placed[w.name]
		if !ok {
			t.Errorf("работа «%s» не размещена", w.name)
			continue
		}
		if got.X.Val != w.x || got.Y.Val != w.y || got.Width.Val != w.w || got.Height.Val != w.h {
			t.Errorf("«%s»: (%v, %v, %v, %v), ожидалось (%v, %v, %v, %v)",
				w.name, got.X.Val, got.Y.Val, got.Width.Val, got.Height.Val, w.x, w.y, w.w, w.h)
		}
	}
}

// TestSingleBlockGoesToSheetCentre — одна работа на диаграмме.
//
// Формула вырождается: делить диагональ не на что. Наблюдать, куда Ramus кладёт
// единственный блок, не на чем — у корневых работ трёх моделей центры разные, и
// в двух из трёх блок явно растянут руками. Центр листа — единственный выбор,
// который не выдаёт догадку за измерение (Р0-3).
func TestSingleBlockGoesToSheetCentre(t *testing.T) {
	m := model("корень")
	layout.Apply(m)

	root := boxes(m)["корень"]
	if root == nil {
		t.Fatal("единственная работа не размещена")
	}
	const centreX, centreY = (7 + 793) / 2.0, (7 + 437) / 2.0
	if got := root.X.Val + root.Width.Val/2; got != centreX {
		t.Errorf("центр блока по горизонтали %v, ожидался центр листа %v", got, centreX)
	}
	if got := root.Y.Val + root.Height.Val/2; got != centreY {
		t.Errorf("центр блока по вертикали %v, ожидался центр листа %v", got, centreY)
	}
}

// TestTwoBlocksTakeDiagonalEnds — два блока садятся в концы диагонали.
func TestTwoBlocksTakeDiagonalEnds(t *testing.T) {
	m := model("корень", "первая", "вторая")
	layout.Apply(m)

	placed := boxes(m)
	if got := placed["первая"]; got == nil || got.X.Val != 80 || got.Y.Val != 80 {
		t.Errorf("первая: %+v, ожидалось начало диагонали (80, 80)", got)
	}
	if got := placed["вторая"]; got == nil || got.X.Val != 660 || got.Y.Val != 322 {
		t.Errorf("вторая: %+v, ожидался конец диагонали (660, 322)", got)
	}
}

// Лист. Измерено по крайним точкам стрелок «Изготовления юбки».
const (
	sheetLeft, sheetRight = 7.0, 793.0
	sheetTop, sheetBottom = 7.0, 437.0
)

// overlap — пересекаются ли прямоугольники. Блоки пересекаются, только когда
// перекрываются и по горизонтали, и по вертикали: диагональ разносит их по X,
// и вертикальная теснота сама по себе безвредна.
func overlap(a, b *ir.FunctionLayout) bool {
	byX := a.X.Val < b.X.Val+b.Width.Val && b.X.Val < a.X.Val+a.Width.Val
	byY := a.Y.Val < b.Y.Val+b.Height.Val && b.Y.Val < a.Y.Val+a.Height.Val
	return byX && byY
}

// TestFitsAndDoesNotOverlap — до двадцати пяти работ блоки не наезжают и не
// вылезают за лист.
//
// Двадцать пять — названная граница (FR-003): дальше шаг становится меньше
// наименьшего блока, и лист физически не вмещает диагональ. Это далеко за
// каноном IDEF0 (3–6 блоков на диаграмме).
func TestFitsAndDoesNotOverlap(t *testing.T) {
	for n := 1; n <= 25; n++ {
		m := model("корень", names(n)...)
		layout.Apply(m)

		var placed []*ir.FunctionLayout
		for _, f := range m.Functions {
			if f.Of.Name == "" {
				continue
			}
			box, ok := boxes(m)[f.Name.Name]
			if !ok {
				t.Fatalf("n=%d: работа «%s» не размещена", n, f.Name.Name)
			}
			placed = append(placed, box)
		}

		for i, box := range placed {
			if box.X.Val < sheetLeft || box.X.Val+box.Width.Val > sheetRight {
				t.Errorf("n=%d: блок %d вышел за лист по горизонтали: %v..%v",
					n, i, box.X.Val, box.X.Val+box.Width.Val)
			}
			if box.Y.Val < sheetTop || box.Y.Val+box.Height.Val > sheetBottom {
				t.Errorf("n=%d: блок %d вышел за лист по вертикали: %v..%v",
					n, i, box.Y.Val, box.Y.Val+box.Height.Val)
			}
			for j := i + 1; j < len(placed); j++ {
				if overlap(box, placed[j]) {
					t.Errorf("n=%d: блоки %d и %d накладываются", n, i, j)
				}
			}
		}
	}
}

// TestSizeThresholds — граница поведения названа числом, а не «примерно».
//
// До девяти блоков диагональ разносит их по горизонтали, и размер остаётся
// таким же, как у Ramus. С десяти блок мельчает, сохраняя пропорцию.
func TestSizeThresholds(t *testing.T) {
	sizeAt := func(n int) (float64, float64) {
		m := model("корень", names(n)...)
		layout.Apply(m)
		box := boxes(m)[names(n)[0]]
		return box.Width.Val, box.Height.Val
	}

	if w, h := sizeAt(9); w != 72 || h != 50.4 {
		t.Errorf("при девяти блоках размер (%v, %v), ожидался прежний (72, 50.4)", w, h)
	}
	w, h := sizeAt(10)
	if w >= 72 {
		t.Errorf("при десяти блоках ширина %v, ожидалось сжатие", w)
	}
	if got := h / w; got < 0.699 || got > 0.701 {
		t.Errorf("пропорция при сжатии %v, ожидалось 0.7", got)
	}
}

// Рост под стрелки (specs/012-arrow-port-spacing). Модели собираются вручную,
// как и выше: поведение надо знать при любом числе стрелок, а в настоящих
// документах их ровно столько, сколько есть.

// crowd вешает на сторону работы потоки, приходящие с края листа: у каждого
// потребитель есть, производителя на диаграмме нет. Выход — наоборот:
// производитель есть, потребителя нет, и поток уходит за край. Половинки
// связей — те же, что строит ir.Build из списков ICOM.
func crowd(m *ir.Model, function, side string, n int) {
	for i := range n {
		flow := fmt.Sprintf("%s/%s/%d", function, side, i)
		if side == ir.SideOut {
			m.Links = append(m.Links, produces(function, flow))
			continue
		}
		m.Links = append(m.Links, &ir.Link{
			Flow: ir.Ref{Name: flow}, To: ir.Ref{Name: function}, Side: side, Sugar: true,
		})
	}
}

// TestBlockGrowsAlongCrowdedSide — блок растёт по той оси, вдоль которой
// тесно, и ровно до (n+1)·15; другая ось остаётся прежней (FR-004–FR-006).
func TestBlockGrowsAlongCrowdedSide(t *testing.T) {
	type sides struct{ in, control, mechanism, out int }
	cases := []struct {
		name          string
		sides         sides
		width, height float64
	}{
		{"три входа", sides{in: 3}, 72, 60},
		{"четыре механизма", sides{mechanism: 4}, 75, 50.4},
		{"тесно слева и снизу", sides{in: 4, mechanism: 5}, 90, 75},
		{"семь входов, два механизма", sides{in: 7, mechanism: 2}, 72, 120},
		{"выходов больше, чем входов", sides{in: 1, out: 5}, 72, 90},
		{"две стрелки на каждой стороне", sides{2, 2, 2, 2}, 72, 50.4},
		{"одна стрелка", sides{in: 1}, 72, 50.4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := model("корень", "первая", "вторая", "третья")
			crowd(m, "вторая", ir.SideIn, c.sides.in)
			crowd(m, "вторая", ir.SideControl, c.sides.control)
			crowd(m, "вторая", ir.SideMechanism, c.sides.mechanism)
			crowd(m, "вторая", ir.SideOut, c.sides.out)
			layout.Apply(m)

			got := boxes(m)["вторая"]
			if got.Width.Val != c.width || got.Height.Val != c.height {
				t.Errorf("размер %v × %v, ожидался %v × %v",
					got.Width.Val, got.Height.Val, c.width, c.height)
			}
			for _, other := range []string{"первая", "третья"} {
				if b := boxes(m)[other]; b.Width.Val != 72 || b.Height.Val != 50.4 {
					t.Errorf("«%s» без стрелок выросла до %v × %v", other, b.Width.Val, b.Height.Val)
				}
			}
		})
	}
}

// TestSingleBlockGrowsAroundCentre — единственный блок диаграммы растёт вокруг
// центра листа, где и стоял (research Р-7). Двенадцать входов — как у A-0
// «чахохбили».
func TestSingleBlockGrowsAroundCentre(t *testing.T) {
	m := model("корень")
	crowd(m, "корень", ir.SideIn, 12)
	layout.Apply(m)

	b := boxes(m)["корень"]
	if b.Width.Val != 72 || b.Height.Val != 195 {
		t.Errorf("размер %v × %v, ожидался 72 × 195", b.Width.Val, b.Height.Val)
	}
	cx, cy := b.X.Val+b.Width.Val/2, b.Y.Val+b.Height.Val/2
	if math.Abs(cx-(sheetLeft+sheetRight)/2) > 1e-9 || math.Abs(cy-(sheetTop+sheetBottom)/2) > 1e-9 {
		t.Errorf("центр ушёл в (%v, %v), ожидался центр листа", cx, cy)
	}
}

// checkLadder требует от блоков одной диаграммы того, что делает их лестницей:
// каждый следующий правее и ниже предыдущего, соседи не пересекаются и между
// ними остаётся промежуток под маршруты, всё внутри листа (FR-007, SC-004).
func checkLadder(t *testing.T, blocks []*ir.FunctionLayout) {
	t.Helper()
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].X.Val < blocks[j].X.Val })
	const eps = 1e-9
	for i, b := range blocks {
		if b.X.Val < sheetLeft || b.X.Val+b.Width.Val > sheetRight ||
			b.Y.Val < sheetTop || b.Y.Val+b.Height.Val > sheetBottom {
			t.Errorf("«%s» вне листа: (%v, %v) %v × %v",
				b.Function.Name, b.X.Val, b.Y.Val, b.Width.Val, b.Height.Val)
		}
		if i == 0 {
			continue
		}
		prev := blocks[i-1]
		if gapX := b.X.Val - (prev.X.Val + prev.Width.Val); gapX < 6-eps {
			t.Errorf("между «%s» и «%s» по горизонтали %v — меньше просвета",
				prev.Function.Name, b.Function.Name, gapX)
		}
		if b.Y.Val <= prev.Y.Val {
			t.Errorf("«%s» не ниже «%s»: y %v против %v — лестница сломалась",
				b.Function.Name, prev.Function.Name, b.Y.Val, prev.Y.Val)
		}
	}
}

// TestGrownLadder — выросшие блоки остаются лестницей.
func TestGrownLadder(t *testing.T) {
	t.Run("chakhokhbili.yaml", func(t *testing.T) {
		m := modelOf(t, example("chakhokhbili.yaml"))
		layout.Apply(m)
		for owner, blocks := range blocksByDiagram(m) {
			if len(blocks) < 2 {
				continue
			}
			t.Run(owner, func(t *testing.T) { checkLadder(t, blocks) })
		}
	})

	// Шесть работ, как на A0 «чахохбили»: у пятой семь входов, у четвёртой
	// пять механизмов — и высота, и ширина растут посреди лестницы.
	t.Run("шесть работ", func(t *testing.T) {
		children := names(6)
		m := model("корень", children...)
		crowd(m, children[4], ir.SideIn, 7)
		crowd(m, children[3], ir.SideMechanism, 5)
		layout.Apply(m)
		checkLadder(t, blocksByDiagram(m)["корень"])
	})

	// Крайний случай (research И5): одна высокая работа рядом с низкой.
	// Сумма высот съедает весь вертикальный размах, и без страховки второй
	// блок встал бы выше первого.
	t.Run("высокая рядом с низкой", func(t *testing.T) {
		m := model("корень", "низкая", "высокая")
		crowd(m, "высокая", ir.SideIn, 19)
		layout.Apply(m)
		blocks := blocksByDiagram(m)["корень"]
		if h := boxes(m)["высокая"].Height.Val; h != 300 {
			t.Fatalf("высота %v, ожидалась 300 — случай не тот, что проверяется", h)
		}
		checkLadder(t, blocks)
	})
}

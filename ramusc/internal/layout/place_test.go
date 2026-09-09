package layout_test

import (
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

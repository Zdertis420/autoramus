package layout

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
)

// TestDetourFallsBackToCorridor — стрелке, у которой прежний шаблон и все
// полосы между блоками закрыты, остаётся путь через коридор над блоками, а
// если закрыт и он — под ними (specs/014-arrows-avoid-blocks, research И2).
// Чистый шаблон при этом не трогается.
func TestDetourFallsBackToCorridor(t *testing.T) {
	source := box{x: 100, y: 100, width: 72, height: 50, owner: "источник"}
	target := box{x: 400, y: 100, width: 72, height: 50, owner: "приёмник"}
	a := &arrow{
		flow: "поток",
		from: end{function: "источник", side: ir.SideOut, place: slot{1, 1}},
		to:   end{function: "приёмник", side: ir.SideIn, place: slot{1, 1}},
	}
	from, to := attach(source, a.from), attach(target, a.to)

	t.Run("чистый шаблон не меняется", func(t *testing.T) {
		blocks := []box{source, target}
		got := route(a, from, to, blocks)
		want := clean(forward(a, from, to, blocks))
		if !samePoints(got, want) {
			t.Errorf("маршрут %v, прежний шаблон %v", got, want)
		}
	})

	t.Run("коридор над блоками", func(t *testing.T) {
		// Помеха закрывает и середину промежутка, и обе полосы по краям: любая
		// горизонталь на высоте выхода идёт сквозь неё.
		wall := box{x: 200, y: 90, width: 170, height: 210, owner: "помеха"}
		blocks := []box{source, target, wall}
		got := route(a, from, to, blocks)
		if crossesRoute(got, blocks) {
			t.Fatalf("маршрут %v задевает блок", got)
		}
		if above := corridorAbove(blocks); !passesAt(got, above) {
			t.Errorf("маршрут %v не идёт коридором над блоками (y=%v)", got, above)
		}
	})

	t.Run("коридор под блоками", func(t *testing.T) {
		// Помеха от самого верха листа: коридору над блоками места нет.
		wall := box{x: 200, y: sheetTop, width: 170, height: 300, owner: "помеха"}
		blocks := []box{source, target, wall}
		got := route(a, from, to, blocks)
		if crossesRoute(got, blocks) {
			t.Fatalf("маршрут %v задевает блок", got)
		}
		if below := corridorBelow(blocks); !passesAt(got, below) {
			t.Errorf("маршрут %v не идёт коридором под блоками (y=%v)", got, below)
		}
	})
}

func samePoints(a, b []point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// passesAt — у ломаной есть горизонталь на высоте y.
func passesAt(points []point, y float64) bool {
	for i := 1; i < len(points); i++ {
		if points[i-1].y == y && points[i].y == y {
			return true
		}
	}
	return false
}

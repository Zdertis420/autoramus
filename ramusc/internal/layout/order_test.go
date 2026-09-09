package layout_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
)

// order отдаёт имена работ диаграммы в порядке их размещения на диагонали:
// левее и выше — раньше.
func order(m *ir.Model, parent string) []string {
	type placed struct {
		name string
		x    float64
	}
	var all []placed
	for _, f := range m.Functions {
		if f.Of.Name != parent {
			continue
		}
		if box, ok := boxes(m)[f.Name.Name]; ok {
			all = append(all, placed{f.Name.Name, box.X.Val})
		}
	}
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].x < all[j-1].x; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	out := make([]string, len(all))
	for i, p := range all {
		out[i] = p.name
	}
	return out
}

// link — половинка связи, как её строит ir.Build из списков ICOM: у записи из
// in/control/mechanism заполнено только To, у записи из out — только From.
func produces(f, flow string) *ir.Link {
	return &ir.Link{Flow: ir.Ref{Name: flow}, From: ir.Ref{Name: f}, Sugar: true}
}

func consumes(f, flow string) *ir.Link {
	return &ir.Link{Flow: ir.Ref{Name: flow}, To: ir.Ref{Name: f}, Side: ir.SideIn, Sugar: true}
}

// TestOrderFollowsLinks — порядок берётся из связей, а не из объявления.
//
// Работы объявлены наоборот: сначала последняя по потоку, затем первая. Порядок
// на диаграмме обязан их развернуть, иначе стрелки пойдут справа налево.
func TestOrderFollowsLinks(t *testing.T) {
	m := model("корень", "третья", "вторая", "первая")
	m.Links = []*ir.Link{
		produces("первая", "a"), consumes("вторая", "a"),
		produces("вторая", "b"), consumes("третья", "b"),
	}
	layout.Apply(m)

	want := []string{"первая", "вторая", "третья"}
	got := order(m, "корень")
	if len(got) != len(want) {
		t.Fatalf("размещено %d работ, ожидалось %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("порядок %v, ожидался %v", got, want)
			break
		}
	}
}

// TestOrderKeepsDeclarationWhenLinksAreSilent — при равенстве порядок берётся
// из документа: он обычно осмыслен (Р3).
func TestOrderKeepsDeclarationWhenLinksAreSilent(t *testing.T) {
	m := model("корень", "вторая", "первая", "третья")
	layout.Apply(m)

	want := []string{"вторая", "первая", "третья"}
	got := order(m, "корень")
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("порядок %v, ожидался порядок объявления %v", got, want)
		}
	}
}

// TestOrderSurvivesFeedback — обратная связь не зацикливает раскладку.
//
// Цикл в потоках законен: это стрелка обратной связи IDEF0, и валидатор её не
// запрещает. Запрещены только циклы в иерархии работ.
func TestOrderSurvivesFeedback(t *testing.T) {
	m := model("корень", "первая", "вторая")
	m.Links = []*ir.Link{
		produces("первая", "вперёд"), consumes("вторая", "вперёд"),
		produces("вторая", "назад"), consumes("первая", "назад"),
	}

	done := make(chan struct{})
	go func() {
		layout.Apply(m)
		close(done)
	}()
	<-done

	if got := order(m, "корень"); len(got) != 2 {
		t.Fatalf("размещено %d работ, ожидалось 2: %v", len(got), got)
	}
}

// TestOrderIsStable — два прогона дают одно и то же.
//
// Обход отображений в Go случаен от запуска к запуску, и порядок блоков обязан
// от него не зависеть (принцип III конституции).
func TestOrderIsStable(t *testing.T) {
	build := func() []string {
		m := model("корень", "а", "б", "в", "г", "д")
		m.Links = []*ir.Link{
			produces("а", "x"), consumes("б", "x"),
			produces("в", "y"), consumes("г", "y"),
		}
		layout.Apply(m)
		return order(m, "корень")
	}

	first := build()
	for i := 0; i < 10; i++ {
		if got := build(); !equal(got, first) {
			t.Fatalf("порядок поплыл: %v против %v", got, first)
		}
	}
}

func equal(a, b []string) bool {
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

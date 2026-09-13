package rsf_test

import (
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// TestTunnelsOnRealModels — сверка правила туннелирования с настоящими файлами.
//
// Три модели из четырёх Ramus записал сам, и туннелей на их диаграммах нет.
// Ноль здесь — не украшение: будь правило выведено неверно, скобки нашлись бы
// там, где их на картинке не видно. Единственный туннель во всём наборе —
// в «ФормированииТП», и он нарисован автором модели.
func TestTunnelsOnRealModels(t *testing.T) {
	for _, fx := range fixtures.All() {
		t.Run(fx.Name, func(t *testing.T) {
			f, err := rsf.Open(fx.Path)
			if err != nil {
				t.Fatal(err)
			}
			m, err := rsf.NewModel(f)
			if err != nil {
				t.Fatal(err)
			}

			got := m.Tunnels()
			if len(got) != fx.Tunnels {
				t.Errorf("туннелей %d, ожидалось %d: %+v", len(got), fx.Tunnels, got)
			}
			for _, tn := range got {
				// Правило выдаёт туннель только у наполовину пустого узла.
				// Если сюда попал полный, ошибка не в файле, а в правиле.
				if tn.Ins > 0 && tn.Outs > 0 {
					t.Errorf("сектор %d: узел %d полон (вход=%d выход=%d), туннеля быть не должно",
						tn.Sector, tn.Crosspoint, tn.Ins, tn.Outs)
				}
			}
		})
	}
}

// TestTunnelIsDeterministic — порядок ответа не зависит от обхода отображений.
func TestTunnelIsDeterministic(t *testing.T) {
	m := model(t)
	first := m.Tunnels()
	for i := 0; i < 5; i++ {
		again := m.Tunnels()
		if len(again) != len(first) {
			t.Fatalf("прогон %d: туннелей %d, в первый раз %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("прогон %d, туннель %d: %+v, в первый раз %+v", i, j, again[j], first[j])
			}
		}
	}
}

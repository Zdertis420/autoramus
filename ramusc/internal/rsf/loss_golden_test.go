package rsf_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/fixtures"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

var update = flag.Bool("update", false, "переписать эталоны перечня потерь")

// lossDoc — эталонная форма записи. Вид элемента печатается словом: эталон
// читают глазами, и число там ничего не скажет.
type lossDoc struct {
	Attribute string   `json:"attribute"`
	Kind      string   `json:"kind"`
	Columns   []string `json:"columns,omitempty"`
	Count     int      `json:"count"`
}

// TestLossesGolden держит перечень потерь неизменным. Дифф эталона показывает
// ровно то, что изменилось: научились читать атрибут — запись исчезла, завели
// новую модель — появились её потери. Без эталона такие сдвиги остаются
// незамеченными, а именно незаметность и есть дефект, который чинит эта работа.
func TestLossesGolden(t *testing.T) {
	for _, f := range fixtures.All() {
		t.Run(f.Name, func(t *testing.T) {
			m := openModel(t, f.Path)

			docs := make([]lossDoc, 0, len(m.Losses()))
			for _, l := range m.Losses() {
				docs = append(docs, lossDoc{
					Attribute: l.Attribute,
					Kind:      l.Kind.String(),
					Columns:   l.Columns,
					Count:     l.Count,
				})
			}
			got, err := json.MarshalIndent(docs, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')

			golden := filepath.Join("..", "..", "testdata", "golden", goldenName(f.Name)+".loss.json")
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("нет эталона (создайте его флагом -update): %v", err)
			}
			if string(want) != string(got) {
				t.Errorf("перечень потерь разошёлся с эталоном %s\n--- эталон ---\n%s\n--- получено ---\n%s",
					golden, want, got)
			}
		})
	}
}

// goldenName переводит имя модели в имя файла эталона.
func goldenName(model string) string {
	switch model {
	case "ИзготовлениеЮбки":
		return "izgotovlenie-yubki"
	case "ФормированиеТП":
		return "formirovanie-tp"
	case "тест":
		return "test"
	default:
		return model
	}
}

var _ = rsf.KindModel // пакет используется через openModel

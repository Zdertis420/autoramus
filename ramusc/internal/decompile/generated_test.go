package decompile_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// Круг замыкается на нашем собственном выводе.
//
// Принцип V конституции делает декомпилятор исполняемой спецификацией
// генератора: всё, что он вычитывает из файла, генератор обязан уметь записать.
// Проверялось это на файлах, сделанных Ramus. Теперь генератор пишет стрелки
// сам, и проверять надо и его вывод: документ → файл → документ, и на выходе
// ноль ошибок.

// documents — валидные документы-фикстуры плюс примеры поставки.
//
// `dfd-tunnel.yaml` сюда не входит намеренно: декомпилятор не переносит
// `kind: dfd` (CLAUDE.md, «Элементы DFD не переносим»), и на обратном пути
// работы диаграммы потоков данных превращаются в работы IDEF0, с которых
// спрашивается полный ICOM. Это известная граница декомпилятора, а не дефект
// туннелей: сами они через круг проходят, и это проверяет
// `two-sides.yaml` не входит по другой причине: декомпилятор канонизирует
// связи в списки ICOM и секцию links не пишет вовсе (Р14). Явная связь
// «Заготовка из Первой во Вторую стороной control» возвращается сахаром
// `control: [Заготовка]`, и производитель в записи пропадает — хотя на рисунке
// остаётся, потому что выводится по имени потока на той же диаграмме.
// Сравнение связей кортежами этого не различает.

// TestGeneratedKeepsTunnels.
func documents() []string {
	return []string{
		filepath.Join("..", "..", "..", "examples", "skirt.yaml"),
		filepath.Join("..", "..", "..", "examples", "skirt.json"),
		filepath.Join("..", "..", "testdata", "documents", "feedback.yaml"),
		filepath.Join("..", "..", "testdata", "documents", "nested.yaml"),
		filepath.Join("..", "..", "testdata", "documents", "staircase.yaml"),
	}
}

// TestGeneratedOutputPasses — собранный нами файл проходит круг без ошибок.
func TestGeneratedOutputPasses(t *testing.T) {
	for _, path := range documents() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			back := rebuild(t, path)

			var buf bytes.Buffer
			if err := decompile.WriteYAML(&buf, back, decompile.Options{}); err != nil {
				t.Fatal(err)
			}

			diags, err := validate.Source(buf.Bytes())
			if err != nil {
				t.Fatalf("внутренняя ошибка: %v", err)
			}
			if errors, _ := diags.Count(); errors != 0 {
				var text bytes.Buffer
				_ = diags.WriteText(&text, "вывод")
				t.Errorf("ошибок %d:\n%s\n--- документ ---\n%s", errors, text.String(), buf.String())
			}
		})
	}
}

// TestGeneratedKeepsLinks — из собранного файла вычитываются те же связи.
//
// Проверка не про форму документа, а про существо: потеря связи на выходном
// конце конвейера ничем не лучше потери на входном.
func TestGeneratedKeepsLinks(t *testing.T) {
	for _, path := range documents() {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source := compiled(t, path)
			back := rebuild(t, path)

			want := links(source)
			got := links(back)
			for key := range want {
				if !got[key] {
					t.Errorf("связь потеряна: %s", key)
				}
			}
			for key := range got {
				if !want[key] {
					t.Errorf("в файле появилась связь, которой не было в документе: %s", key)
				}
			}
		})
	}
}

// compiled проводит документ через разбор, проверку и раскладку — ровно то,
// что делает компиляция перед генератором.
func compiled(t *testing.T, path string) *ir.Model {
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

// rebuild собирает файл из документа и декомпилирует его обратно.
func rebuild(t *testing.T, path string) *ir.Model {
	t.Helper()

	file, err := generate.File(compiled(t, path))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := rsf.NewModel(file)
	if err != nil {
		t.Fatalf("собранный файл не читается: %v", err)
	}
	back, err := decompile.Model(parsed)
	if err != nil {
		t.Fatalf("собранный файл не декомпилируется: %v", err)
	}
	return back
}

// links сводит связи к множеству строк «поток|источник|приёмник|сторона».
func links(m *ir.Model) map[string]bool {
	out := make(map[string]bool, len(m.Links))
	for _, l := range m.Links {
		out[l.Flow.Name+"|"+l.From.Name+"|"+l.To.Name+"|"+l.SideName()] = true
	}
	return out
}

// `two-sides.yaml` не входит по другой причине: декомпилятор канонизирует
// связи в списки ICOM и секцию links не пишет вовсе (Р14). Явная связь
// «Заготовка из Первой во Вторую стороной control» возвращается сахаром
// `control: [Заготовка]`, и производитель в записи пропадает — хотя на рисунке
// остаётся, потому что выводится по имени потока на той же диаграмме.
// Сравнение связей кортежами этого не различает.

// TestGeneratedKeepsTunnels — объявление tunnel переживает круг.
//
// Признака туннеля в файле нет: он вычисляется по наполнению узла. Значит
// декомпилятор обязан этот счёт повторить и выдать обратно то самое поле,
// которое автор написал, — иначе круг «документ → файл → документ» теряет
// утверждение о модели, а не только рисунок.
func TestGeneratedKeepsTunnels(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "documents", "dfd-tunnel.yaml")
	back := rebuild(t, path)

	got := make(map[string][]string)
	for _, f := range back.Functions {
		for _, ref := range f.Tunnel {
			got[f.Name.Name] = append(got[f.Name.Name], ref.Name)
		}
	}

	want := []string{"Заявка", "Регламент", "Менеджер", "Принятая заявка"}
	have := got["Приём заявок"]
	if len(have) != len(want) {
		t.Fatalf("туннелей у «Приёма заявок» %d (%v), объявлено %d (%v)",
			len(have), have, len(want), want)
	}
	seen := make(map[string]bool, len(have))
	for _, flow := range have {
		seen[flow] = true
	}
	for _, flow := range want {
		if !seen[flow] {
			t.Errorf("поток «%s» потерял пометку tunnel", flow)
		}
	}

	// И ни у кого другого туннелей взяться не должно.
	for name, flows := range got {
		if name != "Приём заявок" {
			t.Errorf("у работы «%s» появились туннели: %v", name, flows)
		}
	}
}

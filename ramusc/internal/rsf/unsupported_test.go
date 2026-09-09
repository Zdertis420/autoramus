package rsf_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// TestUnsupportedFindsUnnamed — работа без имени языком не выражается: имя
// работы и есть её идентификатор (Р4). В testModel.rsf имени нет у четырёх
// живых работ из пяти. Ещё четыре элемента того же квалификатора помечены
// удалёнными (REMOVED_BRANCH_ID=0) и работами уже не являются — считать их
// значило бы отказывать из-за того, чего в модели нет.
func TestUnsupportedFindsUnnamed(t *testing.T) {
	m := openModel(t, fixtureByName(t, "testModel").Path)

	found := m.Unsupported()
	if len(found) == 0 {
		t.Fatal("находок нет, ожидалась хотя бы одна")
	}

	var unnamed *rsf.Unsupported
	for i := range found {
		if found[i].Kind == rsf.UnsupportedUnnamedFunction {
			unnamed = &found[i]
		}
	}
	if unnamed == nil {
		t.Fatalf("находки про безымянные работы нет: %+v", found)
	}
	if unnamed.Count != 4 {
		t.Errorf("безымянных работ %d, ожидалось 4", unnamed.Count)
	}
}

// TestUnsupportedSilentOnGoodModel — модель, которая выражается языком
// полностью, находок не даёт. Иначе отказ сработает там, где всё в порядке,
// и сломает уже работающую декомпиляцию.
func TestUnsupportedSilentOnGoodModel(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ИзготовлениеЮбки").Path)

	if found := m.Unsupported(); len(found) != 0 {
		t.Errorf("находок %d, ожидалось ноль: %+v", len(found), found)
	}
}

// TestUnsupportedDescribesItself — сообщение собирается из находки, а не
// склеивается по случаю: автор должен понять, чего не хватает, не заглядывая
// в исходники.
func TestUnsupportedDescribesItself(t *testing.T) {
	m := openModel(t, fixtureByName(t, "testModel").Path)

	for _, u := range m.Unsupported() {
		s := u.String()
		if s == "" {
			t.Errorf("находка %v описывает себя пустой строкой", u.Kind)
		}
		if u.Kind == rsf.UnsupportedUnnamedFunction && !strings.Contains(s, "4") {
			t.Errorf("в описании %q нет количества", s)
		}
	}
}

// Стрелки без имени. Ramus позволяет нарисовать стрелку и не дать ей имени;
// языком такая стрелка не выражается, потому что идентификатор потока и есть
// его имя (Р4, Р18). Проверки идут на настоящих файлах: узор воспроизводится
// в трёх моделях из четырёх, а «ИзготовлениеЮбки» служит контрольным случаем.

// unnamedArrows отбирает находки про стрелки без имени.
func unnamedArrows(found []rsf.Unsupported) []rsf.Unsupported {
	var out []rsf.Unsupported
	for _, u := range found {
		if u.Kind == rsf.UnsupportedUnnamedArrow {
			out = append(out, u)
		}
	}
	return out
}

// TestUnnamedArrowsCounted — считаются стрелки, а не сегменты. Это главная
// ловушка: в «ФормированииТП» шесть секторов складываются в одну разветвлённую
// стрелку, в «тесте» девять — в три. Без сборки по кросспоинтам получилось бы
// шесть и девять находок, то есть отказ соврал бы автору о числе мест, которые
// надо править.
func TestUnnamedArrowsCounted(t *testing.T) {
	for _, c := range []struct {
		model string
		want  int
	}{
		{"ИзготовлениеЮбки", 0}, // все стрелки названы
		{"ФормированиеТП", 1},   // 6 секторов, одна стрелка с ветвлением
		{"тест", 3},             // 9 секторов, три стрелки подряд
		{"testModel", 4},        // 12 секторов между безымянными работами
	} {
		t.Run(c.model, func(t *testing.T) {
			m := openModel(t, fixtureByName(t, c.model).Path)
			if got := len(unnamedArrows(m.Unsupported())); got != c.want {
				t.Errorf("стрелок без имени %d, ожидалось %d", got, c.want)
			}
		})
	}
}

// TestUnnamedArrowsNamePlaces — находка называет места в терминах модели.
// Прежде о том же говорила сводка потерь строкой вида
// «F_FUNCTION_SECTOR (сектор): OTHER_ELEMENT — 6», по которой нельзя понять,
// что пропало с картинки.
func TestUnnamedArrowsNamePlaces(t *testing.T) {
	m := openModel(t, fixtureByName(t, "тест").Path)

	want := []string{
		"«под работа 1» (выход) → «под работа 2» (вход)",
		"«под работа 2» (выход) → «под работа 3» (вход)",
		"«под работа 3» (выход) → «под работа 4» (вход)",
	}
	got := unnamedArrows(m.Unsupported())
	if len(got) != len(want) {
		t.Fatalf("находок %d, ожидалось %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Where != w {
			t.Errorf("находка %d: %q, ожидалось %q", i, got[i].Where, w)
		}
	}
}

// TestUnnamedArrowBranchListsAllTargets — ветвление не теряет приёмников.
// Стрелка «ФормированияТП» уходит от одной работы к двум; назвать только
// первую значило бы отправить автора править половину места.
func TestUnnamedArrowBranchListsAllTargets(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ФормированиеТП").Path)

	got := unnamedArrows(m.Unsupported())
	if len(got) != 1 {
		t.Fatalf("находок %d, ожидалась одна: %+v", len(got), got)
	}
	const want = "«Оформление, нормконтроль и утверждение пояснительной записки» (выход) → " +
		"«Разработка описания основных технических решений» (вход), " +
		"«Разработка по подготовке объекта автоматизации к вводу системы в действие» (вход)"
	if got[0].Where != want {
		t.Errorf("находка: %q,\nожидалось: %q", got[0].Where, want)
	}
}

// TestUnnamedArrowStable — порядок находок задан, а не взят из обхода map.
// Вывод читают глазами и сравнивают диффом, поэтому два запуска обязаны дать
// одно и то же.
func TestUnnamedArrowStable(t *testing.T) {
	m := openModel(t, fixtureByName(t, "тест").Path)

	first := unnamedArrows(m.Unsupported())
	for i := 0; i < 5; i++ {
		next := unnamedArrows(m.Unsupported())
		if len(next) != len(first) {
			t.Fatalf("число находок поплыло: %d против %d", len(next), len(first))
		}
		for j := range first {
			if next[j].Where != first[j].Where {
				t.Fatalf("порядок находок поплыл на %d: %q против %q", j, next[j].Where, first[j].Where)
			}
		}
	}
}

// Краевые случаи собираются вручную поверх настоящего файла: во всех четырёх
// моделях безымянные стрелки прицеплены к работам обоими концами, а ветки
// «прицеплен один конец» и «не прицеплен ни один» в отказе всё равно
// возможны — неприсоединённые концы Ramus оставляет, и у именованных стрелок
// они встречаются. Строки добавляются в те же таблицы того же файла, поэтому
// проверяется настоящий код чтения, а не его подобие.

// tableID ищет в таблице file идентификатор по имени: у квалификаторов и
// атрибутов номера свои в каждом файле, зашивать их числом нельзя.
func tableID(t *testing.T, m *rsf.Model, table, nameField, name, idField string) string {
	t.Helper()
	tb, err := m.File.Table(table)
	if err != nil {
		t.Fatalf("%s: %v", table, err)
	}
	for _, row := range tb.Rows {
		if tb.Str(row, nameField) == name {
			return tb.Str(row, idField)
		}
	}
	t.Fatalf("в таблице %s нет %q", table, name)
	return ""
}

// addSector дописывает сектор без потока: элемент квалификатора F_SECTORS,
// у которого строки F_SECTOR_STREAM нет вовсе. Именно этим безымянная стрелка
// и отличается — ссылки на поток у неё не существует.
func addSector(t *testing.T, m *rsf.Model, id string, ends map[string][2]string) {
	t.Helper()

	sectors := tableID(t, m, "qualifiers", "QUALIFIER_NAME", "F_SECTORS", "QUALIFIER_ID")
	elements, err := m.File.Table("elements")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := elements.Add(map[string]string{
		"ELEMENT_ID":        id,
		"ELEMENT_NAME":      "",
		"QUALIFIER_ID":      sectors,
		"CREATED_BRANCH_ID": "0",
		"REMOVED_BRANCH_ID": rsf.AliveBranch,
	}); err != nil {
		t.Fatal(err)
	}

	borders, err := m.File.Table("attribute_sector_borders")
	if err != nil {
		t.Fatal(err)
	}
	// ends: имя атрибута конца -> {работа, сторона}. Работа «-1» означает
	// конец, ни к чему не прицепленный.
	for attribute, end := range ends {
		if _, err := borders.Add(map[string]string{
			"ATTRIBUTE_ID":    tableID(t, m, "attributes", "ATTRIBUTE_NAME", attribute, "ATTRIBUTE_ID"),
			"ELEMENT_ID":      id,
			"FUNCTION":        end[0],
			"FUNCTION_TYPE":   end[1],
			"BORDER_TYPE":     "-1",
			"CROSSPOINT":      "-1",
			"TUNNEL_SOFT":     "0",
			"VALUE_BRANCH_ID": "0",
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// TestUnnamedArrowWithLooseEnds — у стрелки, ни один конец которой не
// прицеплен к работе, назвать в сообщении нечего, и оно обязано остаться
// осмысленным: молчаливого пропуска такой стрелки быть не должно.
//
// Основой берётся «ИзготовлениеЮбки»: своих безымянных стрелок в ней нет,
// поэтому найденная — заведомо та, что добавлена здесь.
func TestUnnamedArrowWithLooseEnds(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ИзготовлениеЮбки").Path)
	addSector(t, m, "950", nil)

	got := unnamedArrows(m.Unsupported())
	if len(got) != 1 {
		t.Fatalf("находок %d, ожидалась одна: %+v", len(got), got)
	}
	if got[0].Where != "оба конца не присоединены" {
		t.Errorf("описание %q, ожидалось «оба конца не присоединены»", got[0].Where)
	}
}

// TestUnnamedArrowWithOneEnd — прицеплен один конец: называется он, а второй
// не выдумывается. Стрелка с одним концом в Ramus рисуется и языком не
// выражается ровно так же, как стрелка с двумя.
func TestUnnamedArrowWithOneEnd(t *testing.T) {
	m := openModel(t, fixtureByName(t, "ИзготовлениеЮбки").Path)

	functions := m.Functions()
	if len(functions) == 0 {
		t.Fatal("в модели нет работ, случай не собрать")
	}
	target := functions[0]

	addSector(t, m, "951", map[string][2]string{
		"F_SECTOR_BORDER_END": {fmt.Sprint(target.ID), fmt.Sprint(int(rsf.SideLeft))},
	})

	got := unnamedArrows(m.Unsupported())
	if len(got) != 1 {
		t.Fatalf("находок %d, ожидалась одна: %+v", len(got), got)
	}
	want := "→ «" + target.Name + "» (вход)"
	if got[0].Where != want {
		t.Errorf("описание %q, ожидалось %q", got[0].Where, want)
	}
}

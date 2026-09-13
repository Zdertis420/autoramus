package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// dump печатает содержимое .rsf: работы, потоки, сегменты стрелок и сводку
// ICOM. Первые три раздела повторяют вывод algodemo/rsf.py слово в слово —
// пока декомпилятора нет, эталоном служит именно он, и расхождение вывода
// сразу видно диффом.
func dump(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "ramusc: dump ожидает ровно один файл, получено %d\n\n", len(args))
		usage(stderr)
		return exitInternal
	}
	filename := args[0]

	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n", err)
		return exitInternal
	}

	file, err := rsf.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", filename, err)
		return exitInvalid
	}
	model, err := rsf.NewModel(file)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", filename, err)
		return exitInvalid
	}

	writeDump(stdout, filename, model)
	return exitOK
}

func writeDump(w io.Writer, filename string, m *rsf.Model) {
	functions := m.Functions()
	names := make(map[int64]string, len(functions))
	for _, f := range functions {
		names[f.ID] = f.Name
	}
	streams := m.Streams()
	streamNames := make(map[int64]string, len(streams))
	for _, s := range streams {
		streamNames[s.ID] = s.Name
	}

	fmt.Fprintf(w, "файл: %s\n", filename)
	fmt.Fprintf(w, "модель: элемент %d, квалификатор работ %d (%s)\n",
		m.Element, m.FunctionQualifier, m.QualifierName())

	fmt.Fprintf(w, "\nРАБОТЫ\n")
	for _, f := range functions {
		fmt.Fprintf(w, "  %s  parent=%s  type=%s %s %s\n",
			padLeft(fmt.Sprint(f.ID), 4),
			padLeft(fmt.Sprint(f.Parent), 3),
			pad(fmt.Sprint(f.Type), 4),
			pad(bounds(f.Bounds), 34),
			f.Name)
	}

	fmt.Fprintf(w, "\nПОТОКИ (стрелки-сущности)\n")
	for _, s := range streams {
		fmt.Fprintf(w, "  %s  %s\n", padLeft(fmt.Sprint(s.ID), 4), s.Name)
	}

	fmt.Fprintf(w, "\nСЕКТОРЫ (сегменты стрелок)\n")
	for _, s := range m.Sectors() {
		diagram, ok := names[s.Diagram]
		if !ok {
			// Сегменты контекстной диаграммы принадлежат элементу модели,
			// а он работой не является.
			diagram = fmt.Sprint(s.Diagram)
		}
		fmt.Fprintf(w, "  %s  диаграмма=%s поток=%s %s -> %s\n",
			padLeft(fmt.Sprint(s.ID), 4),
			pad(quote(diagram), 28),
			pad(quote(streamNames[s.Stream]), 32),
			end(s.Start, names),
			end(s.End, names))
	}

	fmt.Fprintf(w, "\nICOM ПО РАБОТАМ\n")
	icom := m.ICOM()
	for _, f := range functions {
		fmt.Fprintf(w, "  %s\n", f.Name)
		sides := icom[f.ID]
		for _, side := range []rsf.Side{rsf.SideLeft, rsf.SideTop, rsf.SideBottom, rsf.SideRight} {
			var flows []string
			if sides != nil {
				flows = sides.Side(side)
			}
			mark, list := "  ", strings.Join(flows, ", ")
			if len(flows) == 0 {
				// Пустая сторона у работы IDEF0 — повод посмотреть внимательно.
				mark, list = "!!", "— ПУСТО —"
			}
			fmt.Fprintf(w, "    %s %s: %s\n", mark, pad(side.ICOM(), 9), list)
		}
	}

	// Раздел идёт последним и намеренно: первые три повторяют вывод
	// algodemo/rsf.py слово в слово, и сдвигать их нельзя — на этом держится
	// сверка диффом. Туннели же нужны не для сверки, а автору: они отвечают на
	// вопрос «почему у меня скобки», не открывая Ramus.
	fmt.Fprintf(w, "\nТУННЕЛИ (концы в скобках)\n")
	tunnels := m.Tunnels()
	if len(tunnels) == 0 {
		fmt.Fprintf(w, "  туннелей нет\n")
	}
	for _, t := range tunnels {
		where := "конец "
		if t.Start {
			where = "начало"
		}
		diagram, ok := names[t.Diagram]
		if !ok {
			diagram = fmt.Sprint(t.Diagram)
		}
		fmt.Fprintf(w, "  %s  %s  узел#%s вход=%d выход=%d  поток=%s диаграмма=%s\n",
			padLeft(fmt.Sprint(t.Sector), 4),
			where,
			pad(fmt.Sprint(t.Crosspoint), 5),
			t.Ins, t.Outs,
			pad(quote(streamNames[t.Stream]), 32),
			quote(diagram))
	}
}

// end описывает конец сегмента так же, как это делает эталонный дамп.
func end(b *rsf.Border, names map[int64]string) string {
	switch {
	case b == nil:
		return "?"
	case b.OnFunction():
		name, ok := names[b.Function]
		if !ok {
			name = fmt.Sprint(b.Function)
		}
		return name + ":" + b.FunctionType.String()
	case b.BorderType >= 0:
		return fmt.Sprintf("край диаграммы(%d)", b.BorderType)
	default:
		return fmt.Sprintf("узел#%d", b.Crosspoint)
	}
}

// bounds печатает прямоугольник кортежем, как в эталоне: (288.0, 147.0, …).
func bounds(r *rsf.Rect) string {
	if r == nil {
		return "None"
	}
	return "(" + strings.Join([]string{
		rsf.FormatFloat(r.X), rsf.FormatFloat(r.Y),
		rsf.FormatFloat(r.Width), rsf.FormatFloat(r.Height),
	}, ", ") + ")"
}

// quote берёт имя в одинарные кавычки — так печатает repr в эталоне.
func quote(s string) string { return "'" + s + "'" }

// pad и padLeft выравнивают по рунам, а не по байтам: имена кириллические,
// и побайтовое выравнивание развалило бы колонки.
func pad(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

func padLeft(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return strings.Repeat(" ", width-n) + s
	}
	return s
}

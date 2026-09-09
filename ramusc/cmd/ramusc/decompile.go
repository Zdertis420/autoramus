package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/decompile"
	"github.com/Zdertis420/autoramus/ramusc/internal/rsf"
)

// decompileCommand печатает готовую модель Ramus на входном языке. По Р14
// вывод канонический: ровно в том стиле, которого ждут от генератора, — чтобы
// любую существующую модель можно было взять few-shot примером.
func decompileCommand(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n\n", err)
		usage(stderr)
		return exitInternal
	}
	if len(opts.files) != 1 {
		fmt.Fprintf(stderr, "ramusc: decompile ожидает ровно один файл, получено %d\n\n", len(opts.files))
		usage(stderr)
		return exitInternal
	}
	filename := opts.files[0]

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
	source, err := rsf.NewModel(file)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", filename, err)
		return exitInvalid
	}
	// Отказ решается до печати: неполный результат не выдаётся ни при каких
	// условиях, иначе автор примет его за полный и построит на нём работу.
	if found := source.Unsupported(); len(found) > 0 {
		fmt.Fprintf(stderr, "%s: файл не выражается входным языком полностью\n", filename)
		for _, u := range found {
			fmt.Fprintf(stderr, "  %s\n", u)
		}
		fmt.Fprintln(stderr, "ничего не записано")
		return exitInternal
	}

	model, lost, err := decompile.ModelWithReport(source)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", filename, err)
		return exitInvalid
	}

	// Потеря — не отказ. Отказ выше остаётся за содержимым, которое языком не
	// описывается по существу; здесь модель перенесена и работать с ней можно,
	// просто часть оформления не доехала даже люком. Блокировать нечего, но и
	// молчать нельзя: с молчаливых потерь всё и начиналось.
	if len(lost) > 0 {
		fmt.Fprintf(stderr, "%s: перенесено не всё\n", filename)
		for _, l := range lost {
			fmt.Fprintf(stderr, "  %s\n", l)
		}
	}

	var out bytes.Buffer
	if opts.jsonOut {
		err = decompile.WriteJSON(&out, model)
	} else {
		err = decompile.WriteYAML(&out, model, decompile.Options{Header: header(filename, source, lost)})
	}
	if err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n", err)
		return exitInternal
	}

	if opts.output != "" && opts.output != "-" {
		if err := os.WriteFile(opts.output, out.Bytes(), 0o644); err != nil {
			fmt.Fprintf(stderr, "ramusc: %v\n", err)
			return exitInternal
		}
		return exitOK
	}
	if _, err := stdout.Write(out.Bytes()); err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n", err)
		return exitInternal
	}
	return exitOK
}

// header говорит о том, что вызовет замечания валидатора: ожидаемые
// unused_flow не должны приниматься за поломку и «чиниться» фильтрацией flows.
func header(filename string, source *rsf.Model, lost []rsf.Loss) []string {
	lines := []string{fmt.Sprintf("Декомпилировано из %s.", filename)}

	// Перечень потерь повторяется в шапке: вывод часто уходит в файл по -o, и
	// поток ошибок тогда пролетает мимо глаз, а узнать о потере автор должен
	// именно при работе с результатом.
	if len(lost) > 0 {
		lines = append(lines, "Перенесено не всё, ниже — что не доехало:")
		for _, l := range lost {
			lines = append(lines, "  "+l.String())
		}
	}

	if orphans := decompile.OrphanFlows(source); len(orphans) > 0 {
		lines = append(lines,
			fmt.Sprintf("Потоки без единой стрелки (%d): %s.", len(orphans), strings.Join(orphans, ", ")),
			"Так и лежат в файле — обычно остатки переименования. Они объявлены",
			"в flows, поэтому validate даст на них предупреждение unused_flow.")
	}
	return lines
}

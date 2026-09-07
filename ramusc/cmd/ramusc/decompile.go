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
	model, err := decompile.Model(source)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", filename, err)
		return exitInvalid
	}

	var out bytes.Buffer
	if opts.jsonOut {
		err = decompile.WriteJSON(&out, model)
	} else {
		err = decompile.WriteYAML(&out, model, decompile.Options{Header: header(filename, source)})
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
func header(filename string, source *rsf.Model) []string {
	lines := []string{fmt.Sprintf("Декомпилировано из %s.", filename)}

	if orphans := decompile.OrphanFlows(source); len(orphans) > 0 {
		lines = append(lines,
			fmt.Sprintf("Потоки без единой стрелки (%d): %s.", len(orphans), strings.Join(orphans, ", ")),
			"Так и лежат в файле — обычно остатки переименования. Они объявлены",
			"в flows, поэтому validate даст на них предупреждение unused_flow.")
	}
	return lines
}

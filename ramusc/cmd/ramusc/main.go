// Команда ramusc — компилятор моделей Ramus: текстовое описание модели
// превращается в файл .rsf, минуя GUI Ramus.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/Zdertis420/autoramus/ramusc/internal/diag"
	"github.com/Zdertis420/autoramus/ramusc/internal/generate"
	"github.com/Zdertis420/autoramus/ramusc/internal/ir"
	"github.com/Zdertis420/autoramus/ramusc/internal/layout"
	"github.com/Zdertis420/autoramus/ramusc/internal/validate"
)

// Коды возврата — контракт с GUI, см. CLAUDE.md.
const (
	exitOK       = 0 // успех
	exitInvalid  = 1 // вход невалиден: список ошибок показывается пользователю
	exitInternal = 2 // внутренняя ошибка или неверное употребление команды
)

const bugTracker = "https://github.com/Zdertis420/autoramus/issues"

// version подставляется при сборке: -ldflags "-X main.version=…".
var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "внутренняя ошибка ramusc: %v\n%s\n", r, debug.Stack())
			fmt.Fprintf(stderr, "сообщите об этом: %s\n", bugTracker)
			code = exitInternal
		}
	}()

	if len(args) == 0 {
		usage(stderr)
		return exitInternal
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage(stdout)
		return exitOK
	case "version", "--version":
		fmt.Fprintf(stdout, "ramusc %s\n", version)
		return exitOK
	case "validate":
		return command(args[1:], stdout, stderr, false)
	case "dump":
		return dump(args[1:], stdout, stderr)
	case "decompile":
		return decompileCommand(args[1:], stdout, stderr)
	default:
		return command(args, stdout, stderr, true)
	}
}

// command исполняет и проверку, и компиляцию: разбор с валидацией у них общий,
// двух наборов сообщений об одном и том же быть не должно.
func command(args []string, stdout, stderr io.Writer, compile bool) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n\n", err)
		usage(stderr)
		return exitInternal
	}
	if len(opts.files) != 1 {
		fmt.Fprintf(stderr, "ramusc: ожидается ровно один входной файл, получено %d\n\n", len(opts.files))
		usage(stderr)
		return exitInternal
	}
	filename := opts.files[0]

	src, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n", err)
		return exitInternal
	}

	model, diags, internal := validate.Build(src)
	if internal != nil {
		fmt.Fprintf(stderr, "внутренняя ошибка ramusc: %v\n", internal)
		fmt.Fprintf(stderr, "сообщите об этом: %s\n", bugTracker)
		return exitInternal
	}

	// Документ печатается в stdout только при -o -, и мешать его с диагностикой
	// нельзя: .rsf бинарен. В этом случае всё сообщаемое уходит в поток ошибок.
	reportTo := stdout
	if compile && opts.output == "-" {
		reportTo = stderr
	}
	if err := report(reportTo, stderr, filename, diags, opts.jsonOut); err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n", err)
		return exitInternal
	}
	if diags.HasErrors() {
		return exitInvalid
	}
	if !compile {
		return exitOK
	}
	return build(model, filename, opts, stdout, stderr)
}

// build собирает .rsf и записывает его.
//
// Отказ генератора — не ошибка автора: документ проверку прошёл, а не хватает
// того, чего язык не требует (координат), либо сломана сама программа. Поэтому
// код 2, а не 1, и сообщение отдельным каналом, а не объектом диагностики.
func build(model *ir.Model, filename string, opts options, stdout, stderr io.Writer) int {
	// Раскладка стоит между проверкой и генератором — ровно там, где её место
	// в конвейере. Оркеструет команда, а не генератор: тот знает только про IR
	// и о существовании раскладки не подозревает (Р16).
	layout.Apply(model)

	file, err := generate.File(model)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", filename, err)
		fmt.Fprintln(stderr, "ничего не записано")
		return exitInternal
	}
	data, err := file.Bytes()
	if err != nil {
		fmt.Fprintf(stderr, "внутренняя ошибка ramusc: %v\n", err)
		fmt.Fprintf(stderr, "сообщите об этом: %s\n", bugTracker)
		return exitInternal
	}

	// Файл создаётся только теперь, когда весь ZIP собран в память:
	// наполовину записанного .rsf не остаётся ни при какой ошибке.
	if opts.output == "-" {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "ramusc: %v\n", err)
			return exitInternal
		}
		return exitOK
	}
	out := opts.output
	if out == "" {
		out = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".rsf"
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "ramusc: %v\n", err)
		return exitInternal
	}
	return exitOK
}

func report(stdout, stderr io.Writer, filename string, diags diag.List, jsonOut bool) error {
	if jsonOut {
		return diags.WriteJSON(stdout)
	}
	if err := diags.WriteText(stderr, filename); err != nil {
		return err
	}
	errors, warnings := diags.Count()
	if errors > 0 {
		fmt.Fprintf(stderr, "%s: ошибок — %d%s\n", filename, errors, warned(warnings))
		return nil
	}
	// Предупреждение не мешает компиляции, но и молчать о нём нельзя:
	// иначе его никто никогда не прочитает.
	_, err := fmt.Fprintf(stdout, "%s: ошибок нет%s\n", filename, warned(warnings))
	return err
}

func warned(warnings int) string {
	if warnings == 0 {
		return ""
	}
	return fmt.Sprintf(", предупреждений — %d", warnings)
}

type options struct {
	jsonOut bool
	output  string
	files   []string
}

// parseArgs разбирает флаги вручную: в документированном виде вызова флаг
// стоит после имени файла (`ramusc validate model.json --json`), а пакет flag
// такое не принимает.
func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json" || arg == "-json":
			opts.jsonOut = true
		case arg == "-o" || arg == "--output":
			if i+1 >= len(args) {
				return opts, errors.New("у флага -o нет значения")
			}
			i++
			opts.output = args[i]
		case strings.HasPrefix(arg, "-o="):
			opts.output = strings.TrimPrefix(arg, "-o=")
		case strings.HasPrefix(arg, "--output="):
			opts.output = strings.TrimPrefix(arg, "--output=")
		case len(arg) > 1 && strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("неизвестный флаг %s", arg)
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts, nil
}

func usage(w io.Writer) {
	fmt.Fprint(w, `ramusc — компилятор моделей Ramus

Использование:
  ramusc <файл> [-o вывод.rsf]   компиляция; по умолчанию model.json -> model.rsf
  ramusc validate <файл>         проверка, человекочитаемый вывод
  ramusc validate <файл> --json  проверка, машинный вывод для GUI
  ramusc dump <файл.rsf>         показать модель из готового файла Ramus
  ramusc decompile <файл.rsf>    напечатать её на входном языке (YAML; --json)
  ramusc version                 версия
  ramusc help                    эта справка

Флаги:
  -o, --output <файл>  куда писать результат; «-» — в стандартный вывод
      --json           печатать диагностику массивом JSON

Коды возврата:
  0  успех
  1  вход невалиден
  2  внутренняя ошибка или неверное употребление команды
`)
}

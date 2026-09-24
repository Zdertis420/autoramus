// Command build — сборочные цели проекта. Makefile зовёт её через `go run`
// и сам никакой логики не содержит.
//
// Причина такого устройства: GNU Make на Windows берёт cmd.exe, когда не
// находит sh.exe в PATH, а рецепты на POSIX-диалекте там не разбираются —
// одинарные кавычки не снимаются, /dev/null это несуществующий путь. Здесь
// внешние команды запускаются через os/exec, аргументы передаются массивом,
// и шелла между make и `go build` нет вовсе.
//
// Пакет обязан зависеть только от стандартной библиотеки. Иначе ошибка
// компиляции в любом пакете ramusc сломает сам сборщик, и вместо внятной
// диагностики разработчик получит отказ запустить сборку.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Флаги сборки. Вывод детерминированный: -trimpath убирает пути сборочной
// машины из бинаря, версия подставляется явно.
const (
	binName    = "ramusc"
	pkgPath    = "./cmd/ramusc"
	distDir    = "dist"
	trimPath   = "-trimpath"
	ldflagsFmt = "-s -w -X main.version=%s"
)

// platforms — цели кросс-компиляции. GUI фактически Linux-only, остальным
// пользователям достаётся CLI.
var platforms = []string{
	"linux/amd64",
	"linux/arm64",
	"windows/amd64",
	"darwin/amd64",
	"darwin/arm64",
}

func main() {
	flag.CommandLine.SetOutput(os.Stderr)
	flag.Usage = usage

	version := flag.String("version", "", "версия для main.version; пустая — взять у git describe")

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	verb := os.Args[1]
	if verb == "help" || verb == "-h" || verb == "--help" {
		flag.CommandLine.SetOutput(os.Stdout)
		usage()
		return
	}
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

	var err error
	switch verb {
	case "build":
		err = build(*version)
	case "cross":
		err = cross(*version)
	case "check":
		err = check()
	case "clean":
		err = clean()
	default:
		fmt.Fprintf(os.Stderr, "неизвестная цель %q\n\n", verb)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, `Сборочные цели ramusc.

Использование:
  go run ./internal/build <цель> [флаги]

Цели:
  build   собрать %s для текущей платформы
  cross   собрать все платформы в %s/
  check   форматирование, go vet, тесты
  clean   удалить сборочные артефакты
  help    эта справка

Флаги:
`, binName, distDir)
	flag.PrintDefaults()
}

// fail печатает ошибку и выходит. Код дочернего процесса сохраняется: ради
// него Makefile и существует — красная цель обязана оставаться красной.
func fail(err error) {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// Дочерний процесс уже всё сказал в свой stderr.
		os.Exit(exit.ExitCode())
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// build собирает бинарь для текущей платформы.
func build(version string) error {
	out := binName + exeSuffix(runtime.GOOS)
	return goBuild(resolveVersion(version), out, nil)
}

// cross собирает все платформы из перечня. Первая ошибка прекращает цикл:
// половина dist/ хуже, чем его отсутствие.
func cross(version string) error {
	v := resolveVersion(version)
	for _, platform := range platforms {
		goos, goarch, ok := strings.Cut(platform, "/")
		if !ok {
			return fmt.Errorf("платформа %q не вида os/arch", platform)
		}
		fmt.Printf("  %s/%s\n", goos, goarch)
		out := filepath.Join(distDir, fmt.Sprintf("%s-%s-%s%s", binName, goos, goarch, exeSuffix(goos)))
		env := []string{"GOOS=" + goos, "GOARCH=" + goarch, "CGO_ENABLED=0"}
		if err := goBuild(v, out, env); err != nil {
			return err
		}
	}
	return nil
}

// goBuild — единственное место, где собирается командная строка go build.
// Флаги здесь ровно те же, что были в Makefile: на них опираются тесты,
// сравнивающие байты.
func goBuild(version, out string, env []string) error {
	ldflags := fmt.Sprintf(ldflagsFmt, version)
	return run(env, "go", "build", trimPath, "-ldflags", ldflags, "-o", out, pkgPath)
}

// check — проверка перед коммитом. Неотформатированный код считается ошибкой
// сборки, а не замечанием на ревью, поэтому до vet и тестов дело не доходит.
func check() error {
	listed, err := output("gofmt", "-l", ".")
	if err != nil {
		return err
	}
	if files := strings.Fields(listed); len(files) > 0 {
		fmt.Fprintln(os.Stderr, "не отформатировано:")
		for _, f := range files {
			fmt.Fprintln(os.Stderr, "  "+f)
		}
		return errors.New("выполните gofmt -w .")
	}
	if err := run(nil, "go", "vet", "./..."); err != nil {
		return err
	}
	return run(nil, "go", "test", "./...")
}

// clean удаляет артефакты сборки. Оба имени бинаря, а не только имя для
// текущей платформы: файл без расширения остаётся на Windows после сборки
// из Git Bash, и чистка, которая его не видит, — неприятный сюрприз.
func clean() error {
	for _, path := range []string{binName, binName + ".exe", distDir} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// exeSuffix — расширение исполняемого файла для целевой платформы.
func exeSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

// resolveVersion: явно заданная версия важнее вычисленной. Пустой ответ git
// или его отсутствие — не ошибка сборки, просто версии нет.
func resolveVersion(explicit string) string {
	if explicit != "" {
		return explicit
	}
	described, err := output("git", "describe", "--tags", "--always", "--dirty")
	if err != nil || described == "" {
		return "dev"
	}
	return described
}

// run запускает команду, показав её перед запуском. Показывает ради того,
// что терялось при переезде логики из Makefile: точную строку сборки, которую
// можно повторить руками.
func run(env []string, name string, args ...string) error {
	fmt.Println(display(env, name, args))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd.Run()
}

// output запускает команду ради её стандартного вывода. Чужой stderr молчит:
// отсутствие тегов в репозитории — не повод пугать сообщением от git.
func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// display собирает команду в вид, пригодный для копирования в терминал.
// Переменные окружения печатаются префиксом: для cross они часть ответа на
// вопрос «чем именно собран этот файл».
func display(env []string, name string, args []string) string {
	parts := make([]string, 0, len(env)+len(args)+1)
	parts = append(parts, env...)
	parts = append(parts, name)
	for _, a := range args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

// quote берёт аргумент в кавычки, когда без них строку нельзя вставить в
// терминал как есть.
func quote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " 	\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

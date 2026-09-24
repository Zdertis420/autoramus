# Сборка ramusc. Логика целей живёт в ./internal/build — программе на Go,
# которую цели зовут через `go run`. Причина: GNU Make на Windows берёт
# cmd.exe, когда не находит sh.exe в PATH, и рецепты на POSIX-диалекте там
# не разбираются — одинарные кавычки не снимаются, /dev/null это путь,
# которого нет. Программа на Go запускает go build через os/exec, аргументы
# передаёт массивом, и шелла между make и сборкой не остаётся вовсе.
#
# Makefile здесь — оглавление операций, а не язык программирования. Сам он
# необязателен: `go run ./internal/build <цель>` делает то же самое, и на
# машине без make собрать проект можно так.

# Пустая по умолчанию: версию добудет ./internal/build через git describe.
# Явное значение важнее вычисленного — make build VERSION=1.2.3.
VERSION ?=

TOOL := go run ./internal/build

.PHONY: all build test vet fmt check cross clean golden

all: build

build:
	$(TOOL) build -version "$(VERSION)"

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Проверка перед коммитом: форматирование, вет, тесты.
check:
	$(TOOL) check

# Переписать эталоны после осознанного изменения сообщений или вывода dump.
golden:
	go test ./internal/validate -update
	go test ./internal/rsf -update
	go test ./cmd/ramusc -update

cross:
	$(TOOL) cross -version "$(VERSION)"

clean:
	$(TOOL) clean

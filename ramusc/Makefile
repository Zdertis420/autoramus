# Сборка ramusc. Вывод детерминированный: -trimpath убирает пути сборочной
# машины из бинаря, версия подставляется явно.

BIN     := ramusc
PKG     := ./cmd/ramusc
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath

# Платформы для кросс-компиляции. GUI фактически Linux-only, остальным
# пользователям достаётся CLI.
PLATFORMS := linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64

.PHONY: all build test vet fmt check cross clean golden

all: build

build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Проверка перед коммитом: форматирование, вет, тесты.
check:
	@test -z "$$(gofmt -l .)" || { echo "не отформатировано:"; gofmt -l .; exit 1; }
	$(MAKE) vet
	$(MAKE) test

# Переписать эталоны после осознанного изменения сообщений или вывода dump.
golden:
	go test ./internal/validate -update
	go test ./internal/rsf -update
	go test ./cmd/ramusc -update

cross:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "  $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build $(GOFLAGS) -ldflags '$(LDFLAGS)' \
			-o dist/$(BIN)-$$os-$$arch$$ext $(PKG) || exit 1; \
	done

clean:
	rm -rf $(BIN) dist

# Phase 1. Как убедиться, что работает

Все команды — из каталога `ramusc/`.

## До начала

```bash
git add examples/тест.rsf     # сейчас файл не под контролем версий
make check                    # исходное состояние обязано быть зелёным
```

## Слепок «как было»

Снять до правок, чтобы потом сравнить диффом:

```bash
go run ./cmd/ramusc decompile ../examples/ИзготовлениеЮбки.rsf > /tmp/before-yubka.yaml
go run ./cmd/ramusc decompile ../examples/тест.rsf            > /tmp/before-test.yaml
```

Сегодня второй файл печатается с кодом 0 и сводкой потерь на девять секторов —
это и есть дефект, который чинится.

## Сценарий 1 — отказ на простейшей модели (FR-003, FR-004)

```bash
go run ./cmd/ramusc decompile ../examples/тест.rsf; echo "код: $?"
```

Ожидается: stdout пуст, код 2, на stderr три строки о безымянных стрелках —
по одной на стрелку, с именами «под работа 1…4». Точный текст —
[contracts/cli.md](./contracts/cli.md).

Проверяется заодно FR-002: строк должно быть **три**, а не девять. Девять строк
означают, что сборка по кросспоинтам не сделана и считаются сегменты.

## Сценарий 2 — ветвление считается одной стрелкой (FR-002)

```bash
go run ./cmd/ramusc decompile ../examples/ФормированиеТП.rsf; echo "код: $?"
```

Ожидается: код 2, **одна** строка о безымянной стрелке, в ней один источник и
два приёмника через запятую.

## Сценарий 3 — файл не пишется при отказе (контракт)

```bash
rm -f /tmp/out.yaml
go run ./cmd/ramusc decompile ../examples/тест.rsf -o /tmp/out.yaml; echo "код: $?"
test ! -e /tmp/out.yaml && echo "файл не создан — верно"
```

## Сценарий 4 — контрольный случай не сдвинулся (FR-008, SC-005)

```bash
go run ./cmd/ramusc decompile ../examples/ИзготовлениеЮбки.rsf > /tmp/after-yubka.yaml
diff /tmp/before-yubka.yaml /tmp/after-yubka.yaml && echo "не изменилось — верно"
```

Диффа быть не должно ни на байт.

## Сценарий 5 — все причины сразу (FR-005)

```bash
go run ./cmd/ramusc decompile testdata/testModel.rsf; echo "код: $?"
```

Ожидается прежняя строка `работ без имени: 4`. Безымянных стрелок в этом файле
нет, новых строк не появляется — проверка на ложные срабатывания.

## Сценарий 6 — перечень моделей управляет проверками (FR-009, FR-010, FR-011)

```bash
go test ./internal/decompile -run 'TestOutputPasses|TestRefusedModelsAreRefused|TestNothingIsLostSilently' -v
```

Ожидается: `TestOutputPasses` идёт по одной модели, `TestRefusedModelsAreRefused`
— по трём (`testModel`, `ФормированиеТП`, `тест`) и на каждой находит непустой
перечень находок.

```bash
go test ./internal/rsf -run 'TestLossesGolden' -v
```

Ожидается: проверка идёт по всем четырём моделям, включая отвергаемые, — отказ
касается декомпиляции, а не чтения файла.

## Сценарий 7 — эталоны

```bash
make golden      # появится testdata/golden/test.loss.json
git diff --stat testdata/golden/
```

В дифф должен попасть только новый эталон для `тест.rsf`. Изменение существующих
эталонов означает, что задет контрольный случай, и требует разбирательства, а не
`make golden` вслепую.

## Наконец

```bash
make check
```

Форматирование, `go vet`, все тесты. Коммитить код и переписанные эталоны одним
изменением — правило конституции.

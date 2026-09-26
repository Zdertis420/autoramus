# Quickstart: как проверить фичу

Команды — от корня репозитория.

## 0. Сборка

```bash
cd ramusc && make build && cd ..     # даст ./ramusc/ramusc
```

## 1. До: мера

```bash
for f in examples/*.yaml ramusc/testdata/documents/*.yaml; do
  ./ramusc/ramusc "$f" -o "/tmp/$(basename "$f" .yaml).rsf" >/dev/null
done
python3 specs/014-arrows-avoid-blocks/evidence/through.py /tmp/*.rsf
```

Ожидается сегодня: «чахохбили» — 2 отрезка сквозь «Подготовку томатной
массы» («Курица с луком»); `skirt-full` — 5; остальные — 0.

## 2. После: ноль

Те же команды. Ожидается ноль отрезков сквозь блоки на всех документах
(SC-001, SC-002, SC-006).

## 3. Документы без задетых блоков не изменились

Собрать набор до и после и сравнить `cmp`: меняются только `chakhokhbili` и
`skirt-full` (SC-003).

## 4. Предупреждение

```bash
./ramusc/ramusc validate ramusc/testdata/models/arrow-through-block.yaml --json
echo "код возврата: $?"
```

Ожидается один объект `"severity": "warning", "code": "arrow_through_block"`
с `line`/`column` точки и код возврата 0. Компиляция того же документа
собирает файл и печатает то же предупреждение.

## 5. Тесты

```bash
cd ramusc && make check
```

Новые проверки — `contracts/no-arrow-through-block.md` §5.

## 6. Пересборка примеров и взгляд в Ramus

```bash
./ramusc/ramusc examples/chakhokhbili.yaml -o examples/chakhokhbili.rsf
```

Открыть в Ramus A0: «Курица с луком» обходит «Подготовку томатной массы» и
не читается как её вход (SC-004).

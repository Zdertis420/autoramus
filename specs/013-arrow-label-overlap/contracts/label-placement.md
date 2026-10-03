# Контракт: место и рамка подписи

## 1. Внешние интерфейсы не меняются

| Поверхность | Изменение |
|---|---|
| CLI `ramusc`: команды, флаги, коды возврата | нет |
| `--json` диагностики, коды `internal/diag` | нет |
| Входной язык, `model.schema.json` | нет |
| IR (`internal/ir`) | нет |
| `internal/layout` | не трогается |
| Таблицы, колонки, число строк `.rsf` | нет |

Меняются **значения** `TEXT_X`, `TEXT_Y`, `TEXT_WIDTH`, `TEXT_HIEGHT` у
показанных подписей. `SHOW_TEXT`, `TRANSPARENT` и то, какой сектор подписан, —
прежние (правило PR #4).

## 2. Ширина текста

```go
// glyphs.go — сгенерировано evidence/Glyphs.java, руками не править.
// Ширина знака в Dialog 8: наибольшая из DejaVu Sans (Linux) и Liberation
// Sans (метрика Arial, Windows).
var glyphWidth = map[rune]float64{ /* 166 знаков */ }

// textWidth — ширина строки: сумма ширин знаков. Знак вне таблицы — ширина
// самого широкого знака таблицы.
func textWidth(s string) float64
```

## 3. Раскладки подписи

```go
// layouts — разбиения имени на строки: сначала одна строка, затем всё больше
// строк, каждая с наименьшей возможной шириной самой широкой строки. Разбиение
// — по пробелам и после дефиса, как у Ramus (TextPaintCache, BreakIterator).
func layouts(name string) []textLayout
```

Ширина раскладки ≥ ширины самого длинного слова всегда.

## 4. Расстановка

```go
// placeLabels расставляет подписи всех диаграмм модели.
//
// Вызывается из writeSectors до записи секторов; writeSector получает готовую
// рамку вместо вызова labelBox.
func placeLabels(source *ir.Model) map[*ir.Segment]label
```

| Правило | Значение |
|---|---|
| кто подписан | `labeled(seg)` — без изменений |
| отступ от своей линии | `labelGap = 3` |
| предел удаления | 34 от ближайшего отрезка своей линии |
| жёсткие препятствия | поставленные подписи, блоки диаграммы, край листа |
| мягкие препятствия | отрезки чужих потоков |
| порядок подписей | порядок стрелок документа (research Р-5: «самые стеснённые первыми» выигрыша не дали) |
| выбор кандидата | research Р-4 |

## 5. Инварианты, которые проверяются тестами

Новые, в `internal/generate/sector_test.go`, по собранному файлу на всём наборе
(`testdata/documents/*.yaml` перечнем + примеры):

| Инвариант | Тест |
|---|---|
| подписи не пересекаются, не заходят на блоки и за лист; исключения названы поимённо | `TestLabelsDoNotCollide` |
| каждая подпись не дальше 34 от своей линии | `TestLabelNearOwnLine` |
| ширина рамки не меньше самого длинного слова | `TestLabelFitsWords` |
| мера: таблица даёт ширины однострочных подписей Ramus до единицы (13 из 13) | `TestGlyphWidthsMatchRamus` |
| разбиение на строки — только по пробелам и дефисам, от одной строки к многим | `TestLayoutsBreakBetweenWords` (`label_test.go`) |

Обязаны уцелеть без правки:

| Инвариант | Тест |
|---|---|
| подпись одна на пару «диаграмма + поток», у молчащих рамка нулевая | `TestSectorLabelOnce`, `TestSectorAttributeShowText` |
| геометрия стрелок, узлы, ординаты | `TestJunctionEndsShareOrdinates`, `TestCrosspointIsOnePoint`, `TestPortSpacing` |
| две сборки — один файл | `TestDeterministic`, `TestDeterministicWithArrows` |
| круг «документ → файл → документ» | `TestRoundTrip` |

## 6. Эталоны

`testdata/golden/` сняты с файлов Ramus; `dump` рамок подписей не печатает.
`make golden` не нужен. Пересобрать надо `examples/chakhokhbili.rsf` и
`examples/skirt.rsf`.

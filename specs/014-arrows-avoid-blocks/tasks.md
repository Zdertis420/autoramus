---

description: "Task list for 014-arrows-avoid-blocks"
---

# Tasks: Стрелка никогда не идёт сквозь блок

**Input**: Design documents from `/specs/014-arrows-avoid-blocks/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/no-arrow-through-block.md, quickstart.md

**Tests**: включены. Спецификация требует автоматической проверки прямо
(FR-007, FR-012): отрезки меряются по собранному файлу, новый код диагностики
— эталоном.

**Organization**: задачи сгруппированы по историям из `spec.md`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: можно делать параллельно (разные файлы, нет зависимости от незакрытых задач)
- **[Story]**: к какой истории относится задача (US1, US2, US3)
- В описании — точный путь к файлу

## Path Conventions

Пути от корня репозитория. Обход и сдвиг блоков — в `ramusc/internal/layout`;
предупреждение — в `ramusc/internal/validate` и `ramusc/internal/diag`;
проверка по собранному файлу — в `ramusc/internal/generate`. Генератор не
трогается.

Тесты `internal/layout` — внешние (`package layout_test`) и зовут только
`layout.Apply`. Обходчик проверяется напрямую внутренним тестом
`ramusc/internal/layout/detour_internal_test.go` (`package layout`).

Правила, на которые ссылаются задачи:

* кандидаты обхода — data-model «Кандидат маршрута», research Р-1, Р-2;
* доказательство, что коридор чист всегда, — research И2;
* сдвиг авто-блоков — contracts §4, research Р-4;
* ступень геометрии и предупреждение — contracts §1–§2, research Р-5.

### Где `skirt-full` станет зелёным

`TestNoArrowThroughBlock` пишется в US1 и краснеет на «чахохбили» и
`skirt-full`. US1 лечит «чахохбили»; у `skirt-full` причина другая — авто-блок
лёг на авторский (research И3), — и её лечит US3. Между ними `skirt-full`
остаётся красным, и это ожидаемо.

---

## Phase 1: Setup (мера)

**Purpose**: записать числа, по которым будет видно, что сделано

- [X] T001 Собрать `ramusc` (`cd ramusc && make build`) и снять меру до по `specs/014-arrows-avoid-blocks/quickstart.md` шаг 1 скриптом `specs/014-arrows-avoid-blocks/evidence/through.py`: «чахохбили» — 2 отрезка сквозь «Подготовку томатной массы» («Курица с луком»), `skirt-full` — 5, остальные документы — 0. Если числа другие — остановиться и выяснить почему
- [X] T002 [P] Собрать каждый из `ramusc/testdata/documents/*.yaml` и `examples/*.yaml` в `/tmp/f014/before-<имя>.rsf` — по ним в T031 видно, что изменилось

**Checkpoint**: известно, что мерить и с чем сверять

---

## Phase 2: Foundational (обходчик)

**Purpose**: один перебор кандидатов, которым пользуются все категории маршрутов

- [X] T003 [P] Написать `TestApplyIsIdempotent` в `ramusc/internal/layout/layout_test.go`: на `examples/chakhokhbili.yaml`, `examples/skirt-full.yaml`, `examples/diamond-production.yaml` и `testdata/documents/feedback.yaml` второй `layout.Apply` не меняет `m.Layout` (сравнить `fmt.Sprintf("%+v")` снимков, как в research И4). Зелёный сразу — это сторож для US3, где раскладка переедет в `validate.Build`
- [X] T004 Написать `TestDetourFallsBackToCorridor` в `ramusc/internal/layout/detour_internal_test.go` (`package layout`): блоки, в которых все полосы из `lanes()` между источником и приёмником закрыты — обходчик возвращает путь через коридор над блоками, не задевающий ни одного блока (`crossesRoute` ложно); если закрыт и коридор над (блок до верха листа) — через коридор под. Если прежний шаблон чист — возвращается он же, точка в точку. Обязан не компилироваться/покраснеть до T005
- [X] T005 В `ramusc/internal/layout/route.go` добавить обходчик по research Р-1, Р-2: на входе — прежний шаблон (как функция от полосы, по образцу `pick`), точки и направления начала и конца, блоки диаграммы; перебор — (1) прежний шаблон, (2) тот же с полосами из `lanes()` по близости к прежней, (3) пять-шесть точек через `corridorAbove`: выход вбок на `stub` от своей стороны, коридор, подход к концу с его стороны на `stub` (для управления — вертикаль в порт сверху), (4) то же через `corridorBelow`. Первый, у которого `crossesRoute` ложно. Комментарий — почему кандидат (3)/(4) чист всегда в собственной лестнице (research И2) и почему это не поиск пути
- [X] T006 Проверить, что `TestDetourFallsBackToCorridor` зелёный, а все тесты `ramusc/internal/layout` — зелёные без правки (обходчик пока нигде не вызван)

**Checkpoint**: обходчик есть и проверен отдельно; поведение раскладки не изменилось

---

## Phase 3: User Story 1 — Ни одна стрелка раскладки не идёт сквозь блок (Priority: P1) 🎯 MVP

**Goal**: ни один отрезок, проложенный раскладкой, не проходит внутри блока своей диаграммы

**Independent Test**: `go test ./internal/generate/ -run TestNoArrowThroughBlock` — «чахохбили» зелёный (`skirt-full` — после US3); `evidence/through.py` на «чахохбили» — 0

### Tests for User Story 1

- [X] T007 [US1] Написать `TestNoArrowThroughBlock` в `ramusc/internal/generate/sector_test.go`: по `buildFrom` на наборе `labelDocuments(t)` (все `testdata/documents/*.yaml` и примеры поставки) для каждого сектора — ни один его отрезок не проходит внутри блока своей диаграммы (`m.Functions()` с `Parent == s.Diagram`, строгие неравенства с допуском 1e-6, как `evidence/through.py`). Секторы авторских стрелок пропускаются: пары «поток + диаграмма» из `layout.arrows` документа (по образцу `pinnedSides` фичи 012; диаграмма — работа `on` либо элемент модели при `Context`). Сообщение называет документ, поток, работу и отрезок
- [X] T008 [US1] Убедиться, что `TestNoArrowThroughBlock` **красный** на «чахохбили» («Курица с луком» сквозь «Подготовку томатной массы») и `skirt-full` и зелёный на остальных
  - **Итог T008**: T009 сделан вместе с T005 — тест обходчика T004 идёт через настоящий `route()`, и зелёным он стал только с подключённой входной стороной. Поэтому красноту на «чахохбили» проверил откатом `route.go` к `HEAD`: тест ловит «Курицу с луком» (2 отрезка сквозь «Подготовку томатной массы») и проходит на новом коде. `skirt-full` — красный, 5 отрезков, до US3

### Implementation for User Story 1

- [X] T009 [US1] В `ramusc/internal/layout/route.go` провести `forward()` для стороны входа через обходчик: прежний шаблон «вправо, вниз, вправо» по `middle` — первым кандидатом, полосы из `lanes()` — вторыми. Комментарий — дефект «через одну ступень — ровно центр пропущенного блока» (`CLAUDE.md`) и почему прежний шаблон остаётся первым
- [X] T010 [US1] В `ramusc/internal/layout/route.go` провести `forward()` для стороны управления через обходчик: оба прежних шаблона (три точки и обход сверху фичи 012) — первыми кандидатами в прежнем порядке
- [X] T011 [US1] В `ramusc/internal/layout/route.go` провести `forward()` для стороны механизма через обходчик: прежний шаблон через `corridorBelow` с вертикалью `middle` — первым, вертикаль из `lanes()` — вторыми
- [X] T012 [US1] В `ramusc/internal/layout/route.go` в `route()` провести выход к правому краю листа (`a.to.onBorder()`) через обходчик: прямая горизонталь — первым кандидатом; обход — вбок на `stub`, коридор над или под, к краю листа по коридору (конец на краю листа вправе сменить высоту: сшивка уровней держится номером узла, а не координатой)
- [X] T013 [US1] В `ramusc/internal/layout/route.go` добавить коридорный запасной вариант в `fromBorder()` и `feedback()`: где сегодня `pick()` при отсутствии чистой полосы возвращает канонический маршрут, вместо него — кандидаты (3)/(4) обходчика
- [X] T014 [US1] В `ramusc/internal/layout/tree.go` проверить ветки граничного дерева (`tree`): ветка от узла на магистрали к блоку, задевающая блок, строится обходчиком от узла (направление выхода — поперёк магистрали) к порту; магистраль и узлы не трогаются — узел остаётся одной точкой (`TestTreeNodesAreOnePoint`)
- [X] T015 [US1] Проверить, что `TestForwardRouteMatchesRamus`, `TestStaircaseMatchesRamus`, `TestMatchesRamus`, `TestTreeShape`, `TestFeedbackGoesAround` в `ramusc/internal/layout/` зелёные **без правки чисел**: чистые шаблоны, снятые с `тест.rsf`, не тронуты
- [X] T016 [US1] В `ramusc/internal/layout/arrow_test.go` добавить `example("chakhokhbili.yaml")` в `documents()`; в `ramusc/internal/layout/channel_test.go` и `ramusc/internal/layout/tree_test.go` заменить три вызова `overlapDocuments()` на `documents()` и удалить `overlapDocuments` вместе с комментарием о дефекте. Проверить, что `TestGeometryIsLawful` зелёный на «чахохбили»
- [X] T017 [US1] Прогнать `cd ramusc && go test ./...`: `TestNoArrowThroughBlock` зелёный на «чахохбили»; `skirt-full` — ещё красный (лечит US3); прочие тесты зелёные, в том числе `TestNoSegmentOverlap`, `TestLabelsDoNotCollide`, `TestPortSpacing`
  - **Итог T017**: красный только `TestNoArrowThroughBlock/skirt-full` (5 отрезков — блоки внахлёст, US3). Побайтово изменился один документ — «чахохбили»; `skirt-full` не изменился: пока блоки лежат друг на друге, чистого пути нет, и обходчик честно оставляет прежний шаблон

**Checkpoint**: «Курица с луком» обходит «Подготовку томатной массы»; «чахохбили» проверяется наравне со всеми

---

## Phase 4: User Story 2 — Обход читается (Priority: P2)

**Goal**: из чистых обходов выбран ближайший к прежнему рисунку, без лишних изломов

**Independent Test**: `go test ./internal/layout/ -run TestDetourIsNearest`

- [X] T018 [US2] Написать `TestDetourIsNearest` в `ramusc/internal/layout/route_test.go`: на `examples/chakhokhbili.yaml` сегмент «Курицы с луком» на диаграмме «Приготовление чахохбили» — ровно четыре точки (как у прежнего шаблона: не больше двух изломов, US2 сценарий 2), вертикаль — в промежутке между «Подготовкой томатной массы» и «Тушением» (research Р-2), ни один отрезок не задевает блоки
- [X] T019 [US2] Проверить, что `TestDetourIsNearest` зелёный. Если нет — поправить порядок кандидатов в обходчике (`ramusc/internal/layout/route.go`), а не ожидание теста; повторить T015 и T017

**Checkpoint**: обход — соседняя полоса, а не коридор, там, где полоса есть

---

## Phase 5: User Story 3 — Авторская геометрия (Priority: P3)

**Goal**: авто-блоки не ложатся на авторские; авторская стрелка сквозь блок — предупреждение, не правка

**Independent Test**: `go test ./internal/layout/ -run TestAutoBlocksAvoidAuthored`, `go test ./internal/validate/ -run TestGolden`, `ramusc validate testdata/models/arrow-through-block.yaml --json`

### Tests for User Story 3

- [X] T020 [P] [US3] Написать `TestAutoBlocksAvoidAuthored` в `ramusc/internal/layout/override_test.go`: на `examples/skirt-full.yaml` и на синтетической модели (`pin` блока в место, где стоит ступень лестницы соседа) — ни один авто-блок не перекрывает авторский; авторские координаты прежние; авто-блок, которому ничто не мешает, стоит на своей ступени. Обязан покраснеть до T022
- [X] T021 [P] [US3] Создать `ramusc/testdata/models/arrow-through-block.yaml` с шапкой-комментарием, что он проверяет: валидный документ IDEF0 без ошибок, у которого в `layout.arrows` одна авторская стрелка проведена сквозь блок диаграммы (например, горизонталь от края листа прямо через середину блока к соседнему) — единственная диагностика должна быть `arrow_through_block`

### Implementation for User Story 3

- [X] T022 [US3] В `ramusc/internal/layout/place.go` в блок решений добавить `blockClearance = 4 * gap` с комментарием (research Р-4); в `ramusc/internal/layout/layout.go` в `Apply` после расчёта ступеней диаграммы — сдвиг авто-блоков по порядку лестницы: налезает на авторский блок диаграммы или уже поставленный авто-блок — вправо за правый край помехи + `blockClearance`; вышел за правый край листа — вместо этого вниз за нижний край помехи + `blockClearance`; вышел и так — остаётся на ступени (названная граница, комментарий). Авторские блоки не двигаются; авто-блок без помех не двигается (`TestPartialOverride` — без правки)
- [X] T023 [US3] Проверить, что `TestAutoBlocksAvoidAuthored` и `TestPartialOverride` зелёные, а `TestNoArrowThroughBlock` зелёный теперь и на `skirt-full`; в `TestGeometryIsLawful` (`ramusc/internal/layout/route_test.go`) на `skirt-full` проверка пересечений больше не отключается (`blocksOverlap` ложно) — убедиться и, если отключение стало мёртвым кодом для всего набора, оставить его с комментарием, что оно — для авторских блоков внахлёст (FR-005)
- [X] T024 [US3] В `ramusc/internal/diag/code.go` добавить отдельный блок «Геометрия» с `CodeArrowThroughBlock Code = "arrow_through_block"` и комментарием из `contracts/no-arrow-through-block.md` §1
- [X] T025 [US3] Создать `ramusc/internal/validate/geometry.go` с `Geometry(m *ir.Model, authored map[*ir.Segment]bool) diag.List`: для каждого авторского сегмента, отрезок которого проходит внутри блока своей диаграммы (блоки — `m.Layout.Functions` работ этой диаграммы; строгие неравенства), — одно предупреждение `diag.NewWarning(diag.CodeArrowThroughBlock, …)` на сегмент: позиция и путь — первой точки такого отрезка, текст — поток, диаграмма, работа
- [X] T026 [US3] В `ramusc/internal/validate/validate.go` в `Build`: после смысловых проверок без ошибок — запомнить авторские сегменты (`m.Layout.Arrows`, если секция есть), вызвать `layout.Apply(m)`, дописать `Geometry(...)` к диагностике. `Source` идёт через `Build` — убедиться. Комментарий — почему раскладка здесь (research Р-5, Complexity Tracking плана) и что лесенка соблюдена
- [X] T027 [US3] В `ramusc/cmd/ramusc/main.go` в `build()`: модель приходит уже разложенной; вызов `layout.Apply` убрать или оставить с комментарием об идемпотентности (`TestApplyIsIdempotent`). Проверить, что `ramusc examples/chakhokhbili.yaml -o` даёт тот же файл, что до правки этой задачи
- [X] T028 [US3] Снять эталон: `cd ramusc && go test ./internal/validate -run TestGolden -update`; `git diff --stat ramusc/testdata/golden/` — ровно один новый файл `arrow-through-block.diag.json` и ни одного изменённого; проверить в нём `severity: warning`, `line`/`column` точки
- [X] T029 [US3] В `ramusc/cmd/ramusc/main_test.go` добавить проверку: `ramusc validate testdata/models/arrow-through-block.yaml --json` — объект с кодом `arrow_through_block`, код возврата 0; компиляция того же документа — файл записан, код возврата 0, в выводе то же предупреждение (двух наборов сообщений нет). `TestEveryCodeIsExercised` в `ramusc/internal/diag` — зелёный

**Checkpoint**: авто-блоки не на авторских; авторская стрелка сквозь блок — одно предупреждение, сборка проходит

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T030 `cd ramusc && make check` — форматирование, `go vet`, все тесты
- [X] T031 Собрать набор в `/tmp/f014/after-<имя>.rsf` и сравнить `cmp` с T002: `≠` только у `chakhokhbili` и `skirt-full` (SC-003); лишний `≠` — ошибка обходчика, а не повод принять
- [X] T032 Сверка снаружи: `specs/014-arrows-avoid-blocks/evidence/through.py` на файлах T031 — ноль отрезков сквозь блоки на всех документах (SC-001, SC-002, SC-006)
- [X] T033 Пересобрать `examples/chakhokhbili.rsf` (`./ramusc/ramusc examples/chakhokhbili.yaml -o examples/chakhokhbili.rsf`)
- [X] T034 [P] Обновить `CLAUDE.md` (файл в `.gitignore` — правка руками): «Известный дефект, ждущий своей фичи» — закрыт фичей 014, `overlapDocuments` больше нет; раздел «Конвейер» — `validate.Build` заканчивается раскладкой и ступенью геометрии, выдающей только предупреждения; «Классификация стрелок» — маршруты перебирают кандидатов обходчика; «Порядок работ» — пункт 12
- [ ] T035 Открыть `examples/chakhokhbili.rsf` в Ramus, A0: «Курица с луком» обходит «Подготовку томатной массы» и не читается как её вход (SC-004)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: зависимостей нет
- **Phase 2 (Foundational)**: после Phase 1; блокирует US1 и US2
- **Phase 3 (US1)**: после Phase 2
- **Phase 4 (US2)**: после US1 — проверяет выбор кандидата на маршрутах US1
- **Phase 5 (US3)**: сдвиг блоков (T020, T022, T023) — после US1, потому что проверяется `TestNoArrowThroughBlock`; ступень геометрии (T021, T024–T029) — независима от US1 и US2 и может идти параллельно им
- **Phase 6 (Polish)**: после всех

### Within Each User Story

- Тест пишется до реализации и обязан покраснеть (T004, T008, T020)
- Прежний шаблон — всегда первым кандидатом: сначала проверка «чистые не тронуты» (T015), потом всё остальное
- Эталон — только после того, как код и модель стоят (T028)

### Parallel Opportunities

- T002 — отдельно от T001
- T003 — отдельный файл, параллельно T004–T005
- T020, T021 — разные файлы
- T034 — документ, параллельно T031–T033
- T009–T013 последовательны намеренно: все правят `route.go`
- Ступень геометрии US3 (T021, T024–T029) — параллельно US1 и US2: другие пакеты

## Parallel Example: User Story 3

```bash
# Тест сдвига блоков и модель для нового кода — разные файлы:
Task: "TestAutoBlocksAvoidAuthored в ramusc/internal/layout/override_test.go"
Task: "testdata/models/arrow-through-block.yaml"
```

## Implementation Strategy

### MVP (US1)

US1 — прямая просьба автора и сам дефект «чахохбили». После неё ни одна
стрелка, которую прокладывает раскладка на своей лестнице, не идёт сквозь
блок, и «чахохбили» проверяется наравне со всеми.

1. Phase 1 — мера
2. Phase 2 — обходчик, проверенный отдельно
3. Phase 3 — все категории маршрутов через него
4. **Остановиться и проверить**: «чахохбили» — 0 сквозь блоки, чистые
   шаблоны Ramus не тронуты
5. Phase 4 — ближайший обход
6. Phase 5 — авторская геометрия: сдвиг блоков и предупреждение
7. Phase 6 — охрана, документация, взгляд в Ramus

### Порядок, который менять не стоит

`TestForwardRouteMatchesRamus` и `TestStaircaseMatchesRamus` без правки чисел
после каждой задачи T009–T014: их числа сняты с `тест.rsf`, и если они
покраснели, обходчик тронул чистый шаблон.

Красный `TestNoArrowThroughBlock` до правки маршрутов: проверку «ноль
сквозь блоки» подогнать под построенное слишком легко.

## Notes

- `[P]` — разные файлы, нет зависимостей
- Ни языка, ни IR, ни генератора, ни декомпилятора фича не касается
- Новый код диагностики — один; новых зависимостей нет
- Коммит после каждой задачи или логической группы; перед коммитом — `make check`

---

description: "Task list for 015-remove-double-crossings"
---

# Tasks: Без ненужных двойных пересечений

**Input**: Design documents from `/specs/015-remove-double-crossings/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/lane-choice.md, quickstart.md

**Tests**: включены. Спецификация требует автоматической проверки по
собранному файлу (FR-008).

**Organization**: задачи сгруппированы по историям из `spec.md`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: можно делать параллельно (разные файлы, нет зависимости от незакрытых задач)
- **[Story]**: к какой истории относится задача (US1, US2)
- В описании — точный путь к файлу

## Path Conventions

Пути от корня репозитория. Правка — одна функция в
`ramusc/internal/layout/channel.go`; проверки по файлу — в
`ramusc/internal/generate/sector_test.go`.

Правило выбора полосы — `contracts/lane-choice.md` §2; определение
пересечения и лишнего пересечения — `data-model.md`; потолки — `contracts/lane-choice.md` §3.

### Почему счёт пересечений — в основе, а не в US2

Потолки сняты скриптом `evidence/double.py`. Тест, который их сторожит, обязан
считать ровно так же — иначе он сторожит не те числа. Поэтому он пишется до
правки и должен быть **зелёным на сегодняшнем коде**: это проверка счёта, а не
раскладки.

---

## Phase 1: Setup (мера)

- [X] T001 Собрать `ramusc` (`cd ramusc && make build`) и снять меру до по `specs/015-remove-double-crossings/quickstart.md` шаг 1: `evidence/swap.py` — две лишние пары («чахохбили»: «Курица с луком» × «Томатная масса»; `channels-pair`: «первый» × «второй»); `evidence/double.py` — пересечения по таблице потолков `contracts/lane-choice.md` §3. Если числа другие — остановиться и выяснить почему (фича 014 должна быть закоммичена или хотя бы собрана в рабочем дереве)
- [X] T002 [P] Собрать каждый из `ramusc/testdata/documents/*.yaml` и `examples/*.yaml` в `/tmp/f015/before-<имя>.rsf` — для побайтового сравнения в T011

**Checkpoint**: известно, что мерить и с чем сверять

---

## Phase 2: Foundational (счёт пересечений)

- [X] T003 В `ramusc/internal/generate/sector_test.go` добавить помощники по `data-model.md`: `arrowLines(m *rsf.Model) map[[2]int64][][]rsf.Point` — ломаные секторов, сгруппированные по «диаграмма + поток» (сектор без потока — своей группой); `crossings(a, b [][]rsf.Point) int` — число **различных** точек, где горизонталь одной строго внутри вертикали другой (касание концами — нет; допуск 1e-6; точки округлять до 0.01, как `evidence/double.py`); `totalCrossings(m)` — сумма по всем парам разных стрелок одной диаграммы
- [X] T004 Написать `TestCrossingsDoNotGrow` в `ramusc/internal/generate/sector_test.go`: по `buildFrom` на наборе `labelDocuments(t)` — `totalCrossings` не больше потолка из таблицы `contracts/lane-choice.md` §3 (карта «имя документа → потолок» в тесте, с комментарием: замер `evidence/double.py` до фичи 015, снижается осознанно); документ без потолка — `t.Fatalf` (новый документ обязан получить свой). Обязан быть **зелёным сразу**; если красный — счёт расходится со скриптом, чинить счёт, а не потолок

**Checkpoint**: тест считает пересечения ровно как скрипт; потолки стерегут «не хуже»

---

## Phase 3: User Story 1 — Соседние стрелки не перепутаны (Priority: P1) 🎯 MVP

**Goal**: ни одной пары стрелок, которую можно распутать обменом соседних полос

**Independent Test**: `go test ./internal/generate/ -run TestNoAvoidableDoubleCrossing`; `evidence/swap.py` — ноль лишних пар

### Tests for User Story 1

- [X] T005 [US1] Написать `TestNoAvoidableDoubleCrossing` в `ramusc/internal/generate/sector_test.go` — порт `evidence/swap.py`: на наборе `labelDocuments(t)` для каждой пары стрелок одной диаграммы с `crossings ≥ 2` перебрать пары внутренних отрезков (не первый и не последний отрезок ломаной — те прицеплены к блоку, краю или узлу), по одному от каждой стрелки, параллельных, с расстоянием по координате не больше `2 * 6` и общей протяжённостью; поменять их координаты местами (сдвинув обе точки каждого отрезка, как `move` в скрипте) и пересчитать `crossings` пары. Если стало меньше — ошибка: документ, два потока, было → стало
- [X] T006 [US1] Убедиться, что `TestNoAvoidableDoubleCrossing` **красный** ровно на «чахохбили» (2 → 0) и `channels-pair.yaml` (2 → 0) и зелёный на остальных

### Implementation for User Story 1

- [X] T007 [US1] В `ramusc/internal/layout/channel.go`: в `channels()` собрать для каждой диаграммы список её стрелок (в порядке `all`, без обхода отображений) и передать его в `spread`, а из `spread` — в `pickLane`
- [X] T008 [US1] В `ramusc/internal/layout/channel.go` добавить помощник числа пересечений стрелок заявки с остальными стрелками диаграммы после пробного сдвига на полосу (пробный сдвиг и возврат точек — как в `damages`; пересечение — горизонталь строго внутри вертикали, стрелки одного потока друг другу не пересечения). В `pickLane` на каждом шаге `k` оценить обе свободные полосы `канон + k·шаг`, `канон − k·шаг` и вернуть с меньшим числом пересечений, при равенстве — `канон + k·шаг`; канон, если свободен, — по-прежнему без оценки. Комментарий — research И1 (сторона выбиралась порядком перебора) и почему шаг от канона важнее пересечений (research Р-1)
- [X] T009 [US1] В `ramusc/internal/layout/channel_test.go` в `TestCanonicalLaneKept` поменять ожидание «второго» с `canonical+step` на `canonical-step` и дописать в комментарий теста: направление сдвига не было правилом — оно следовало из порядка перебора; теперь сторона выбирается по пересечениям, и с `−6` «второй» не пересекает «первого» (research И2). «Первый на каноне» и «ровно один шаг» — без изменений
- [X] T010 [US1] Проверить, что `TestNoAvoidableDoubleCrossing`, `TestCanonicalLaneKept` и `TestCrossingsDoNotGrow` зелёные

**Checkpoint**: лишних пар ноль; у входа в «Тушение» стрелки не перекрещиваются

---

## Phase 4: User Story 2 — Рисунок не становится хуже в другом месте (Priority: P2)

**Goal**: общее число пересечений не выросло нигде; прочие свойства раскладки целы

**Independent Test**: `go test ./...`; побайтовое сравнение с T002

- [X] T011 [US2] Собрать набор в `/tmp/f015/after-<имя>.rsf` и сравнить `cmp` с T002: `≠` ровно у `chakhokhbili`, `channels-pair`, `channels-trunk` (research И2); лишний `≠` — выяснить, не подгонять
- [X] T012 [US2] Опустить потолки `TestCrossingsDoNotGrow` в `ramusc/internal/generate/sector_test.go` до новых значений (`evidence/double.py` на файлах T011: «чахохбили» 156, `channels-pair` 0, `channels-trunk` 4), с комментарием, что потолок — храповик: снижается, когда раскладка становится лучше, и не поднимается молча
- [X] T013 [US2] Прогнать `cd ramusc && go test ./...`: зелёные без правки `TestNoSegmentOverlap`, `TestSharedLineDisjointSpansUnmoved`, `TestChannelsAreStable`, `TestAuthoredGeometryIsObstacle`, `TestGeometryIsLawful`, `TestNoArrowThroughBlock`, `TestLabelsDoNotCollide`, `TestPortSpacing`, `TestTreeNodesAreOnePoint`, `TestDeterministic`

**Checkpoint**: улучшение нигде не оплачено ухудшением

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T014 `cd ramusc && make check`
- [X] T015 Сверка снаружи: `specs/015-remove-double-crossings/evidence/swap.py` на файлах T011 — ноль лишних пар (SC-001); `evidence/double.py` — «чахохбили» 156 (SC-003)
- [X] T016 Пересобрать `examples/chakhokhbili.rsf` (`./ramusc/ramusc examples/chakhokhbili.yaml -o examples/chakhokhbili.rsf`)
- [X] T017 [P] Обновить `CLAUDE.md` (файл в `.gitignore` — правка руками): в разделе «4. Разведение по каналам» — «каждая следующая отходит на `channelStep` в ту из сторон, где у неё меньше пересечений с остальными стрелками; при равенстве — прежняя»; в «Порядке работ» — пункт 13
- [ ] T018 Открыть `examples/chakhokhbili.rsf` в Ramus, A0, вход в «Тушение»: «Курица с луком» и «Томатная масса» идут рядом, не перекрещиваясь (SC-005)

---

## Dependencies & Execution Order

- **Phase 1** → **Phase 2** → **Phase 3 (US1)** → **Phase 4 (US2)** → **Phase 5**
- US2 проверяет то, что сделала US1; своего кода у неё нет — только мера и храповик
- Внутри US1: тест красный (T006) — до правки `channel.go` (T007–T008); правка ожидания `TestCanonicalLaneKept` (T009) — после правки кода, когда видно, куда встал «второй»

### Parallel Opportunities

- T002 — отдельно от T001
- T017 — документ, параллельно T014–T016
- Остальное последовательно: T003–T005 и T012 правят один `sector_test.go`, T007–T008 — один `channel.go`

## Parallel Example: Polish

```bash
Task: "Сверка снаружи evidence/swap.py и double.py"
Task: "Правка CLAUDE.md"
```

## Implementation Strategy

### MVP (US1)

US1 — случай со скриншота и весь смысл фичи: после неё у входа в «Тушение»
нет перекрещенных стрелок. US2 — страховка, что это не оплачено в другом месте;
она короткая и идёт сразу следом.

### Порядок, который менять не стоит

`TestCrossingsDoNotGrow` зелёный **до** правки: иначе потолки стерегут не то.
`TestNoAvoidableDoubleCrossing` красный **до** правки: иначе неизвестно, что он
вообще что-то ловит.

## Notes

- `[P]` — разные файлы, нет зависимостей
- Ни языка, ни IR, ни генератора, ни валидатора фича не касается
- Коммит после логической группы; перед коммитом — `make check`

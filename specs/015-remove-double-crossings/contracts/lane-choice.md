# Контракт: выбор полосы при разведении по каналам

## 1. Внешние интерфейсы не меняются

| Поверхность | Изменение |
|---|---|
| CLI, коды возврата, `--json`, коды диагностики | нет |
| Входной язык, схема, IR | нет |
| `layout.Apply` | подпись та же |
| `internal/generate`, `internal/validate` | не трогаются |

Меняются координаты стрелок, которым при разведении пришлось отойти с
канонической линии и у которых другая сторона даёт меньше пересечений. На
наборе — «чахохбили», `channels-pair`, `channels-trunk`.

## 2. Правило

`internal/layout/channel.go`, `pickLane`:

```
канон свободен                → канон
иначе для k = 1…maxChannels:
    свободные из {канон + k·шаг, канон − k·шаг}
    нет свободных             → следующий k
    одна                      → она
    обе                       → с меньшим числом пересечений; равно — канон + k·шаг
ничего не нашлось             → канон (названная граница, как прежде)
```

Число пересечений — стрелки заявки с другими стрелками диаграммы, после
пробного сдвига (точки возвращаются на место, как в `damages`).

## 3. Тесты

Новые, `internal/generate/sector_test.go`, по собранному файлу на всём наборе:

| Инвариант | Тест |
|---|---|
| ни одной пары, у которой обмен соседних полос уменьшает пересечения | `TestNoAvoidableDoubleCrossing` |
| пересечений на документе не больше потолка «до» | `TestCrossingsDoNotGrow` |

Потолки (`evidence/double.py` до фичи): `branch-border` 2,
`channels-feedback` 16, `channels-pair` 2, `channels-trunk` 5, `dfd-tunnel` 0,
`feedback` 1, `labels-crowded` 0, `nested` 2, `staircase` 0, `two-sides` 1,
`chakhokhbili` 158, `diamond-production` 10, `skirt-full` 16, `skirt` 8.

Правится осознанно: `TestCanonicalLaneKept` — «второй» на `261 − 6`, а не
`261 + 6`: сторона выбирается по пересечениям, и с `−6` их ноль (research И2).

Обязаны уцелеть без правки: `TestNoSegmentOverlap`,
`TestSharedLineDisjointSpansUnmoved`, `TestChannelsAreStable`,
`TestAuthoredGeometryIsObstacle`, `TestGeometryIsLawful`,
`TestNoArrowThroughBlock`, `TestLabelsDoNotCollide`, `TestPortSpacing`,
`TestTreeNodesAreOnePoint`, `TestDeterministic`.

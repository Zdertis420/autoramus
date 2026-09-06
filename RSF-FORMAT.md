# Формат файлов Ramus (`*.rsf`)

Разобрано по файлам, сохранённым Ramus 2.0, и сверено с исходниками:
<https://github.com/Vitaliy-Yakovchuk/ramus> (GPL).

Ключевые классы:

| Класс | Роль |
|---|---|
| `com.ramussoft.core.impl.FileIEngineImpl` | `open()` / `saveToFile()` — что и в каком порядке кладётся в ZIP |
| `com.ramussoft.core.impl.TableToXML` | выгрузка таблицы БД в XML |
| `com.ramussoft.core.impl.XMLToTable` | загрузка XML обратно в БД |
| `com.ramussoft.database.MemoryDatabase` | поднимает H2 in-memory (`jdbc:h2:mem:`) |
| `com.ramussoft.core.persistent.PersistentFactory` | строит схему таблиц атрибутов из Java-аннотаций |
| `com.ramussoft.idef0.IDEF0Plugin` | имена системных квалификаторов/атрибутов IDEF0 |

---

## 1. Контейнер

`.rsf` — обычный ZIP (deflate). Ничего не зашифровано, контрольных сумм поверх ZIP нет.

```
data/application_metadata.xml     java.util.Properties (storeToXML): версия + список плагинов
data/sequences.xml                java.util.Properties: значения SQL-последовательностей
data/<таблица>.xml                дампы системных таблиц
data/Core/attribute_*.xml         значения атрибутов плагина Core
data/IDEF0/attribute_*.xml        значения атрибутов плагина IDEF0
data/Chart/…, data/Eval/…         прочие плагины
properties/…, user/…              произвольные «потоки» (streams), список в data/streams.xml
```

**Порядок entry в ZIP не важен** — загрузка идёт через `ZipFile.getInputStream(new ZipEntry(имя))`.

При сохранении Ramus переписывает всё, что начинается с `data/`, а остальные entry
копирует из старого файла как есть. Значит, свои файлы можно класть куда угодно, кроме
`data/` — но чтобы Ramus видел их как поток, имя надо добавить в `data/streams.xml`.

### `data/application_metadata.xml`

```xml
<entry key="ApplicationName">Ramus</entry>
<entry key="ApplicationVersion">2.0</entry>
<entry key="FileOpenMinimumVersion">2.0</entry>
<entry key="PluginCount">28</entry>
<entry key="Plugin_0">Attribute.Core.Hierarchical</entry>
…
```

Ловушка: при открытии `checkFileVersion()` требует, чтобы **каждый** `Plugin_N` из файла
присутствовал в сборке Ramus, иначе `FileVersionException`. Не стоит добавлять плагины,
которых нет; лишние (не используемые) убирать можно, но проще не трогать блок вообще.

### `data/sequences.xml`

Сохраняются только последовательности, объявленные плагинами: у IDEF0 это
`ordinates__sequence` и `crosspoint_sequence`. Остальные (`elements_sequence`,
`qualifiers_sequence`, `attributes_sequence`) создаются заново со START 1, а
`createElement()` при коллизии перематывает их до `MAX(ELEMENT_ID)+1`. Поэтому
**новые id можно смело выдавать как `MAX+1`** — Ramus самовосстановится.

При добавлении стрелок вручную `ordinates__sequence` и `crosspoint_sequence` нужно
увеличивать самостоятельно: они не самовосстанавливаются.

---

## 2. Формат таблицы

```xml
<?xml version="1.0" encoding="UTF-8"?>
<table generate-from-table="elements" generate-time="Sat Sep 05 18:31:17 MSK 2026" prefix="ramus_">
  <fields>
    <field id="0" name="ELEMENT_ID" type="BIGINT"/>
    <field id="1" name="ELEMENT_NAME" type="CLOB"/>
    …
  </fields>
  <data>
    <row><f id="0">5</f><f id="1"/><f id="2">14</f><f id="3">0</f><f id="4">2147483647</f></row>
  </data>
</table>
```

Правила `XMLToTable.load()` — это и есть «спецификация записи»:

1. Колонка ищется **по имени** (`<field name=…>` → имя колонки в БД), `id` — лишь ссылка
   для `<f id=…>`. Порядок и состав полей можно менять; можно опускать колонки целиком.
2. Отсутствующий `<f>` → NULL, **но** для `*_branch_id` подставляется `0`,
   а для `removed_branch_id` — `2147483647`.
3. `generate-time` и `prefix` при чтении игнорируются.
4. Типы и конвертеры:

| `type` | Конвертер |
|---|---|
| `CLOB`, `CHAR`, `TEXT`, `bpchar` | строка как есть |
| `BIGINT`, `LONG`, `int8` | long |
| `INTEGER`, `int4` | int |
| `DOUBLE`, `float8` | double (`Double.parseDouble`, точка-разделитель) |
| `BOOLEAN`, `BOOL` | `TRUE` / `FALSE` |
| `TIMESTAMP` | `DateFormat.SHORT/SHORT, Locale.ENGLISH` → `9/4/26 10:23 AM` |
| `BLOB`, `VARBINARY`, `bytea` | hex, **каждый байт сдвинут на +128** |

Про BLOB подробнее (`TableToXML.ByteAConverter`): пишется `digits[byte + 128]`,
читается `(byte)(val - 128)`. Пример: `C4E9E1ECEFE7` → `44 69 61 6C 6F 67` = `Dialog`.
Сами блобы (`attribute_sectors.VISUAL_ATTRIBUTES`, `attribute_visual_datas.DATA`) —
это самописный поток `DataSaver`/`DataLoader` (little-endian double и т.п.), но он
необязателен: пустой блоб = визуальные настройки по умолчанию.

### Версионирование строк (ветки)

Почти в каждой таблице есть `CREATED_BRANCH_ID` / `REMOVED_BRANCH_ID`:

* живая строка — `0` и `2147483647`;
* удалённая — `REMOVED_BRANCH_ID = 0` (строка остаётся в файле!).

Удаление объекта — это пометка, а не физическое стирание. Поэтому в реальных файлах
обычно валяется некоторое количество «мусорных» строк от прошлых правок.

---

## 3. Метамодель (EAV)

Ramus хранит не «IDEF0-диаграмму», а универсальную объектную базу. IDEF0 — просто
набор системных квалификаторов и атрибутов поверх неё.

| Таблица | Смысл |
|---|---|
| `qualifiers` | «типы объектов» / таблицы. `ATTRIBUTE_FOR_NAME` — атрибут, играющий роль имени |
| `attributes` | «колонки»: `ATTRIBUTE_TYPE_PLUGIN_NAME` + `ATTRIBUTE_TYPE_NAME`, напр. `Core.Text` |
| `qualifiers_attributes` | какие атрибуты есть у квалификатора (+ `ATTRIBUTE_POSITION`) |
| `elements` | объекты: `ELEMENT_ID`, `QUALIFIER_ID`. `ELEMENT_NAME` почти всегда пустое |
| `data/<Плагин>/attribute_*.xml` | значения: ключ `(ATTRIBUTE_ID, ELEMENT_ID, VALUE_BRANCH_ID)` |
| `persistents`, `persistent_fields` | описание схемы таблиц значений |

**Важно:** `persistents` и `persistent_fields` при открытии файла *не читаются*
(строки `loadTable("", "persistents")` закомментированы) — схема пересобирается
`PersistentFactory.rebuild()` из аннотаций (`@Table`, `@Integer(id=…)`, `@Binary(id=…)`).
Список плагин/таблица оттуда используется только при выгрузке. Менять их не надо,
удалять — тоже.

Имя объекта берётся не из `elements.ELEMENT_NAME`, а из текстового атрибута,
указанного в `qualifiers.ATTRIBUTE_FOR_NAME`. Для работ это пользовательский
атрибут «Название», для потоков — системный `F_STREAM_NAME`.

### Иерархия

`Core/attribute_hierarchicals.xml` задаёт дерево: `PARENT_ELEMENT_ID` +
`PREVIOUS_ELEMENT_ID` (односвязный список братьев; `-1` = первый). Порядок детей
определяется цепочкой `PREVIOUS`, а не порядком строк.

---

## 4. Карта IDEF0

### Квалификаторы

Нумерация системных квалификаторов стабильна для файлов Ramus 2.0:

| id | Имя | Что это |
|---|---|---|
| 1–5 | `HistoryQualifier`, `QualifiersQualifier`, `AttributesQualifier`, `IconsQualifier`, … | служебные |
| 6 | `F_SECTORS` | сегменты стрелок |
| 7 | `F_STREAMS` | стрелки-сущности (потоки) |
| 8 | `F_BASE_FUNCTIONS` | модели; атрибут `F_BASE_FUNCTION_QUALIFIER_ID` указывает на квалификатор работ |
| 9 | `F_MODEL_TREE` | дерево моделей проекта |
| 10–13 | отчёты, диаграммы чартов | |
| 14+ | пользовательский, напр. `Работы` | **работы (функциональные блоки) модели** |

Как найти квалификатор работ программно: взять элемент из `F_BASE_FUNCTIONS`, у которого
атрибут `F_BASE_FUNCTION_QUALIFIER_ID` указывает не на сам `F_BASE_FUNCTIONS`.

### Атрибуты

id ниже — типичные для файла с одной моделью; полагаться на них не стоит, надёжнее
искать атрибут по имени в `attributes`.

| id | Имя | Тип | Таблица значений |
|---|---|---|---|
| 1 | `HierarchicalAttribute` | `Core.Hierarchical` | `Core/attribute_hierarchicals` |
| 20 | `F_VISUAL_DATA` | `IDEF0.VisualData` | `IDEF0/attribute_visual_datas` |
| 21 | `F_PAGE_SIZE` | `Core.Text` | `Core/attribute_texts` |
| 22 / 23 | `F_BACKGROUND` / `F_FOREGROUND` | `IDEF0.Color` | `IDEF0/attribute_colors` (int ARGB: `-1` белый, `-16777216` чёрный) |
| 24 | `F_BOUNDS` | `IDEF0.FRectangle` | `IDEF0/attribute_rectangles` (X, Y, WIDTH, HEIGHT — double) |
| 25 | `F_FONT` | `IDEF0.Font` | `IDEF0/attribute_fonts` (NAME/SIZE/STYLE) |
| 26 | `F_STATUS` | `IDEF0.Status` | `IDEF0/attribute_statuses` |
| 27 | `F_TYPE` | `IDEF0.Type` | `IDEF0/attribute_function_types` |
| 29 | `F_DECOMPOSITION_TYPE` | | `IDEF0/attribute_decomposition_types` (`-1` = не задан) |
| 30–33 | автор, даты создания/ревизии | | `Core/attribute_texts`, `Core/attribute_dates` |
| 35 | `F_FUNCTION_SECTOR` | `Core.OtherElement` | **на чьей диаграмме нарисован сегмент** |
| 36 | `F_SECTOR_STREAM` | `Core.OtherElement` | какой поток несёт сегмент |
| 37 | `F_SECTOR_POINTS` | | `IDEF0/attribute_sector_points` |
| 38 | `F_SECTOR_PROPERTIES` | | подпись сегмента (позиция текста, тильда) |
| 39 | `F_STREAM_NAME` | `Core.Text` | имя потока |
| 40 | `F_SECTOR_ATTRIBUTE` | `IDEF0.Sector` | стиль линии (блоб) |
| 41 / 42 | `F_SECTOR_BORDER_START` / `_END` | | `IDEF0/attribute_sector_borders` |
| 44 | `F_BASE_FUNCTION_QUALIFIER_ID` | `Core.Long` | |
| 45 | `F_PROJECT_PREFERENCES` | | `IDEF0/attribute_model_preferences` (автор, имя проекта, размер листа) |
| 55 | `Название` | `Core.Text` | пользовательский, имя работы |

### Типы блоков (`F_TYPE`, `com.ramussoft.pb.Function`)

```
0 TYPE_PROCESS_KOMPLEX   1 TYPE_PROCESS        2 TYPE_PROCESS_PART
3 TYPE_OPERATION         4 TYPE_ACTION
1001 EXTERNAL_REFERENCE  1002 DATA_STORE       1003 DFDS_ROLE   (DFD)
```

Корневой блок контекстной диаграммы обычно `3`, дочерние работы — `1`.

### Стрелки

Стрелка в модели — это **поток** (`F_STREAMS`) плюс набор **секторов** (`F_SECTORS`),
по одному на каждый нарисованный сегмент на каждой диаграмме. Сектор:

* `F_FUNCTION_SECTOR` — на чьей диаграмме сегмент нарисован. Для контекстной
  диаграммы A-0 это элемент модели из `F_BASE_FUNCTIONS`, для A0 — корневая
  работа, для декомпозиций — соответствующая работа.
* `F_SECTOR_STREAM` — поток.
* начало/конец — строки в `attribute_sector_borders` с атрибутом
  `F_SECTOR_BORDER_START` или `_END`:
  * `FUNCTION` ≥ 0 → конец прицеплен к блоку, сторона в `FUNCTION_TYPE`;
  * `BORDER_TYPE` ≥ 0 → конец на краю листа (туннель на верхний уровень);
  * иначе `CROSSPOINT` → узел ветвления/слияния;
  * строки может не быть вовсе — «висящий» неприсоединённый конец.
* геометрия — `attribute_sector_points` (`X_ORDINATE_ID`/`Y_ORDINATE_ID` — общие
  «направляющие», чтобы соседние стрелки выравнивались; отсюда `ordinates__sequence`).

Стороны (`FUNCTION_TYPE`, `MovingPanel`):

```
0 RIGHT  — выход
1 BOTTOM — механизм
2 LEFT   — вход
3 TOP    — управление
```

`BORDER_TYPE` использует те же коды для края листа.

---

## 5. Как писать в файл

### Вариант A: править XML напрямую

Годится для: переименований, координат/размеров, добавления и удаления работ,
добавления потоков, массовых правок и генерации моделей из внешних данных.

Чек-лист для новой работы — нужны строки во **всех** этих таблицах:

| Таблица | Что |
|---|---|
| `elements` | `ELEMENT_ID` = `MAX+1`, `QUALIFIER_ID` = квалификатор работ, `ELEMENT_NAME` = пусто |
| `Core/attribute_hierarchicals` | `HierarchicalAttribute`: `PARENT_ELEMENT_ID`, `PREVIOUS_ELEMENT_ID` (последний брат или `-1`), `ICON_ID=-1` |
| `Core/attribute_texts` | атрибут-имя квалификатора работ |
| `IDEF0/attribute_rectangles` | `F_BOUNDS`: X, Y, WIDTH, HEIGHT |
| `IDEF0/attribute_function_types` | `F_TYPE`: `1` |
| `IDEF0/attribute_statuses` | `F_STATUS`: `TYPE=0` |
| `IDEF0/attribute_fonts` | `F_FONT`: `Dialog / 10 / 0` |
| `IDEF0/attribute_colors` | `F_BACKGROUND` (`-1`) и `F_FOREGROUND` (`-16777216`) |
| `IDEF0/attribute_decomposition_types` | `F_DECOMPOSITION_TYPE`: `-1` |

Везде `VALUE_BRANCH_ID=0`; в `elements` — `CREATED_BRANCH_ID=0`,
`REMOVED_BRANCH_ID=2147483647`.

Удаление — не стирать строку, а поставить `REMOVED_BRANCH_ID=0` в `elements`.

Стрелки этим способом делать можно, но муторно: нужно согласованно создать сектор,
две строки границ, точки с ординатами, свойства подписи и обновить
`ordinates__sequence`/`crosspoint_sequence`.

### Вариант B: Ramus core как Java-библиотека (рекомендуется для стрелок)

```java
Database db = FileDatabaseFactory.createDatabase(new File("model.rsf"));
Engine engine = db.getEngine(null);
// ... правки через Engine / RowSet / IDEF0Plugin
((FileIEngineImpl) engine.getDeligate()).saveToFile(new File("model.rsf"));
```

Вся логика ординат, кросспоинтов, туннелирования и согласования уровней
декомпозиции остаётся внутри Ramus. Готовый пример чтения —
`ramus-core-demo/src/main/java/com/ramussoft/demo/RSFViewer.java` в репозитории.
Перед первым вызовом полезно выставить
`System.setProperty("user.ramus.application.name", "MyTool")`, чтобы не конфликтовать
с сессиями настоящего Ramus (он держит `~/.ramus/sessions/*/.lock`).

### Вариант C: с нуля

Технически возможно (файл самодостаточен), но проще держать пустой `.rsf` как шаблон
и наполнять его вариантом A или B.

---

## 6. Подводные камни

* Не менять `Plugin_N` в `application_metadata.xml`.
* Не забывать про `PREVIOUS_ELEMENT_ID` — иначе новая работа не появится в дереве
  (объект будет, а в списке его не видно).
* Даты в `TIMESTAMP` — только английский короткий формат (`9/4/26 10:23 AM`),
  иначе `ParseException` и NULL.
* Double пишется с точкой (`186.0`), не с запятой.
* `<f id="1"/>` (пустой) и отсутствие `<f>` — **разные вещи**: первое — пустая строка,
  второе — NULL.
* Ramus при открытии копирует файл в сессионную папку и держит на ней блокировку;
  файл стоит править, когда он закрыт в программе.
* Резервная копия обязательна: невалидная строка чаще всего не роняет открытие,
  а тихо теряет объект.

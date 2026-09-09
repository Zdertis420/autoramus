# Specification Quality Checklist: Устранение дефектов валидатора и декомпилятора

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-08
**Updated**: 2026-09-08 — редакция 3, поправки SC-003 и FR-013 по итогам планирования
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

**Редакция 2.** Автор добавил `ramusc/testdata/testModel.rsf`. Спецификация
перестроена под факты, снятые с этого файла.

Снято: ограничение редакции 1 «полный перенос классификаторов нечем проверить на
настоящем файле». В новой модели пять пользовательских справочников, записанных
самим Ramus.

Добавлена история 2 (P1) — вывод декомпилятора не проходит проверку входного языка
на новой модели: 19 ошибок из-за работ без имени, плюс повторы в списках ICOM при
ветвлении. Дефект существовал и раньше, но первая модель его не показывала.

История «несколько проверочных моделей» (теперь 4) понижена в объёме: файл уже в
репозитории и round-trip на нём проходит, остаётся подключить его к проверкам.
Часть про элементы DFD осталась отложенной — их нет ни в одной из двух моделей.

Требования перенумерованы: было 17, стало 19. Соответствие критериев успеха:
было 8, стало 10, и все привязаны к измеренным сегодняшним значениям
(19 ошибок, 0 из 5 справочников, 7 повторов ICOM, 35 из 54 кодов).

**Редакция 3.** Планирование вскрыло два расхождения с фактами, оба исправлены.

SC-003 требовал восстановить «все колонки с типами и все строки со значениями»
пяти справочников. Справочники в `testModel.rsf` пусты: ноль строк, ни одной
пользовательской колонки. Критерий переформулирован на обнаружение пяти
справочников с верными именами; чтение значений отнесено к тестам на собранных
вручную таблицах с явной пометкой об ограничении.

FR-013 приписывал повторы в списках ICOM ветвлению. Причина оказалась другой:
работы адресуются по имени, а у восьми из девяти имени нет, поэтому связи разных
работ сходятся на одной. Требование переформулировано на адресацию по идентичности;
правило о единственном упоминании потока сохранено как дополнительное.

Приведены в соответствие: строка таблицы моделей, повествование истории 2 и её
сценарии (добавлен сценарий на совпадающие имена, всего 5), формулировка истории 5,
два граничных случая. Исправлена фактическая ошибка: безымянных работ восемь из
девяти, а не четыре.

FR-012 закрыт решением Р-1 в `research.md`: безымянная работа приводит к отказу.
Выражение её средствами языка отвергнуто как ломающее Р4 — имя работы и есть её
идентификатор. Открытых вопросов к автору нет.

- Items marked incomplete require spec updates before `/speckit-clarify` or `/speckit-plan`

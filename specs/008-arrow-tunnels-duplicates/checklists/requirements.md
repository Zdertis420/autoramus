# Specification Quality Checklist: Туннели и дубли стрелок

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-13
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

- **Пересмотрено после `/speckit-plan`.** Разведка поправила две формулировки, и
  обе правки внесены в спецификацию с указанием причины: FR-001 (гранулярность
  оверрайда — пара «поток + диаграмма», а не сегмент: посегментность ломает
  инвариант фичи 007) и FR-012 (`TUNNEL_SOFT` остаётся нулём: `0` стоит во всех
  файлах, записанных самим Ramus). Расхождения между spec.md, plan.md и
  research.md на сегодня нет.

- Раздел «Что измерено» намеренно называет имена классов Ramus и одну строку
  `layout/arrow.go`. Это не проектное решение, а **улика**: без неё утверждение
  «туннель вычисляется, а не читается» пришлось бы принимать на веру, а
  конституция (принцип V) требует сверки с исходниками Ramus. Требования FR-001…
  FR-012 и критерии SC-001…SC-005 от этих имён не зависят и сформулированы через
  наблюдаемое поведение.
- SC-001…SC-003 и SC-005 несут текущие измеренные значения (6 туннелей, 3 сегмента
  вместо 5, два сектора вместо одного). Это сделано намеренно: критерий без
  отправной точки нечем проверить.
- Открытый вопрос, сознательно решённый допущением, а не маркером: считать ли
  дубль ICOM + `links` ошибкой или предупреждением. Записано в «Assumptions» с
  условием пересмотра.
- Фича не закрывается без открытия хотя бы одного собранного файла в самом Ramus:
  всё измерение сделано вычислением по таблицам, без запуска GUI.

# Specification Quality Checklist: Декомпиляция без молчаливых потерь

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-08
**Updated**: 2026-09-08 — итерация 2, оба вопроса сняты ответами автора
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

Итерация 2 — пройдено всё.

Итерация 1 не прошла по одному пункту: два маркера [NEEDS CLARIFICATION]. Оба
сняты ответами автора:

| Вопрос | Решение | Куда легло |
|---|---|---|
| Охват люка | расширить на все элементы: модель, стрелку, классификатор, колонку | FR-016, US2, SC-003a |
| Реакция на незакрываемую потерю | предупреждение, код 0, вывод печатается | FR-017, FR-018, FR-019 |

Приведены в соответствие: повествование US2 и его приоритет (утверждали, что язык
расширять не придётся), граничный случай про отсутствие люка, строка вводной
таблицы.

**Следствие, которое стоит держать в голове при планировании.** Расширение люка —
изменение входного языка, а не деталь реализации: затрагивает схему, IR,
валидатор и печать. По соглашениям проекта оно фиксируется отдельным решением в
`DESIGN-DECISIONS.md` наравне с Р1–Р16; это записано в допущениях спецификации.

**Следствие про FR-017.** После расширения люка потерь на трёх имеющихся моделях
не остаётся, и предупреждение на них не сработает ни разу. Правило нужно ради
будущих моделей, поэтому проверять его придётся на собранных вручную данных —
как и чтение значений справочников в фиче `001`.

- Items marked incomplete require spec updates before `/speckit-clarify` or `/speckit-plan`

# Specification Quality Checklist: Стрелка поворачивает внутри себя

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-18
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

- Items marked incomplete require spec updates before `/speckit-clarify` or `/speckit-plan`

### Итог проверки (итерация 1)

Разобранные замечания:

- **«Соединялись» — не проверка.** Слово из запроса само по себе меры не несёт:
  в смысле данных стрелки у нас уже соединены кросспоинтами, и это проверено
  тестами. Пришлось сначала выяснить, что именно видит глаз, и только потом
  писать требования. Мера найдена: доля поворотов, лежащих внутри ломаной.
- **Ложный след отброшен мерой.** Первое подозрение — поле `POINT_TYPE`, которое
  мы пишем `-1` всегда. Снято подсчётом: у Ramus в углах стоит то же `-1`
  (8 из 8, 11 из 11). В спецификацию это вынесено явно, чтобы гипотезу не
  проверяли заново на этапе плана.
- **Порог, а не абсолют.** Первая редакция требовала «ни одного поворота на
  стыке». Мера показала, что у самого Ramus их 4–17 на файл, то есть требование
  было невыполнимо и вдобавок неверно по смыслу: часть таких углов — настоящие
  ответвления. Заменено на долю с порогом «не ниже, чем у Ramus».
- **Охрана предыдущих фич.** Перестройка ветвления трогает код фич 007 и 009.
  Требования FR-006…FR-008 выписаны отдельными пунктами, чтобы «починили
  рисунок — потеряли разведение» не прошло незамеченным.
- **Реализация в тексте.** Из описания убраны имена функций и файлов; названия
  моделей оставлены — это данные измерения, а не устройство кода.

Маркеров [NEEDS CLARIFICATION] не осталось. Направление запроса однозначно, а
всё, что требовало догадки, закрыто измерением на пяти файлах и записано в
«Assumptions». Спецификация готова к `/speckit-plan`.

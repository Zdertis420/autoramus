# Specification Quality Checklist: Раскладка блоков — координаты без ручного `layout`

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-10
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

- Маркеров `[NEEDS CLARIFICATION]` нет. Единственная развилка — входят ли в
  «координаты и раскладку» стрелки — закрыта доводом по существу, а не догадкой:
  генератор стрелки не пишет, и вычислять маршруты значило бы производить то,
  что некому записать. Прочтение вынесено в допущения первым пунктом и правится
  одной строкой, если имелось в виду шире.
- Раздел «Что уже известно про раскладку Ramus» добавлен сверх шаблона: числа
  сняты с настоящих файлов, и вся фича на них опирается. Без него FR-002 читался
  бы как произвол.
- **Honest limit в FR-002 и SC-002**: формула восстановлена по модели с четырьмя
  подработами и подтверждена двумя точками во второй модели. Для другого числа
  блоков поведение Ramus не наблюдалось — это записано в допущениях как
  обобщение, а не как измерение.
- FR-003 (не накладываются, не выходят за лист) при большом числе работ вступит в
  спор с FR-002 (равный шаг по фиксированной диагонали): блоки начнут наезжать.
  Это отмечено в краевых случаях и должно быть разрешено на этапе плана, а не
  спрятано.
- Расхождение с `CLAUDE.md` («около 138×72», «ячейки») названо прямо: мера
  точнее описания, документ придётся привести в соответствие.

# Quickstart: как проверить фичу

Команды — от корня репозитория.

## 0. Сборка

```bash
cd ramusc && make build && cd ..     # даст ./ramusc/ramusc
```

## 1. До: мера

```bash
for f in examples/chakhokhbili.yaml examples/skirt.yaml; do
  ./ramusc/ramusc "$f" -o "/tmp/$(basename "$f" .yaml).rsf"
done
python3 specs/012-arrow-port-spacing/evidence/ports.py /tmp/chakhokhbili.rsf /tmp/skirt.rsf
```

Ожидается сегодня: на «чахохбили» 45 промежутков, из них 29 меньше 15,
минимум 3.9; на «юбке» 4 из 10 меньше 15. Все блоки 72 × 50.4.

Прогноз после — прототип:

```bash
python3 specs/012-arrow-port-spacing/evidence/ladder.py /tmp/chakhokhbili.rsf
```

## 2. После: промежутки

Те же команды, что в шаге 1, после реализации. Ожидается:

* ноль промежутков меньше 15 на обоих файлах (SC-001), минимум не меньше 15
  (SC-002);
* A-0 «чахохбили» — 120 × 195, «Тушение» — 72 × 120, «Подготовка томатной
  массы» — 90 × 50.4, «Финальная приправка» — 72 × 60;
* A-0 «юбки» — 90 × 50.4, остальные блоки «юбки» — 72 × 50.4.

## 3. После: документы без тесноты не изменились

```bash
git stash -q   # или соберите из main
for f in ramusc/testdata/documents/*.yaml; do
  ./ramusc/ramusc "$f" -o "/tmp/before-$(basename "$f" .yaml).rsf"; done
git stash pop -q && (cd ramusc && make build)
for f in ramusc/testdata/documents/*.yaml; do
  ./ramusc/ramusc "$f" -o "/tmp/after-$(basename "$f" .yaml).rsf"; done
for f in ramusc/testdata/documents/*.yaml; do n=$(basename "$f" .yaml)
  cmp -s "/tmp/before-$n.rsf" "/tmp/after-$n.rsf" && echo "= $n" || echo "≠ $n"; done
```

Ожидается `≠` только у `channels-feedback` (research И2); остальные семь — `=`
(SC-003).

## 4. Тесты

```bash
cd ramusc && make check
```

Новые проверки перечислены в `contracts/block-sizing.md` §6.

## 5. Пересборка примеров

```bash
./ramusc/ramusc examples/chakhokhbili.yaml -o examples/chakhokhbili.rsf
./ramusc/ramusc examples/skirt.yaml -o examples/skirt.rsf
```

## 6. Взгляд в Ramus

Открыть `examples/chakhokhbili.rsf`, диаграмма A0, блок «Тушение»: семь входов
видны порознь, и каждый выделяется мышью без захвата соседнего (SC-005).
Число 15 пробное: если окажется тесно или просторно, правится одна константа
`portGap` (FR-002).

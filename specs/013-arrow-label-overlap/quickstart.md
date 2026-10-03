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
python3 specs/013-arrow-label-overlap/evidence/labels.py /tmp/chakhokhbili.rsf /tmp/skirt.rsf
python3 specs/013-arrow-label-overlap/evidence/near.py /tmp/chakhokhbili.rsf
```

Ожидается сегодня: «чахохбили» — 59 подписей, на подпись 9, на блок 4, на
чужую линию 22, все на своей линии; «юбка» — 20, на подпись 5, на чужую линию 6.

## 2. Метрика шрифта (нужна Java, только для пересборки таблицы)

```bash
cd specs/013-arrow-label-overlap/evidence
javac Glyphs.java One.java && java -Djava.awt.headless=true Glyphs
```

Ожидается: ширина строки равна сумме ширин знаков на всех пробах; у Dialog и
Liberation Sans расходятся 14 знаков из 166, не больше чем на единицу.
Сверка с файлами Ramus — `One.java` по списку однострочных подписей: 13 из 13
`=`.

## 3. После: наложения

Те же команды, что в шаге 1. Ожидается:

* на подпись — 0 на обоих файлах (SC-001), на блок — 0 (SC-002);
* на чужую линию — меньше 22 и меньше 6 (SC-007);
* до своей линии — не больше 34 (SC-003).

Если где-то место не нашлось, `TestLabelsDoNotCollide` назовёт подпись по
имени — это названная граница (FR-012), а не тихая неудача.

## 4. Тесты

```bash
cd ramusc && make check
```

Новые проверки перечислены в `contracts/label-placement.md` §5.

## 5. Пересборка примеров

```bash
./ramusc/ramusc examples/chakhokhbili.yaml -o examples/chakhokhbili.rsf
./ramusc/ramusc examples/skirt.yaml -o examples/skirt.rsf
```

## 6. Взгляд в Ramus

Открыть `examples/chakhokhbili.rsf`, A0 и A-0: ни одного слова, разорванного
переносом (SC-004); каждая подпись без колебаний читается как подпись своей
стрелки (SC-005). Особое внимание — входам «Тушения» и нижней полосе
механизмов, где тесно больше всего.

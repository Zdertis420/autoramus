# Quickstart: как проверить фичу

Команды — от корня репозитория.

## 1. До: мера

```bash
cd ramusc && make build && cd ..
for f in ramusc/testdata/documents/*.yaml examples/*.yaml; do
  ./ramusc/ramusc "$f" -o "/tmp/before-$(basename "$f" .yaml).rsf" >/dev/null
done
python3 specs/015-remove-double-crossings/evidence/swap.py /tmp/before-*.rsf
python3 specs/015-remove-double-crossings/evidence/double.py /tmp/before-*.rsf
```

Ожидается: лишних пар две — «чахохбили» («Курица с луком» × «Томатная
масса») и `channels-pair` («первый» × «второй»); пересечений — по таблице
потолков в `contracts/lane-choice.md` §3.

## 2. После

Те же команды на `/tmp/after-*.rsf`. Ожидается: лишних пар ноль (SC-001);
пересечений не больше потолков, на «чахохбили» — 156 (SC-003); побайтово
изменились только «чахохбили», `channels-pair`, `channels-trunk` (SC-004).

## 3. Тесты

```bash
cd ramusc && make check
```

## 4. Взгляд в Ramus

```bash
./ramusc/ramusc examples/chakhokhbili.yaml -o examples/chakhokhbili.rsf
```

A0, вход в «Тушение»: «Курица с луком» и «Томатная масса» идут рядом, не
перекрещиваясь (SC-005).

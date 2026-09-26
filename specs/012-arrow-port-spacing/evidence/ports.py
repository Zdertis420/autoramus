"""Промежутки между стрелками на одной стороне блока и размеры блоков.

Запуск: python3 ports.py файл.rsf [...]
"""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict, Counter

SIDE = {'0': 'выход', '1': 'механизм', '2': 'вход', '3': 'управление'}


def t(z, p):
    r = ET.fromstring(z.read(p))
    f = {x.get('id'): x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]: (x.text or '') for x in row} for row in r.find('data')]


for path in sys.argv[1:]:
    z = zipfile.ZipFile(path)
    A = {r['ATTRIBUTE_NAME']: r['ATTRIBUTE_ID'] for r in t(z, 'data/attributes.xml')}
    pts = defaultdict(list)
    for r in t(z, 'data/IDEF0/attribute_sector_points.xml'):
        pts[r['ELEMENT_ID']].append(r)
    for k in pts:
        pts[k].sort(key=lambda r: int(r['POSITION']))
    rect = {r['ELEMENT_ID']: tuple(float(r[k]) for k in ('X', 'Y', 'WIDTH', 'HEIGHT'))
            for r in t(z, 'data/IDEF0/attribute_rectangles.xml')}

    # Точка крепления — крайняя точка сектора, чей конец прицеплен к блоку.
    ports = defaultdict(set)
    for r in t(z, 'data/IDEF0/attribute_sector_borders.xml'):
        e = r['ELEMENT_ID']
        if r['FUNCTION'] in ('', '-1') or e not in pts:
            continue
        start = r['ATTRIBUTE_ID'] == A['F_SECTOR_BORDER_START']
        p = pts[e][0] if start else pts[e][-1]
        side = r['FUNCTION_TYPE']
        v = float(p['Y_POSITION']) if side in ('0', '2') else float(p['X_POSITION'])
        ports[(r['FUNCTION'], side)].add(round(v, 3))

    gaps, crowd, tight = [], Counter(), {}
    for (f, s), vs in ports.items():
        if f not in rect:
            continue
        vs = sorted(vs)
        x, y, w, h = rect[f]
        g = [b - a for a, b in zip(vs, vs[1:])]
        gaps += g
        crowd[len(vs)] += 1
        if g:
            tight[(f, SIDE[s])] = (len(vs), round(min(g), 1), round(h if s in ('0', '2') else w, 1))
    gaps.sort()
    sizes = Counter((round(r[2], 1), round(r[3], 1)) for f, r in rect.items()
                    if any(k[0] == f for k in ports))

    print('==', path.split('/')[-1])
    print('   стрелок на стороне → сторон:', dict(sorted(crowd.items())))
    if gaps:
        print(f'   промежутков {len(gaps)}: мин {gaps[0]:.1f}, медиана {gaps[len(gaps) // 2]:.1f},'
              f' меньше 20: {sum(1 for g in gaps if g < 20 - 1e-9)}')
    print('   размеры блоков:', dict(sizes.most_common(6)))
    for k, v in sorted(tight.items(), key=lambda kv: kv[1][1])[:5]:
        print('     ', k, 'стрелок', v[0], 'мин. промежуток', v[1], 'длина стороны', v[2])

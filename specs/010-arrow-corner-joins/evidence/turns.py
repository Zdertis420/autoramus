"""Где происходит поворот стрелки: внутри сектора или на стыке двух секторов.

Ramus скругляет угол только внутри ломаной одного сектора. Поворот, попавший на
границу двух секторов, рисуется двумя отдельными линиями, сходящимися под
прямым углом.
"""
import zipfile, sys, xml.etree.ElementTree as ET
from collections import defaultdict

def table(z, name, sub='IDEF0'):
    root = ET.fromstring(z.read(f'data/{sub}/{name}.xml'))
    fields = {f.get('id'): f.get('name') for f in root.find('fields')}
    return [{fields[f.get('id')]: (f.text or '') for f in r} for r in root.find('data')]

def attrs(z):
    root = ET.fromstring(z.read('data/attributes.xml'))
    fields = {f.get('id'): f.get('name') for f in root.find('fields')}
    out = {}
    for r in root.find('data'):
        d = {fields[f.get('id')]: (f.text or '') for f in r}
        out[d.get('ATTRIBUTE_NAME','')] = d.get('ATTRIBUTE_ID','')
    return out

def run(path):
    z = zipfile.ZipFile(path)
    A = attrs(z)
    pts = defaultdict(list)
    for r in table(z, 'attribute_sector_points'):
        pts[r['ELEMENT_ID']].append(r)
    for k in pts: pts[k].sort(key=lambda r: int(r['POSITION']))

    diagram = {}
    for r in table(z, 'attribute_other_elements', 'Core'):
        if r['ATTRIBUTE_ID'] == A['F_FUNCTION_SECTOR']:
            diagram[r['ELEMENT_ID']] = r['OTHER_ELEMENT']

    start = A['F_SECTOR_BORDER_START']
    ends = defaultdict(dict)
    for r in table(z, 'attribute_sector_borders'):
        ends[r['ELEMENT_ID']][r['ATTRIBUTE_ID']] = r

    def xy(p): return (round(float(p['X_POSITION']),4), round(float(p['Y_POSITION']),4))
    def direction(a, b):
        if a[0] == b[0] and a[1] != b[1]: return 'V'
        if a[1] == b[1] and a[0] != b[0]: return 'H'
        return '?'

    inner = 0
    for eid, ps in pts.items():
        for i in range(1, len(ps)-1):
            a, b, c = xy(ps[i-1]), xy(ps[i]), xy(ps[i+1])
            if direction(a,b) != direction(b,c) and '?' not in (direction(a,b), direction(b,c)):
                inner += 1

    # стыки: группируем концы по (диаграмма, кросспоинт)
    joints = defaultdict(list)
    for eid, e in ends.items():
        for aid, r in e.items():
            cp = r.get('CROSSPOINT','')
            if not cp or cp == '-1': continue
            ps = pts.get(eid, [])
            if len(ps) < 2: continue
            isStart = (aid == start)
            p = xy(ps[0]) if isStart else xy(ps[-1])
            q = xy(ps[1]) if isStart else xy(ps[-2])
            joints[(diagram.get(eid,'?'), cp)].append((p, direction(p,q)))

    bend = 0
    for (dg, cp), items in joints.items():
        coords = {p for p,_ in items}
        if len(coords) != 1 or len(items) < 2: continue
        dirs = {d for _,d in items}
        if 'H' in dirs and 'V' in dirs:
            bend += 1

    print(f"=== {path.split('/')[-1]}")
    print(f"   поворотов ВНУТРИ сектора (Ramus скругляет):      {inner}")
    print(f"   поворотов НА СТЫКЕ секторов (скругления нет):    {bend}")

for p in sys.argv[1:]: run(p)

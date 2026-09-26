"""Прототип лестницы с равными промежутками вдоль диагонали Ramus под выросшие блоки.

Запуск: python3 ladder.py собранный.rsf [...]
Берёт число стрелок на сторонах из файла, растит блоки по правилу (n+1)·15 и
печатает промежутки лестницы. Порядок блоков — по x в файле.
"""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict
S=15.0
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
for path in sys.argv[1:]:
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    rect={r['ELEMENT_ID']:tuple(float(r[k]) for k in ('X','Y','WIDTH','HEIGHT')) for r in t(z,'data/IDEF0/attribute_rectangles.xml')}
    parent={r['ELEMENT_ID']:r['PARENT_ELEMENT_ID'] for r in t(z,'data/Core/attribute_hierarchicals.xml')}
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    ports=defaultdict(set)
    for r in t(z,'data/IDEF0/attribute_sector_borders.xml'):
        e=r['ELEMENT_ID']
        if r['FUNCTION'] in ('','-1') or e not in pts: continue
        ps=sorted(pts[e],key=lambda p:int(p['POSITION']))
        p=ps[0] if r['ATTRIBUTE_ID']==A['F_SECTOR_BORDER_START'] else ps[-1]
        ports[(r['FUNCTION'],r['FUNCTION_TYPE'])].add((p['X_POSITION'],p['Y_POSITION']))
    kids=defaultdict(list)
    for f in rect: kids[parent.get(f)].append(f)
    name=path.split('/')[-1]
    for p,fs in kids.items():
        n=len(fs)
        if n<2: 
            f=fs[0]; h=max(50.4,(max(len(ports[(f,'0')]),len(ports[(f,'2')]))+1)*S); w=max(72,(max(len(ports[(f,'1')]),len(ports[(f,'3')]))+1)*S)
            print(f'{name:22} диаграмма {p}: один блок {w:.0f}×{h:.1f}')
            continue
        fs.sort(key=lambda f:rect[f][0])
        W=[];H=[]
        for f in fs:
            H.append(max(50.4,(max(len(ports[(f,'0')]),len(ports[(f,'2')]))+1)*S))
            W.append(max(72.0,(max(len(ports[(f,'1')]),len(ports[(f,'3')]))+1)*S))
        gx=(660+72-80-sum(W))/(n-1); gy=(322+50.4-80-sum(H))/(n-1)
        grown=sum(1 for w,h in zip(W,H) if w>72 or h>50.4)
        bottom=80+sum(H)+(n-1)*gy
        print(f'{name:22} диаграмма {p}: блоков {n}, выросло {grown}, W={[round(w) for w in W]} H={[round(h) for h in H]} '
              f'промежуток по x {gx:.1f}, по y {gy:.1f}, низ {bottom:.1f}')

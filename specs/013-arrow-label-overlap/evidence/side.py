"""Где подпись Ramus стоит относительно ближайшего отрезка своей линии."""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict, Counter
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
for path in sys.argv[1:]:
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    for k in pts: pts[k].sort(key=lambda r:int(r['POSITION']))
    out=Counter(); offs=[]
    for r in t(z,'data/IDEF0/attribute_sector_properties.xml'):
        if r['SHOW_TEXT']!='1': continue
        x,y,w,h=(float(r[k]) for k in ('TEXT_X','TEXT_Y','TEXT_WIDTH','TEXT_HIEGHT'))
        ps=[(float(p['X_POSITION']),float(p['Y_POSITION'])) for p in pts[r['ELEMENT_ID']]]
        best=None
        for a,b in zip(ps,ps[1:]):
            if a[1]==b[1]:  # горизонталь
                x1,x2=sorted((a[0],b[0]))
                if x2<x or x1>x+w: continue
                d = a[1]-(y+h) if y+h<=a[1] else (y-a[1] if y>=a[1] else 0)
                side='над' if y+h<=a[1]+1e-6 else ('под' if y>=a[1]-1e-6 else 'на')
                k=(abs(d),side,round(d,1))
            else:
                y1,y2=sorted((a[1],b[1]))
                if y2<y or y1>y+h: continue
                d = a[0]-(x+w) if x+w<=a[0] else (x-a[0] if x>=a[0] else 0)
                side='слева' if x+w<=a[0]+1e-6 else ('справа' if x>=a[0]-1e-6 else 'на')
                k=(abs(d),side,round(d,1))
            if best is None or k<best: best=k
        if best: out[best[1]]+=1; offs.append((best[1],best[2]))
    print(path.split('/')[-1], dict(out), sorted(o for s,o in offs if s!='на')[:12])

"""Пересечения отрезков на самой населённой диаграмме — прямо по .rsf."""
import zipfile, sys, xml.etree.ElementTree as ET
from collections import defaultdict
EPS=1e-9
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
def run(path):
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    for k in pts: pts[k].sort(key=lambda r:int(r['POSITION']))
    diag={}
    for r in t(z,'data/Core/attribute_other_elements.xml'):
        if r['ATTRIBUTE_ID']==A['F_FUNCTION_SECTOR']: diag[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
    by=defaultdict(list)
    for eid,ps in pts.items():
        co=[(float(p['X_POSITION']),float(p['Y_POSITION'])) for p in ps]
        for (x1,y1),(x2,y2) in zip(co,co[1:]):
            if abs(y1-y2)<EPS and abs(x1-x2)>EPS: by[diag.get(eid,'?')].append(('H',y1,min(x1,x2),max(x1,x2)))
            elif abs(x1-x2)<EPS and abs(y1-y2)>EPS: by[diag.get(eid,'?')].append(('V',x1,min(y1,y2),max(y1,y2)))
    best=max(by.items(), key=lambda kv: len(kv[1]))
    d,ps=best
    cross=sum(1 for o1,c1,l1,h1 in ps for o2,c2,l2,h2 in ps
              if o1=='H' and o2=='V' and l1-EPS<c2<h1+EPS and l2-EPS<c1<h2+EPS)
    ln=sum(h-l for _,_,l,h in ps)
    print(f"{path.split('/')[-1]:26} диаграмма {d:>4}: отрезков {len(ps):3}  пересечений {cross:4}"
          f"  на отрезок {cross/len(ps):5.2f}  длина {ln:6.0f}")
for p in sys.argv[1:]: run(p)

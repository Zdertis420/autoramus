"""Пары разных стрелок одной диаграммы, пересекающие друг друга дважды и больше."""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
EPS=1e-6
def cross(p,q):
    (a,b),(c,d)=p,q
    ah=abs(a[1]-b[1])<EPS; ch=abs(c[1]-d[1])<EPS
    if ah==ch: return None
    if not ah: (a,b),(c,d)=(c,d),(a,b)
    x1,x2=sorted((a[0],b[0])); y1,y2=sorted((c[1],d[1]))
    if x1+EPS<c[0]<x2-EPS and y1+EPS<a[1]<y2-EPS: return (round(c[0],1),round(a[1],1))
    return None
for path in sys.argv[1:]:
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    diag={};stream={}
    for r in t(z,'data/Core/attribute_other_elements.xml'):
        if r['ATTRIBUTE_ID']==A['F_FUNCTION_SECTOR']: diag[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
        if r['ATTRIBUTE_ID']==A['F_SECTOR_STREAM']: stream[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
    name={r['ELEMENT_ID']:r['VALUE'] for r in t(z,'data/Core/attribute_texts.xml') if r['ATTRIBUTE_ID']==A.get('F_STREAM_NAME')}
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    arrows=defaultdict(list)
    for e,ps in pts.items():
        ps.sort(key=lambda r:int(r['POSITION']))
        key=(diag.get(e), stream.get(e) or 'сектор'+e)
        for a,b in zip(ps,ps[1:]):
            arrows[key].append(((float(a['X_POSITION']),float(a['Y_POSITION'])),(float(b['X_POSITION']),float(b['Y_POSITION']))))
    keys=sorted(arrows); total=0; pairs=[]
    for i,k in enumerate(keys):
        for l in keys[i+1:]:
            if k[0]!=l[0]: continue
            xs={c for p in arrows[k] for q in arrows[l] if (c:=cross(p,q))}
            total+=len(xs)
            if len(xs)>=2: pairs.append((name.get(k[1],k[1]),name.get(l[1],l[1]),len(xs)))
    print(f"{path.split('/')[-1]:28} пересечений {total:4}, пар с двумя и более: {len(pairs)}")
    for p in pairs: print('     ',p)

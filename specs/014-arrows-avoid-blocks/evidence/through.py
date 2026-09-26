"""Отрезки стрелок, проходящие сквозь блоки своей диаграммы."""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
EPS=1e-6
for path in sys.argv[1:]:
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    diag={};stream={}
    for r in t(z,'data/Core/attribute_other_elements.xml'):
        if r['ATTRIBUTE_ID']==A['F_FUNCTION_SECTOR']: diag[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
        if r['ATTRIBUTE_ID']==A['F_SECTOR_STREAM']: stream[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
    sname={r['ELEMENT_ID']:r['VALUE'] for r in t(z,'data/Core/attribute_texts.xml') if r['ATTRIBUTE_ID']==A.get('F_STREAM_NAME')}
    parent={r['ELEMENT_ID']:r['PARENT_ELEMENT_ID'] for r in t(z,'data/Core/attribute_hierarchicals.xml')}
    fname={r['ELEMENT_ID']:r['VALUE'] for r in t(z,'data/Core/attribute_texts.xml')}
    rect={r['ELEMENT_ID']:tuple(float(r[k]) for k in ('X','Y','WIDTH','HEIGHT')) for r in t(z,'data/IDEF0/attribute_rectangles.xml')}
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    hits=[];segs=0;arrows=set()
    for e,ps in pts.items():
        ps.sort(key=lambda r:int(r['POSITION']))
        d=diag.get(e)
        blocks=[(f,rect[f]) for f in rect if parent.get(f)==d]
        for a,b in zip(ps,ps[1:]):
            segs+=1
            x1,x2=sorted((float(a['X_POSITION']),float(b['X_POSITION']))); y1,y2=sorted((float(a['Y_POSITION']),float(b['Y_POSITION'])))
            for f,(x,y,w,h) in blocks:
                if x2>x+EPS and x1<x+w-EPS and y2>y+EPS and y1<y+h-EPS:
                    hits.append((sname.get(stream.get(e),'?'), fname.get(f,f), round(x1,1),round(y1,1),round(x2,1),round(y2,1)))
                    arrows.add((e,f))
    print(f"{path.split('/')[-1]:26} отрезков {segs:4}, сквозь блок {len(hits):3} (секторов×блоков {len(arrows)})")
    for h in hits[:12]: print('    ',h)

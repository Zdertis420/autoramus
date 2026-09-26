"""Как далеко подпись от своей линии и лежит ли она на ней."""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict
sys.path.insert(0,'/tmp/f013')
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
def dist(r,p,q):
    # расстояние от прямоугольника до отрезка (оси параллельны)
    x1,x2=sorted((p[0],q[0])); y1,y2=sorted((p[1],q[1]))
    dx=max(0,max(r[0]-x2, x1-(r[0]+r[2]))); dy=max(0,max(r[1]-y2, y1-(r[1]+r[3])))
    return (dx*dx+dy*dy)**.5
for path in sys.argv[1:]:
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    diag={};stream={}
    for r in t(z,'data/Core/attribute_other_elements.xml'):
        if r['ATTRIBUTE_ID']==A['F_FUNCTION_SECTOR']: diag[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
        if r['ATTRIBUTE_ID']==A['F_SECTOR_STREAM']: stream[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    for k in pts: pts[k].sort(key=lambda r:int(r['POSITION']))
    own=defaultdict(list)
    for e,ps in pts.items():
        for a,b in zip(ps,ps[1:]):
            own[(diag.get(e),stream.get(e))].append(((float(a['X_POSITION']),float(a['Y_POSITION'])),(float(b['X_POSITION']),float(b['Y_POSITION']))))
    ds=[];on=0;tilda=0
    for r in t(z,'data/IDEF0/attribute_sector_properties.xml'):
        if r['SHOW_TEXT']!='1': continue
        if r['SHOW_TILDA']=='1': tilda+=1
        e=r['ELEMENT_ID']; rc=tuple(float(r[k]) for k in ('TEXT_X','TEXT_Y','TEXT_WIDTH','TEXT_HIEGHT'))
        d=min((dist(rc,p,q) for p,q in own[(diag.get(e),stream.get(e))]),default=None)
        if d is None: continue
        ds.append(d); on+= d==0
    ds.sort()
    print(f"{path.split('/')[-1]:26} подписей {len(ds)}: лежат на своей линии {on}, расстояние до своей линии мин {ds[0]:.1f} медиана {ds[len(ds)//2]:.1f} макс {ds[-1]:.1f}; с тильдой {tilda}")

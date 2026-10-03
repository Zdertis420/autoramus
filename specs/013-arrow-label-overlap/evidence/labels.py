"""Подписи стрелок: наложения друг на друга, на блоки и на линии чужих стрелок."""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
def inter(a,b):
    w=min(a[0]+a[2],b[0]+b[2])-max(a[0],b[0]); h=min(a[1]+a[3],b[1]+b[3])-max(a[1],b[1])
    return w>1e-6 and h>1e-6
def seg_hits(r,p,q):
    x1,x2=sorted((p[0],q[0])); y1,y2=sorted((p[1],q[1]))
    return x2>r[0] and x1<r[0]+r[2] and y2>r[1] and y1<r[1]+r[3]
for path in sys.argv[1:]:
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    diag={};stream={}
    for r in t(z,'data/Core/attribute_other_elements.xml'):
        if r['ATTRIBUTE_ID']==A['F_FUNCTION_SECTOR']: diag[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
        if r['ATTRIBUTE_ID']==A['F_SECTOR_STREAM']: stream[r['ELEMENT_ID']]=r['OTHER_ELEMENT']
    name={}
    for r in t(z,'data/Core/attribute_texts.xml'):
        if r['ATTRIBUTE_ID']==A.get('F_STREAM_NAME'): name[r['ELEMENT_ID']]=r['VALUE']
    parent={r['ELEMENT_ID']:r['PARENT_ELEMENT_ID'] for r in t(z,'data/Core/attribute_hierarchicals.xml')}
    rect={r['ELEMENT_ID']:tuple(float(r[k]) for k in ('X','Y','WIDTH','HEIGHT')) for r in t(z,'data/IDEF0/attribute_rectangles.xml')}
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    for k in pts: pts[k].sort(key=lambda r:int(r['POSITION']))
    labels=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_properties.xml'):
        if r['SHOW_TEXT']!='1': continue
        e=r['ELEMENT_ID']
        labels[diag.get(e)].append((e, name.get(stream.get(e),'?'), tuple(float(r[k]) for k in ('TEXT_X','TEXT_Y','TEXT_WIDTH','TEXT_HIEGHT'))))
    total=ll=lb=lx=0; lines=defaultdict(int); rune=[]
    for d,ls in labels.items():
        blocks=[rect[f] for f in rect if parent.get(f)==d]
        segs=[(e,(float(a['X_POSITION']),float(a['Y_POSITION'])),(float(b['X_POSITION']),float(b['Y_POSITION']))) for e,ps in pts.items() if diag.get(e)==d for a,b in zip(ps,ps[1:])]
        for i,(e,n,r) in enumerate(ls):
            total+=1
            lines[round(r[3]/9.80078125,2)]+=1
            if r[3] and abs(r[3]-9.80078125)<0.01 and n!='?': rune.append(r[2]/len(n))
            if any(inter(r,o[2]) for o in ls[i+1:]): ll+=1
            if any(inter(r,b) for b in blocks): lb+=1
            if any(seg_hits(r,p,q) for s,p,q in segs if stream.get(s)!=stream.get(e)): lx+=1
    rn=sorted(rune)
    print(f"{path.split('/')[-1]:28} подписей {total:3}: налезают на подпись {ll:3}, на блок {lb:3}, на чужую линию {lx:3} | строк: {dict(sorted(lines.items()))} | ширина на руну в одну строку: {rn[0]:.2f}…{rn[-1]:.2f}" if rn else f"{path} {total}")

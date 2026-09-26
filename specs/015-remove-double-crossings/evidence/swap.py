"""Какие двойные пересечения лишние: убираются обменом соседних параллельных отрезков двух стрелок."""
import sys, zipfile, xml.etree.ElementTree as ET
from collections import defaultdict
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
EPS=1e-6; NEAR=12.0
def cross(p,q):
    (a,b),(c,d)=p,q
    ah=abs(a[1]-b[1])<EPS; ch=abs(c[1]-d[1])<EPS
    if ah==ch: return None
    if not ah: (a,b),(c,d)=(c,d),(a,b)
    x1,x2=sorted((a[0],b[0])); y1,y2=sorted((c[1],d[1]))
    if x1+EPS<c[0]<x2-EPS and y1+EPS<a[1]<y2-EPS: return (round(c[0],2),round(a[1],2))
    return None
def count(P,Q):
    return len({c for pl in P for qm in Q for p in zip(pl,pl[1:]) for q in zip(qm,qm[1:]) if (c:=cross(p,q))})
def move(poly,i,axis,val):
    """Сдвинуть отрезок i..i+1 ломаной поперёк: вертикаль по x, горизонталь по y."""
    poly=[list(p) for p in poly]
    poly[i][axis]=val; poly[i+1][axis]=val
    return [tuple(p) for p in poly]
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
    polys=defaultdict(list)
    for e,ps in pts.items():
        ps.sort(key=lambda r:int(r['POSITION']))
        polys[(diag.get(e), stream.get(e) or 'сектор'+e)].append([(float(p['X_POSITION']),float(p['Y_POSITION'])) for p in ps])
    keys=sorted(polys); dbl=0; fixable=[]
    for i,k in enumerate(keys):
        for l in keys[i+1:]:
            if k[0]!=l[0]: continue
            P,Q=polys[k],polys[l]; n=count(P,Q)
            if n<2: continue
            dbl+=1; best=n
            # внутренние отрезки (не первый и не последний — те прицеплены к блоку или краю)
            for pi,pl in enumerate(P):
                for a in range(1,len(pl)-2):
                    for qi,qm in enumerate(Q):
                        for b in range(1,len(qm)-2):
                            p1,p2=pl[a],pl[a+1]; q1,q2=qm[b],qm[b+1]
                            for axis in (0,1):
                                o=1-axis
                                if abs(p1[axis]-p2[axis])>EPS or abs(q1[axis]-q2[axis])>EPS: continue
                                if abs(p1[axis]-q1[axis])>NEAR: continue
                                lo=max(min(p1[o],p2[o]),min(q1[o],q2[o])); hi=min(max(p1[o],p2[o]),max(q1[o],q2[o]))
                                if hi<=lo: continue
                                P2=list(P); Q2=list(Q)
                                P2[pi]=move(pl,a,axis,q1[axis]); Q2[qi]=move(qm,b,axis,p1[axis])
                                best=min(best,count(P2,Q2))
            if best<n: fixable.append((name.get(k[1],k[1]),name.get(l[1],l[1]),n,best))
    print(f"{path.split('/')[-1]:28} пар с двумя и более: {dbl:3}, из них лишних (обмен убирает): {len(fixable)}")
    for f in fixable: print('     ',f)

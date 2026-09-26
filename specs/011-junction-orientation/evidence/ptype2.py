import zipfile, sys, xml.etree.ElementTree as ET
from collections import defaultdict
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
def run(path,limit=14):
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    START,END=A['F_SECTOR_BORDER_START'],A['F_SECTOR_BORDER_END']
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    for k in pts: pts[k].sort(key=lambda r:int(r['POSITION']))
    at=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_borders.xml'):
        cp=r.get('CROSSPOINT','')
        if cp in ('','-1'): continue
        eid=r['ELEMENT_ID']; ps=pts.get(eid,[])
        if len(ps)<2: continue
        start = r['ATTRIBUTE_ID']==START
        i,j = (0,1) if start else (len(ps)-1,len(ps)-2)
        co=lambda k:(float(ps[k]['X_POSITION']),float(ps[k]['Y_POSITION']))
        a,b=co(i),co(j)
        orient='V' if abs(a[0]-b[0])<1e-9 else ('H' if abs(a[1]-b[1])<1e-9 else '?')
        at[cp].append((eid,'нач' if start else 'кон',orient,ps[i]['POINT_TYPE'],a))
    print(f"\n=== {path.split('/')[-1]}")
    shown=0
    for cp,items in sorted(at.items(), key=lambda kv:int(kv[0])):
        if len(items)<2: continue
        coords={i[4] for i in items}
        if len(coords)>1: continue   # сшивка уровней, не узел на одной диаграмме
        shown+=1
        if shown>limit: break
        desc=', '.join(f"{e}:{r}/{o}/PT={p}" for e,r,o,p,_ in items)
        print(f"  узел {cp:>3} @{coords.pop()}: {desc}")
run(sys.argv[1])

"""Что означает POINT_TYPE: где он не -1 и с чем связан."""
import zipfile, sys, xml.etree.ElementTree as ET
from collections import defaultdict, Counter
def t(z,p):
    r=ET.fromstring(z.read(p)); f={x.get('id'):x.get('name') for x in r.find('fields')}
    return [{f[x.get('id')]:(x.text or '') for x in row} for row in r.find('data')]
def run(path):
    z=zipfile.ZipFile(path)
    A={r['ATTRIBUTE_NAME']:r['ATTRIBUTE_ID'] for r in t(z,'data/attributes.xml')}
    START,END=A['F_SECTOR_BORDER_START'],A['F_SECTOR_BORDER_END']
    pts=defaultdict(list)
    for r in t(z,'data/IDEF0/attribute_sector_points.xml'): pts[r['ELEMENT_ID']].append(r)
    for k in pts: pts[k].sort(key=lambda r:int(r['POSITION']))
    border=defaultdict(dict)
    for r in t(z,'data/IDEF0/attribute_sector_borders.xml'): border[r['ELEMENT_ID']][r['ATTRIBUTE_ID']]=r
    tally=Counter()
    for eid,ps in pts.items():
        co=[(float(p['X_POSITION']),float(p['Y_POSITION'])) for p in ps]
        for i,p in enumerate(ps):
            pt=p['POINT_TYPE']
            if i==0: role,nb,other=('начало',border[eid].get(START),co[1] if len(co)>1 else None)
            elif i==len(ps)-1: role,nb,other=('конец',border[eid].get(END),co[-2])
            else: role,nb,other=('середина',None,None)
            if other is None: orient='?'
            else:
                orient = 'V' if abs(co[i][0]-other[0])<1e-9 else ('H' if abs(co[i][1]-other[1])<1e-9 else '?')
            if nb is None: kind='—'
            else:
                cp=nb.get('CROSSPOINT','')
                fn=nb.get('FUNCTION','-1')
                bt=nb.get('BORDER_TYPE','-1')
                # Узел — только конец, не прицепленный ни к работе, ни к краю
                # листа. Конец на краю тоже носит номер кросспоинта (так сшиты
                # уровни), и если считать по одному номеру, узлов выйдет вдвое
                # больше, чем есть.
                if fn not in ('','-1'): kind='блок'
                elif bt not in ('','-1'): kind='край'
                elif cp not in ('','-1'): kind='узел'
                else: kind='—'
            tally[(role,kind,orient,pt)]+=1
    print(f"\n=== {path.split('/')[-1]}")
    print(f"{'где':9} {'конец':6} {'ход':4} POINT_TYPE  штук")
    for k in sorted(tally, key=lambda k:(k[0],k[1],k[2],k[3])):
        print(f"  {k[0]:8} {k[1]:6} {k[2]:3}  {k[3]:>6}  {tally[k]}")
for p in sys.argv[1:]: run(p)

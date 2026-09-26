"""Теснота диаграммы: пересечения, длина линий, забитость полей."""
import json, sys
from collections import defaultdict
EPS=1e-9

def pieces(d, diagram):
    out=[]
    for a in d['layout']['arrows']:
        for s in a['segments']:
            if s.get('context') or s.get('on')!=diagram: continue
            pts=s['points']
            for (x1,y1),(x2,y2) in zip(pts,pts[1:]):
                if abs(y1-y2)<EPS and abs(x1-x2)>EPS: out.append((a['flow'],'H',y1,min(x1,x2),max(x1,x2)))
                elif abs(x1-x2)<EPS and abs(y1-y2)>EPS: out.append((a['flow'],'V',x1,min(y1,y2),max(y1,y2)))
    return out

def run(path, diagram, label):
    d=json.load(open(path))
    ps=pieces(d,diagram)
    par={f['name']:f.get('of','') for f in d.get('functions',[])}
    boxes=[(b['x'],b['y'],b['width'],b['height']) for b in d['layout']['functions'] if par.get(b['function'])==diagram]
    top=min(b[1] for b in boxes); bot=max(b[1]+b[3] for b in boxes)
    left=min(b[0] for b in boxes); right=max(b[0]+b[2] for b in boxes)

    cross=0
    for f1,o1,c1,l1,h1 in ps:
        for f2,o2,c2,l2,h2 in ps:
            if o1!='H' or o2!='V': continue
            if l1-EPS<c2<h1+EPS and l2-EPS<c1<h2+EPS: cross+=1
    length=sum(h-l for _,_,_,l,h in ps)
    # линии в полях (вне полосы блоков)
    below=[p for p in ps if p[1]=='H' and p[2]>bot]
    above=[p for p in ps if p[1]=='H' and p[2]<top]
    print(f"{label:22} отрезков {len(ps):3}  пересечений {cross:4}  длина {length:7.0f}"
          f"  горизонталей ниже блоков {len(set(p[2] for p in below)):2}  выше {len(set(p[2] for p in above)):2}")
    return len(below), bot

S='/tmp/claude-1000/-home-Zdertis-FastStorage-Programming-auto-ramus/f81be135-80a5-4eb6-b48d-9880dafd4e66/scratchpad'
run(f'{S}/after/chakhokhbili.json','Приготовление чахохбили','до фичи 010')
run(f'{S}/new/chakhokhbili.json','Приготовление чахохбили','после фичи 010')
run(f'{S}/yubka.json','Изготовление юбки','Ramus «Юбка»')

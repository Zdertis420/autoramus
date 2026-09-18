"""Считает сегменты, проходящие сквозь блок (строго внутрь, касание не в счёт)."""
import json, sys
from collections import defaultdict
EPS = 1e-9

def run(path):
    d = json.load(open(path))
    parent = {f['name']: f.get('of','') for f in d.get('functions',[])}
    boxes = defaultdict(list)
    for b in d.get('layout',{}).get('functions',[]):
        boxes[parent.get(b['function'],'')].append(
            (b['function'], b['x'], b['y'], b['width'], b['height']))
    hits = []
    for a in d.get('layout',{}).get('arrows',[]):
        for s in a.get('segments',[]):
            diag = s.get('on') if not s.get('context') else None
            if diag is None: continue
            pts = s.get('points',[])
            for (x1,y1),(x2,y2) in zip(pts, pts[1:]):
                lo_x, hi_x = min(x1,x2), max(x1,x2)
                lo_y, hi_y = min(y1,y2), max(y1,y2)
                for name,bx,by,bw,bh in boxes.get(diag,[]):
                    if hi_x > bx+EPS and lo_x < bx+bw-EPS and hi_y > by+EPS and lo_y < by+bh-EPS:
                        # свой блок на конце сегмента — не в счёт
                        ends = {s.get('from',{}).get('function'), s.get('to',{}).get('function')}
                        hits.append((diag, a['flow'], name, name in ends,
                                     [round(v,1) for v in (x1,y1,x2,y2)]))
    return hits

for p in sys.argv[1:]:
    hits = run(p)
    own = sum(1 for h in hits if h[3])
    print(f"\n=== {p.split('/')[-1]}: сквозь блок {len(hits)} (из них через свой же блок {own})")
    for h in hits[:15]:
        mark = ' (свой)' if h[3] else ''
        print(f"  диаграмма «{h[0]}»: поток «{h[1]}» сквозь «{h[2]}»{mark}  {h[4]}")

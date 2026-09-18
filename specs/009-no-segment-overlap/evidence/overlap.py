"""Считает наложения сегментов в декомпилированной модели.

Наложение: два отрезка одной ориентации, лежащие на одной координате, чья общая
часть имеет ненулевую длину. Сегменты одной стрелки (один поток на одной
диаграмме) не считаются: их общая магистраль намеренна.
"""
import json, sys
from collections import defaultdict

EPS = 1e-9

def segs(doc):
    """(diagram, flow, orientation, coord, lo, hi) по всем ломаным."""
    out = []
    for arrow in doc.get('layout', {}).get('arrows', []):
        flow = arrow.get('flow', '')
        for s in arrow.get('segments', []):
            diag = ('CTX:' if s.get('context') else 'DEC:') + str(s.get('on'))
            pts = s.get('points', [])
            for (x1, y1), (x2, y2) in zip(pts, pts[1:]):
                if abs(y1 - y2) < EPS and abs(x1 - x2) > EPS:
                    out.append((diag, flow, 'H', y1, min(x1, x2), max(x1, x2)))
                elif abs(x1 - x2) < EPS and abs(y1 - y2) > EPS:
                    out.append((diag, flow, 'V', x1, min(y1, y2), max(y1, y2)))
    return out

def overlaps(doc):
    bucket = defaultdict(list)
    for d, f, o, c, lo, hi in segs(doc):
        bucket[(d, o, round(c, 6))].append((f, lo, hi))
    bad = []
    for (d, o, c), items in bucket.items():
        for i in range(len(items)):
            for j in range(i + 1, len(items)):
                f1, lo1, hi1 = items[i]
                f2, lo2, hi2 = items[j]
                if f1 == f2:
                    continue  # одна стрелка: магистраль общая намеренно
                lo, hi = max(lo1, lo2), min(hi1, hi2)
                if hi - lo > EPS:
                    bad.append((d, o, c, f1, f2, round(hi - lo, 2)))
    return bad

def on_block_side(doc):
    """Сегмент, лежащий на стороне чужого блока."""
    parent = {}
    for f in doc.get('functions', []):
        parent[f['name']] = f.get('of', '')
    boxes = defaultdict(list)
    for b in doc.get('layout', {}).get('functions', []):
        n = b['function']
        boxes[parent.get(n, '')].append((n, b['x'], b['y'], b['width'], b['height']))
    hits = []
    for d, f, o, c, lo, hi in segs(doc):
        diag = d.split(':', 1)[1]
        for name, x, y, w, h in boxes.get(diag, []):
            if o == 'V' and (abs(c - x) < EPS or abs(c - (x + w)) < EPS):
                if min(hi, y + h) - max(lo, y) > EPS:
                    hits.append((diag, f, 'V', name))
            if o == 'H' and (abs(c - y) < EPS or abs(c - (y + h)) < EPS):
                if min(hi, x + w) - max(lo, x) > EPS:
                    hits.append((diag, f, 'H', name))
    return hits

for path in sys.argv[1:]:
    doc = json.load(open(path))
    bad = overlaps(doc)
    sides = on_block_side(doc)
    print(f"\n=== {path.split('/')[-1]}: сегментов {len(segs(doc))}, наложений {len(bad)}, на стороне блока {len(sides)}")
    for d, o, c, f1, f2, l in sorted(bad)[:12]:
        print(f"  {d:38} {o} {c:>8.2f}  «{f1}» ∩ «{f2}»  длина {l}")
    for h in sorted(set(sides))[:8]:
        print(f"  сторона блока: диаграмма {h[0]}, поток «{h[1]}», {h[2]}, блок «{h[3]}»")

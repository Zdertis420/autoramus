import json, sys
from PIL import Image, ImageDraw

path, diagram, out = sys.argv[1], sys.argv[2], sys.argv[3]
d = json.load(open(path))
K = 2  # масштаб
W, H = 800*K, 450*K
img = Image.new('RGB', (W, H), 'white')
g = ImageDraw.Draw(img)

par = {f['name']: f.get('of','') for f in d.get('functions',[])}
for b in d['layout']['functions']:
    if par.get(b['function']) != diagram: continue
    x,y,w,h = b['x']*K, b['y']*K, b['width']*K, b['height']*K
    g.rectangle([x,y,x+w,y+h], outline='black', width=2)
    g.text((x+4,y+4), b['function'][:18], fill='black')

n = 0
for a in d['layout']['arrows']:
    for s in a['segments']:
        if s.get('context') or s.get('on') != diagram: continue
        pts = [(p[0]*K, p[1]*K) for p in s['points']]
        g.line(pts, fill='black', width=2)
        for p in pts:
            g.ellipse([p[0]-3,p[1]-3,p[0]+3,p[1]+3], outline='red')
        n += 1
img.save(out)
print(f"{out}: сегментов {n}")

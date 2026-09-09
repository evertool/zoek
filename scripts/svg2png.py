#!/usr/bin/env python3
"""Convert all SVG icons to PNG using cairosvg for proper stroke rendering."""
import cairosvg
import os
import glob

SRC = 'docs/design/icons'
DST = 'miniprogram/assets/icons'

os.makedirs(DST, exist_ok=True)

# Also generate tabbar icons at 81x81
TABBAR_DST = 'miniprogram/assets/tabbar'
os.makedirs(TABBAR_DST, exist_ok=True)

for svg in sorted(glob.glob(os.path.join(SRC, '*.svg'))):
    name = os.path.splitext(os.path.basename(svg))[0]
    if '(1)' in name:
        continue

    out = os.path.join(DST, name + '.png')
    with open(svg) as f:
        content = f.read()
    cairosvg.svg2png(bytestring=content.encode(), write_to=out, output_width=96, output_height=96)
    sz = os.path.getsize(out)
    print(f'{name}.png: {sz} bytes')

# Generate tabbar icons at 81x81
for name in ['tab-table', 'tab-leaderboard', 'tab-history', 'tab-mine']:
    for suffix in ['default', 'active']:
        svg = os.path.join(SRC, f'{name}-{suffix}.svg')
        if os.path.exists(svg):
            out = os.path.join(TABBAR_DST, f'{name}-{suffix}.png')
            with open(svg) as f:
                content = f.read()
            cairosvg.svg2png(bytestring=content.encode(), write_to=out, output_width=81, output_height=81)
            sz = os.path.getsize(out)
            print(f'tabbar/{name}-{suffix}.png: {sz} bytes')

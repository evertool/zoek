#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""把房间页 4 个等腰梯形的 4 个角做成圆角，输出可粘进 WXSS 的 polygon() 值。

为什么要「多段折线拟合」而不是 clip-path: path()/SVG：
clip-path: polygon() 是这几个形状现在唯一依赖的能力（内层白底还要靠同一个百分比多边形做描边），
换成 SVG 背景或 path() 在部分安卓 WebView 上有兜不住的风险——真机一旦不支持，座位会退化成矩形。
用折线拟合圆弧，最坏情况也只是圆角略显棱角，形状本身不会坏。

用法：改 RADIUS 后重跑，把输出覆盖 miniprogram/pages/room/room.wxss 里对应的 8 条 clip-path。
自检：输出的面积应比原梯形略小（r=20 时约 -0.8%）；若差得多，说明切点算错了。
"""
import math

RADIUS = 20.0      # 圆角半径（rpx）
SEGMENTS = 4       # 每个角用几段折线拟合（4 段时与真圆弧的偏差 ≈ 0.02r ≈ 0.4rpx，肉眼不可见）

# 与现有 room.wxss 完全一致的几何（元素本地坐标，rpx）
# 短边 190、长边 316、进深 210；短边那一侧贴中心圆
SHAPES = {
    # 上/下：盒子 316×210
    'top':    (316, 210, [(62.9, 210), (253.1, 210), (316, 0), (0, 0)]),
    'bottom': (316, 210, [(62.9, 0), (253.1, 0), (316, 210), (0, 210)]),
    # 左/右：盒子 210×316
    'left':   (210, 316, [(0, 0), (0, 316), (210, 253.1), (210, 62.9)]),
    'right':  (210, 316, [(210, 0), (210, 316), (0, 253.1), (0, 62.9)]),
}


def sub(a, b):
    return (a[0] - b[0], a[1] - b[1])


def add(a, b):
    return (a[0] + b[0], a[1] + b[1])


def mul(a, k):
    return (a[0] * k, a[1] * k)


def norm(a):
    n = math.hypot(a[0], a[1])
    return (a[0] / n, a[1] / n)


def round_corner(prev_p, p, next_p, r):
    """返回该角上的圆弧采样点（含两端切点），直线段由调用方连接。"""
    u = norm(sub(prev_p, p))   # 指向前一个顶点
    v = norm(sub(next_p, p))   # 指向后一个顶点
    cos_t = max(-1.0, min(1.0, u[0] * v[0] + u[1] * v[1]))
    theta = math.acos(cos_t)                     # 角内夹角
    t = r / math.tan(theta / 2)                  # 切点距顶点的距离
    # 角平分线方向（指向形状内部一侧由顶点顺序决定，这里取 u+v 的归一化）
    bis = norm(add(u, v))
    center = add(p, mul(bis, r / math.sin(theta / 2)))
    a_start = math.atan2(*(sub(add(p, mul(u, t)), center))[::-1])
    a_end = math.atan2(*(sub(add(p, mul(v, t)), center))[::-1])
    # 走劣弧（角度差 < π）
    d = a_end - a_start
    while d > math.pi:
        d -= 2 * math.pi
    while d < -math.pi:
        d += 2 * math.pi
    pts = []
    for i in range(SEGMENTS + 1):
        ang = a_start + d * i / SEGMENTS
        pts.append((center[0] + r * math.cos(ang), center[1] + r * math.sin(ang)))
    return pts


def rounded_polygon(w, h, verts, r):
    n = len(verts)
    out = []
    for i in range(n):
        out.extend(round_corner(verts[(i - 1) % n], verts[i], verts[(i + 1) % n], r))
    return [(x / w * 100.0, y / h * 100.0) for x, y in out]


def css(points):
    return 'polygon(' + ', '.join(f'{x:.1f}% {y:.1f}%' for x, y in points) + ')'


def area(points):
    s = 0.0
    for i in range(len(points)):
        x1, y1 = points[i]
        x2, y2 = points[(i + 1) % len(points)]
        s += x1 * y2 - x2 * y1
    return abs(s) / 2.0


print(f'/* 圆角半径 {RADIUS:g}rpx，每角 {SEGMENTS} 段折线 */')
for name, (w, h, verts) in SHAPES.items():
    poly = rounded_polygon(w, h, verts, RADIUS)
    # 形状面积守恒性自检：圆角只会削掉四角，面积应略小于原梯形
    raw = [(x / w * 100.0, y / h * 100.0) for x, y in verts]
    print(f'/* {name}: 顶点 {len(poly)} 个，面积 {area(poly):.1f}%²（原 {area(raw):.1f}%²，应为略小） */')
    print(f'.table-side.side-{name} {{ clip-path: {css(poly)}; }}')
    print(f'.side-{name} .side-fill {{ clip-path: {css(poly)}; }}')
    print()

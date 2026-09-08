// utils/lottie-chart.js — 把得分序列转成 Lottie 折线图动画数据
// 视觉：灰网格 → 红色折线描线生长（trim path）→ 节点金色圆点逐个弹出 + 面积淡入
// 输出的 animationData 直接交给 lottie-miniprogram 渲染到 type=2d canvas。

function buildLineChart(values, opts) {
  opts = opts || {}
  const W = opts.width || 320
  const H = opts.height || 150
  const padL = 18
  const padR = 18
  const padT = 22
  const padB = 20
  const lineColor = opts.lineColor || [0.91, 0.267, 0.18, 1]   // #E8442E
  const dotColor = opts.dotColor || [0.788, 0.635, 0.329, 1]   // #C9A254
  const gridColor = opts.gridColor || [0.929, 0.906, 0.847, 1] // #EDE7D8
  const fillBase = opts.fillColor || [0.91, 0.267, 0.18]

  const n = (values || []).length
  if (n < 2) return null

  let mn = Math.min(0, ...values)
  let mx = Math.max(0, ...values)
  if (mx === mn) mx = mn + 1
  // 上下各留 8% 呼吸空间
  const span = mx - mn
  mx += span * 0.08
  if (mn < 0) mn -= span * 0.08

  const innerW = W - padL - padR
  const innerH = H - padT - padB
  const xAt = (i) => padL + innerW * i / (n - 1)
  const yAt = (v) => padT + innerH * (1 - (v - mn) / (mx - mn))
  const r1 = (x) => Math.round(x * 10) / 10

  const points = values.map((v, i) => [r1(xAt(i)), r1(yAt(v))])
  const zeroY = r1(yAt(0))

  const FR = 60
  const revealStart = 14
  const revealEnd = Math.min(78, revealStart + n * 4)

  const layers = []

  // ---- 网格线（静态） ----
  const gridItems = []
  const gridCount = 3
  for (let gi = 0; gi < gridCount; gi++) {
    const gy = r1(padT + innerH * gi / (gridCount - 1))
    gridItems.push({
      ty: 'sh',
      ks: { a: 0, k: { i: [[0, 0], [0, 0]], o: [[0, 0], [0, 0]], v: [[padL, gy], [W - padR, gy]], c: false } }
    })
  }
  gridItems.push({
    ty: 'st', c: { a: 0, k: gridColor }, o: { a: 0, k: 100 },
    w: { a: 0, k: 2 }, lc: 2, lj: 2, bm: 0, nm: 'grid-stroke'
  })
  layers.push({
    ddd: 0, ind: 1, ty: 4, nm: 'grid', sr: 1,
    ks: {
      o: { a: 0, k: 100 }, r: { a: 0, k: 0 },
      p: { a: 0, k: [0, 0, 0] }, a: { a: 0, k: [0, 0, 0] }, s: { a: 0, k: [100, 100, 100] }
    },
    ao: 0,
    shapes: [{ ty: 'gr', it: gridItems, nm: 'grid-grp', np: gridItems.length, bm: 0, hd: false }],
    ip: 0, op: FR * 4, st: 0, bm: 0
  })

  // ---- 零线（稍深，帮助看出正负分界） ----
  layers.push({
    ddd: 0, ind: 2, ty: 4, nm: 'zero-line', sr: 1,
    ks: {
      o: { a: 0, k: 100 }, r: { a: 0, k: 0 },
      p: { a: 0, k: [0, 0, 0] }, a: { a: 0, k: [0, 0, 0] }, s: { a: 0, k: [100, 100, 100] }
    },
    ao: 0,
    shapes: [{
      ty: 'gr', it: [
        { ty: 'sh', ks: { a: 0, k: { i: [[0, 0], [0, 0]], o: [[0, 0], [0, 0]], v: [[padL, zeroY], [W - padR, zeroY]], c: false } } },
        { ty: 'st', c: { a: 0, k: [0.78, 0.75, 0.68, 1] }, o: { a: 0, k: 80 }, w: { a: 0, k: 2 }, lc: 2, lj: 2, bm: 0, nm: 'zero-stroke' }
      ], nm: 'zero-grp', np: 2, bm: 0, hd: false
    }],
    ip: 0, op: FR * 4, st: 0, bm: 0
  })

  // ---- 面积填充：折线下压到零线，描线完成后淡入 ----
  const areaPts = points.concat([[points[n - 1][0], zeroY], [points[0][0], zeroY]])
  const closedPath = {
    i: areaPts.map(() => [0, 0]),
    o: areaPts.map(() => [0, 0]),
    v: areaPts,
    c: true
  }
  layers.push({
    ddd: 0, ind: 3, ty: 4, nm: 'area', sr: 1,
    ks: {
      o: { a: 1, k: [
        { t: revealEnd - 8, s: [0], i: { x: [0.4], y: [1] }, o: { x: [0.6], y: [0] } },
        { t: revealEnd + 12, s: [7] }
      ] },
      r: { a: 0, k: 0 },
      p: { a: 0, k: [0, 0, 0] }, a: { a: 0, k: [0, 0, 0] }, s: { a: 0, k: [100, 100, 100] }
    },
    ao: 0,
    shapes: [{
      ty: 'gr', it: [
        { ty: 'sh', ks: { a: 0, k: closedPath } },
        { ty: 'fl', c: { a: 0, k: [fillBase[0], fillBase[1], fillBase[2], 1] }, o: { a: 0, k: 100 }, r: 1, bm: 0, nm: 'area-fill' }
      ], nm: 'area-grp', np: 2, bm: 0, hd: false
    }],
    ip: 0, op: FR * 4, st: 0, bm: 0
  })

  // ---- 主折线：trim path 描线生长 ----
  const linePath = {
    i: points.map(() => [0, 0]),
    o: points.map(() => [0, 0]),
    v: points,
    c: false
  }
  layers.push({
    ddd: 0, ind: 4, ty: 4, nm: 'trend-line', sr: 1,
    ks: {
      o: { a: 0, k: 100 }, r: { a: 0, k: 0 },
      p: { a: 0, k: [0, 0, 0] }, a: { a: 0, k: [0, 0, 0] }, s: { a: 0, k: [100, 100, 100] }
    },
    ao: 0,
    shapes: [{
      ty: 'gr', it: [
        { ty: 'sh', ks: { a: 0, k: linePath }, nm: 'line-sh' },
        {
          ty: 'tm', s: { a: 0, k: 0 },
          e: { a: 1, k: [
            { t: revealStart, s: [0], i: { x: [0.3], y: [1] }, o: { x: [0.6], y: [0] } },
            { t: revealEnd, s: [100] }
          ] },
          o: { a: 0, k: 0 }, m: 1, nm: 'line-trim'
        },
        { ty: 'st', c: { a: 0, k: lineColor }, o: { a: 0, k: 100 }, w: { a: 0, k: 6 }, lc: 2, lj: 2, bm: 0, nm: 'line-stroke' }
      ], nm: 'line-grp', np: 3, bm: 0, hd: false
    }],
    ip: 0, op: FR * 4, st: 0, bm: 0
  })

  // ---- 数据点：金色圆点逐个弹出 ----
  let ind = 5
  points.forEach((pt, i) => {
    const t0 = Math.round(revealStart + (revealEnd - revealStart) * i / (n - 1))
    const sz = (i === n - 1) ? 16 : 12
    layers.push({
      ddd: 0, ind: ind, ty: 4, nm: 'dot-' + i, sr: 1,
      ks: {
        o: { a: 1, k: [{ t: t0, s: [0] }, { t: t0 + 6, s: [100] }] },
        r: { a: 0, k: 0 },
        p: { a: 0, k: [pt[0], pt[1], 0] },
        a: { a: 0, k: [0, 0, 0] },
        s: { a: 1, k: [
          { t: t0, s: [0, 0, 100], i: { x: [0.3, 0.3, 0.3], y: [1, 1, 1] }, o: { x: [0.5, 0.5, 0.5], y: [0, 0, 0] } },
          { t: t0 + 5, s: [130, 130, 100] },
          { t: t0 + 9, s: [100, 100, 100] }
        ] }
      },
      ao: 0,
      shapes: [{
        ty: 'gr', it: [
          { ty: 'el', s: { a: 0, k: [sz, sz] }, p: { a: 0, k: [0, 0] }, d: 1, bm: 0, nm: 'dot-el' },
          { ty: 'fl', c: { a: 0, k: dotColor }, o: { a: 0, k: 100 }, r: 1, bm: 0, nm: 'dot-fill' }
        ], nm: 'dot-grp', np: 2, bm: 0, hd: false
      }],
      ip: 0, op: FR * 4, st: 0, bm: 0
    })
    ind++
  })

  return {
    v: '5.7.4', fr: FR, ip: 0, op: revealEnd + 30, w: W, h: H,
    nm: 'trend-line-chart', ddd: 0, assets: [], layers: layers, markers: []
  }
}

module.exports = { buildLineChart }

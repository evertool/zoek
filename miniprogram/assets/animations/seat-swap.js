// assets/animations/seat-swap.js
// 长按换座时的 Lottie 反馈动画 — 两个玩家色点沿弧线互换 + 中央双向箭头脉冲
// 画布 240 x 240，时长 1 秒（fr:40, op:40），loop:false
// 注意: lottie-miniprogram Canvas renderer 不支持文字层(ty:5)

// 完整 transform 对象
function tr(px, py, sx, sy, rot) {
  return {
    ty: 'tr',
    p: { a: 0, k: [px, py], ix: 2 },
    a: { a: 0, k: [0, 0], ix: 1 },
    s: { a: 0, k: [sx, sy], ix: 3 },
    r: { a: 0, k: rot, ix: 6 },
    o: { a: 0, k: 100, ix: 7 },
    sk: { a: 0, k: 0, ix: 4 },
    sa: { a: 0, k: 0, ix: 5 },
    nm: 'Transform'
  }
}

// 圆形玩家色点
function dotLayer(ind, name, color, kfs, startT) {
  return {
    ddd: 0, ind: ind, ty: 4, nm: name, sr: 1,
    ks: {
      o: { a: 0, k: 100, ix: 11 },
      r: { a: 0, k: 0, ix: 10 },
      p: { a: 1, k: kfs, ix: 2 },
      a: { a: 0, k: [0, 0, 0], ix: 1 },
      s: { a: 0, k: [100, 100, 100], ix: 6 }
    },
    ao: 0,
    shapes: [{
      ty: 'gr',
      it: [
        { ty: 'el', d: 1, s: { a: 0, k: [58, 58], ix: 2 }, p: { a: 0, k: [0, 0], ix: 3 }, nm: 'Dot' },
        { ty: 'fl', c: { a: 0, k: color, ix: 4 }, o: { a: 0, k: 100, ix: 5 }, r: 1, bm: 0, nm: 'Fill' },
        tr(0, 0, 100, 100, 0)
      ],
      nm: name + 'Grp', np: 3, bm: 0, hd: false
    }],
    ip: 0, op: 40, st: 0, bm: 0
  }
}

// 三角箭头（dir=1 指向右, dir=-1 指向左）
function arrowLayer(ind, name, color, x, startT) {
  var s = 26
  var v = [[-s, -s], [s, 0], [-s, s]]
  if (x < 0) v = [[s, -s], [-s, 0], [s, s]]
  return {
    ddd: 0, ind: ind, ty: 4, nm: name, sr: 1,
    ks: {
      o: { a: 0, k: 100, ix: 11 },
      r: { a: 0, k: 0, ix: 10 },
      p: { a: 0, k: [120 + x, 120], ix: 2 },
      a: { a: 0, k: [0, 0, 0], ix: 1 },
      s: { a: 1, k: [
        { t: startT, s: [70, 70, 100], i: { x: [0.3, 0.3, 0.3], y: [1, 1, 1] }, o: { x: [0.6, 0.6, 0.6], y: [0, 0, 0] } },
        { t: startT + 20, s: [130, 130, 100] },
        { t: startT + 40, s: [70, 70, 100] }
      ], ix: 6 }
    },
    ao: 0,
    shapes: [{
      ty: 'gr',
      it: [
        { ty: 'sh', ks: { a: 0, k: { i: [[0, 0], [0, 0], [0, 0]], o: [[0, 0], [0, 0], [0, 0]], v: v, c: true } }, nm: 'ArrowPath' },
        { ty: 'fl', c: { a: 0, k: color, ix: 4 }, o: { a: 0, k: 100, ix: 5 }, r: 1, bm: 0, nm: 'Fill' },
        tr(0, 0, 100, 100, 0)
      ],
      nm: name + 'Grp', np: 3, bm: 0, hd: false
    }],
    ip: 0, op: 40, st: 0, bm: 0
  }
}

// 颜色
var RED = [0.91, 0.26, 0.18, 1]   // 当前玩家
var BLUE = [0.10, 0.10, 0.18, 1]  // 目标座位
var GOLD = [0.79, 0.64, 0.33, 1]  // 箭头

// 红点：左上 → 沿上弧 → 右下
var redKfs = [
  { t: 0, s: [92, 120], i: { x: [0.4, 0.4], y: [1, 1] }, o: { x: [0.6, 0.6], y: [0, 0] } },
  { t: 20, s: [120, 82] },
  { t: 40, s: [148, 120] }
]
// 蓝点：右下 → 沿下弧 → 左上
var blueKfs = [
  { t: 0, s: [148, 120], i: { x: [0.4, 0.4], y: [1, 1] }, o: { x: [0.6, 0.6], y: [0, 0] } },
  { t: 20, s: [120, 158] },
  { t: 40, s: [92, 120] }
]

module.exports = {
  v: '5.7.6',
  fr: 40,
  ip: 0,
  op: 40,
  w: 240,
  h: 240,
  nm: 'SeatSwap',
  ddd: 0,
  assets: [],
  layers: [
    dotLayer(1, 'Dot-Red', RED, redKfs, 0),
    dotLayer(2, 'Dot-Blue', BLUE, blueKfs, 0),
    arrowLayer(3, 'Arrow-Right', GOLD, 16, 0),
    arrowLayer(4, 'Arrow-Left', GOLD, -16, 0)
  ]
}

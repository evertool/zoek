// assets/animations/login-tiles.js
// 麻将牌从天而降 Lottie 动画 — v2 增强版
// 6 个麻将牌从不同方向掉落 + 弹跳 + 阴影
// 注意: lottie-miniprogram Canvas renderer 不支持文字层(ty:5)
// 所以这里用不同颜色的圆角矩形 + 装饰图形来区分牌面

// 完整的 transform 对象
function tr(px, py, sx, sy, rot) {
  return {
    "ty": "tr",
    "p": { "a": 0, "k": [px, py], "ix": 2 },
    "a": { "a": 0, "k": [0, 0], "ix": 1 },
    "s": { "a": 0, "k": [sx, sy], "ix": 3 },
    "r": { "a": 0, "k": rot, "ix": 6 },
    "o": { "a": 0, "k": 100, "ix": 7 },
    "sk": { "a": 0, "k": 0, "ix": 4 },
    "sa": { "a": 0, "k": 0, "ix": 5 },
    "nm": "Transform"
  };
}

// 麻将牌 shape group — 带颜色装饰
// fillColor: [r, g, b, a] 牌面颜色
// accentColor: [r, g, b, a] 装饰条颜色
function tileGroup(name, fillColor, accentColor) {
  return {
    "ty": "gr",
    "it": [
      // 外层圆角矩形（牌身）
      {
        "ty": "rc",
        "d": 1,
        "s": { "a": 0, "k": [70, 100], "ix": 2 },
        "p": { "a": 0, "k": [0, 0], "ix": 3 },
        "r": { "a": 0, "k": 10, "ix": 4 },
        "nm": "TileBody"
      },
      // 牌面填充
      {
        "ty": "fl",
        "c": { "a": 0, "k": fillColor, "ix": 4 },
        "o": { "a": 0, "k": 100, "ix": 5 },
        "r": 1,
        "bm": 0,
        "nm": "TileFill"
      },
      // 牌面描边
      {
        "ty": "st",
        "c": { "a": 0, "k": [0.84, 0.80, 0.71, 1], "ix": 3 },
        "o": { "a": 0, "k": 60, "ix": 4 },
        "w": { "a": 0, "k": 2, "ix": 5 },
        "lc": 1,
        "lj": 1,
        "bm": 0,
        "nm": "TileStroke"
      },
      tr(0, 0, 100, 100, 0)
    ],
    "nm": name,
    "np": 4,
    "cix": 2,
    "bm": 0,
    "ix": 0,
    "mn": "ADBE Vector Group",
    "hd": false
  };
}

// 带装饰的麻将牌（中间有一个彩色圆点/方块来区分不同牌）
function decoratedTileGroup(name, fillColor, accentColor) {
  return {
    "ty": "gr",
    "it": [
      // 牌身外框
      {
        "ty": "rc",
        "d": 1,
        "s": { "a": 0, "k": [70, 100], "ix": 2 },
        "p": { "a": 0, "k": [0, 0], "ix": 3 },
        "r": { "a": 0, "k": 10, "ix": 4 },
        "nm": "TileBody"
      },
      // 牌面填充
      {
        "ty": "fl",
        "c": { "a": 0, "k": fillColor, "ix": 4 },
        "o": { "a": 0, "k": 100, "ix": 5 },
        "r": 1,
        "bm": 0,
        "nm": "TileFill"
      },
      // 牌面描边
      {
        "ty": "st",
        "c": { "a": 0, "k": [0.84, 0.80, 0.71, 1], "ix": 3 },
        "o": { "a": 0, "k": 60, "ix": 4 },
        "w": { "a": 0, "k": 2, "ix": 5 },
        "lc": 1,
        "lj": 1,
        "bm": 0,
        "nm": "TileStroke"
      },
      // 中心装饰圆点（区分牌面）
      {
        "ty": "gr",
        "it": [
          {
            "ty": "el",
            "d": 1,
            "s": { "a": 0, "k": [28, 28], "ix": 2 },
            "p": { "a": 0, "k": [0, 0], "ix": 3 },
            "nm": "AccentDot"
          },
          {
            "ty": "fl",
            "c": { "a": 0, "k": accentColor, "ix": 4 },
            "o": { "a": 0, "k": 100, "ix": 5 },
            "r": 1,
            "bm": 0,
            "nm": "AccentFill"
          },
          tr(0, 0, 100, 100, 0)
        ],
        "nm": "Accent",
        "np": 3,
        "cix": 2,
        "bm": 0,
        "ix": 1,
        "mn": "ADBE Vector Group",
        "hd": false
      },
      tr(0, 0, 100, 100, 0)
    ],
    "nm": name,
    "np": 5,
    "cix": 2,
    "bm": 0,
    "ix": 0,
    "mn": "ADBE Vector Group",
    "hd": false
  };
}

// 阴影 shape group
function shadowGroup(name) {
  return {
    "ty": "gr",
    "it": [
      {
        "ty": "el",
        "d": 1,
        "s": { "a": 0, "k": [40, 16], "ix": 2 },
        "p": { "a": 0, "k": [0, 0], "ix": 3 },
        "nm": "Ellipse"
      },
      {
        "ty": "fl",
        "c": { "a": 0, "k": [0, 0, 0, 1], "ix": 4 },
        "o": { "a": 0, "k": 100, "ix": 5 },
        "r": 1,
        "bm": 0,
        "nm": "Fill"
      },
      tr(0, 0, 100, 100, 0)
    ],
    "nm": name,
    "np": 3,
    "cix": 2,
    "bm": 0,
    "ix": 0,
    "mn": "ADBE Vector Group",
    "hd": false
  };
}

// === Keyframe 辅助函数 ===

// 掉落轨迹 keyframes — 更快的掉落 + 更大的弹跳
// startT: 起始时间, fromY: 起始Y, landY: 落地Y, xPos: X位置
function dropKeyframes(startT, fromY, landY, xPos) {
  var dropDist = landY - fromY;
  return [
    { "i": { "x": 0.2, "y": 1 }, "o": { "x": 0.4, "y": 0 }, "t": startT, "s": [xPos, fromY], "to": [0, dropDist / 3], "ti": [0, -dropDist / 3] },
    { "i": { "x": 0.3, "y": 1 }, "o": { "x": 0.5, "y": 0 }, "t": startT + 20, "s": [xPos, landY], "to": [0, -12], "ti": [0, 12] },
    { "i": { "x": 0.25, "y": 1 }, "o": { "x": 0.5, "y": 0 }, "t": startT + 28, "s": [xPos, landY - 35], "to": [0, 12], "ti": [0, -12] },
    { "i": { "x": 0.3, "y": 1 }, "o": { "x": 0.6, "y": 0 }, "t": startT + 38, "s": [xPos, landY - 5], "to": [0, -4], "ti": [0, 4] },
    { "i": { "x": 0.3, "y": 1 }, "o": { "x": 0.7, "y": 0 }, "t": startT + 48, "s": [xPos, landY - 15], "to": [0, 5], "ti": [0, -5] },
    { "t": 100, "s": [xPos, landY - 8] }
  ];
}

// 弧形掉落轨迹（从侧面飞入）
function arcDropKeyframes(startT, fromX, fromY, landX, landY) {
  var midX = (fromX + landX) / 2;
  var midY = Math.min(fromY, landY) - 60; // 弧形最高点
  return [
    { "i": { "x": 0.2, "y": 1 }, "o": { "x": 0.4, "y": 0 }, "t": startT, "s": [fromX, fromY], "to": [(midX - fromX) / 3, (midY - fromY) / 3], "ti": [-(midX - fromX) / 3, -(midY - fromY) / 3] },
    { "i": { "x": 0.3, "y": 1 }, "o": { "x": 0.5, "y": 0 }, "t": startT + 25, "s": [midX, midY], "to": [(landX - midX) / 3, (landY - midY) / 3 * 0.5], "ti": [(fromX - midX) / 3, (fromY - midY) / 3 * 0.5] },
    { "i": { "x": 0.3, "y": 1 }, "o": { "x": 0.5, "y": 0 }, "t": startT + 40, "s": [landX, landY], "to": [0, -12], "ti": [0, 12] },
    { "i": { "x": 0.25, "y": 1 }, "o": { "x": 0.6, "y": 0 }, "t": startT + 48, "s": [landX, landY - 30], "to": [0, 10], "ti": [0, -10] },
    { "i": { "x": 0.3, "y": 1 }, "o": { "x": 0.7, "y": 0 }, "t": startT + 58, "s": [landX, landY - 3], "to": [0, -3], "ti": [0, 3] },
    { "t": 100, "s": [landX, landY - 8] }
  ];
}

// 旋转 keyframes — 更快的旋转
function rotateKeyframes(startT, fromR, toR) {
  return [
    { "i": { "x": [0.2], "y": [1] }, "o": { "x": [0.4], "y": [0] }, "t": startT, "s": [fromR] },
    { "i": { "x": [0.3], "y": [1] }, "o": { "x": [0.5], "y": [0] }, "t": startT + 20, "s": [toR] },
    { "i": { "x": [0.25], "y": [1] }, "o": { "x": [0.6], "y": [0] }, "t": startT + 38, "s": [toR + 30] },
    { "i": { "x": [0.3], "y": [1] }, "o": { "x": [0.7], "y": [0] }, "t": startT + 48, "s": [toR + 15] },
    { "t": 100, "s": [toR + 20] }
  ];
}

// 缩放 keyframes — 更大的挤压弹跳
function scaleKeyframes(startT) {
  return [
    { "i": { "x": [0.2, 0.2, 0.2], "y": [1, 1, 1] }, "o": { "x": [0.4, 0.4, 0.4], "y": [0, 0, 0] }, "t": startT, "s": [70, 70, 100] },
    { "i": { "x": [0.3, 0.3, 0.3], "y": [1, 1, 1] }, "o": { "x": [0.5, 0.5, 0.5], "y": [0, 0, 0] }, "t": startT + 20, "s": [115, 80, 100] },
    { "i": { "x": [0.25, 0.25, 0.25], "y": [1, 1, 1] }, "o": { "x": [0.5, 0.5, 0.5], "y": [0, 0, 0] }, "t": startT + 28, "s": [90, 110, 100] },
    { "i": { "x": [0.3, 0.3, 0.3], "y": [1, 1, 1] }, "o": { "x": [0.6, 0.6, 0.6], "y": [0, 0, 0] }, "t": startT + 38, "s": [105, 95, 100] },
    { "i": { "x": [0.3, 0.3, 0.3], "y": [1, 1, 1] }, "o": { "x": [0.7, 0.7, 0.7], "y": [0, 0, 0] }, "t": startT + 48, "s": [97, 103, 100] },
    { "t": 100, "s": [100, 100, 100] }
  ];
}

// 阴影透明度
function shadowOpacityKeyframes(startT) {
  return [
    { "i": { "x": [0.25], "y": [1] }, "o": { "x": [0.5], "y": [0] }, "t": startT, "s": [0] },
    { "i": { "x": [0.25], "y": [1] }, "o": { "x": [0.5], "y": [0] }, "t": startT + 18, "s": [35] },
    { "t": 100, "s": [20] }
  ];
}

// 阴影缩放
function shadowScaleKeyframes(startT) {
  return [
    { "i": { "x": [0.25, 0.25, 0.25], "y": [1, 1, 1] }, "o": { "x": [0.5, 0.5, 0.5], "y": [0, 0, 0] }, "t": startT, "s": [30, 15, 100] },
    { "i": { "x": [0.25, 0.25, 0.25], "y": [1, 1, 1] }, "o": { "x": [0.5, 0.5, 0.5], "y": [0, 0, 0] }, "t": startT + 20, "s": [110, 45, 100] },
    { "i": { "x": [0.25, 0.25, 0.25], "y": [1, 1, 1] }, "o": { "x": [0.6, 0.6, 0.6], "y": [0, 0, 0] }, "t": startT + 28, "s": [85, 35, 100] },
    { "t": 100, "s": [95, 38, 100] }
  ];
}

// 构建麻将牌 layer
function tileLayer(ind, name, shapes, startT, dropKFs, rotKFs, scaleKFs) {
  return {
    "ddd": 0,
    "ind": ind,
    "ty": 4,
    "nm": name,
    "sr": 1,
    "ks": {
      "o": { "a": 0, "k": 100, "ix": 11 },
      "r": { "a": 1, "k": rotKFs, "ix": 10 },
      "p": { "a": 1, "k": dropKFs, "ix": 2 },
      "s": { "a": 1, "k": scaleKFs, "ix": 6 }
    },
    "ao": 0,
    "shapes": shapes,
    "ip": 0,
    "op": 100,
    "st": startT,
    "bm": 0
  };
}

// 构建阴影 layer
function shadowLayer(ind, name, xPos, yPos, startT) {
  return {
    "ddd": 0,
    "ind": ind,
    "ty": 4,
    "nm": name,
    "sr": 1,
    "ks": {
      "o": { "a": 1, "k": shadowOpacityKeyframes(startT), "ix": 11 },
      "r": { "a": 0, "k": 0, "ix": 10 },
      "p": { "a": 0, "k": [xPos, yPos], "ix": 2 },
      "s": { "a": 1, "k": shadowScaleKeyframes(startT), "ix": 6 }
    },
    "ao": 0,
    "shapes": [shadowGroup(name + "Group")],
    "ip": 0,
    "op": 100,
    "st": startT,
    "bm": 0
  };
}

// === 颜色定义 ===
var TILE_FACE = [0.98, 0.97, 0.93, 1];    // 米白牌面
var GREEN     = [0.12, 0.42, 0.28, 1];    // 翠绿（發）
var RED       = [0.91, 0.26, 0.18, 1];    // 麻将红（中）
var GOLD      = [0.79, 0.64, 0.33, 1];    // 金色（白板）
var BLUE      = [0.10, 0.10, 0.18, 1];    // 深靛蓝
var TEAL      = [0.0, 0.50, 0.50, 1];    // 青色

// === 构建动画 ===
// 6张麻将牌，从不同位置和方向掉落
// 画布 400 x 500，落地位置在 y=180~280 之间

module.exports = {
  "v": "5.7.6",
  "fr": 30,
  "ip": 0,
  "op": 100,
  "w": 400,
  "h": 500,
  "nm": "MahjongTilesDropV2",
  "ddd": 0,
  "assets": [],
  "layers": [
    // === 牌1: 發（绿色装饰） — 从正上方掉落，最早 ===
    tileLayer(
      1, "Tile-Fa",
      [decoratedTileGroup("FaGroup", TILE_FACE, GREEN)],
      0,
      dropKeyframes(0, -80, 220, 80),
      rotateKeyframes(0, -180, -360),
      scaleKeyframes(0)
    ),
    shadowLayer(7, "Shadow-Fa", 80, 310, 0),

    // === 牌2: 中（红色装饰） — 从左上方飞入，弧形轨迹 ===
    tileLayer(
      2, "Tile-Zhong",
      [decoratedTileGroup("ZhongGroup", TILE_FACE, RED)],
      8,
      arcDropKeyframes(8, -50, 100, 160, 250),
      rotateKeyframes(8, 180, 360),
      scaleKeyframes(8)
    ),
    shadowLayer(8, "Shadow-Zhong", 160, 340, 8),

    // === 牌3: 白板（金色装饰） — 从右上方飞入，弧形轨迹 ===
    tileLayer(
      3, "Tile-Bai",
      [decoratedTileGroup("BaiGroup", TILE_FACE, GOLD)],
      16,
      arcDropKeyframes(16, 450, 80, 280, 230),
      rotateKeyframes(16, 90, 270),
      scaleKeyframes(16)
    ),
    shadowLayer(9, "Shadow-Bai", 280, 320, 16),

    // === 牌4: 蓝色装饰 — 从左下方飞入 ===
    tileLayer(
      4, "Tile-Blue",
      [decoratedTileGroup("BlueGroup", TILE_FACE, BLUE)],
      24,
      arcDropKeyframes(24, -50, 400, 120, 270),
      rotateKeyframes(24, -90, -270),
      scaleKeyframes(24)
    ),
    shadowLayer(10, "Shadow-Blue", 120, 360, 24),

    // === 牌5: 青色装饰 — 从右下方飞入 ===
    tileLayer(
      5, "Tile-Teal",
      [decoratedTileGroup("TealGroup", TILE_FACE, TEAL)],
      32,
      arcDropKeyframes(32, 450, 400, 320, 260),
      rotateKeyframes(32, 45, 225),
      scaleKeyframes(32)
    ),
    shadowLayer(11, "Shadow-Teal", 320, 350, 32),

    // === 牌6: 绿色装饰 — 从正上方掉落，最晚 ===
    tileLayer(
      6, "Tile-Green2",
      [decoratedTileGroup("Green2Group", TILE_FACE, GREEN)],
      40,
      dropKeyframes(40, -80, 200, 200),
      rotateKeyframes(40, 360, 180),
      scaleKeyframes(40)
    ),
    shadowLayer(12, "Shadow-Green2", 200, 290, 40)
  ]
};

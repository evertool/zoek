// utils/score-tags.js — 给分抽屉配置（标签 + 快捷预设分值）
// 正常情况由后端下发：GET /adjustments/config（internal/handler/adjustment.go 的
// adjustmentTagConfig 是唯一事实来源），端上缓存 24h（storage key score_config_v1），
// 有效期内直接用缓存不发请求。后端不可达 / 从没拉到过配置时，用下面的内置默认兜底，
// 保证抽屉永远能用。
// code 落库：score_adjustments.tags 存逗号串；labelsOf 只认认识的 code，
// 旧数据 / 端上落后于后端时未知 code 直接丢弃，不显示成乱码。

const DEFAULT_TAGS = [
  { code: 'zimo', label: '自摸' },
  { code: 'fanggang', label: '放杠' },
  { code: 'minggang', label: '明杠' },
  { code: 'angang', label: '暗杠' },
  { code: 'gangbao', label: '杠爆' },
  { code: 'qianggang', label: '抢杠' }
]

const DEFAULT_PRESETS = [4, 5, 7]

const CACHE_KEY = 'score_config_v1'
const CACHE_TTL = 24 * 60 * 60 * 1000

// 注意：这两个数组**原地更新**（splice），绝不整体替换 —— 消费方
// （room/game-chart/game-detail）拿的是模块导出的同一引用，替换会让旧引用变陈旧。
let SCORE_TAGS = DEFAULT_TAGS.slice()
let PRESETS = DEFAULT_PRESETS.slice()
let LABEL_BY_CODE = buildLabels(SCORE_TAGS)

function buildLabels(tags) {
  const acc = {}
  for (var i = 0; i < tags.length; i++) acc[tags[i].code] = tags[i].label
  return acc
}

function replaceInPlace(arr, items) {
  arr.splice(0, arr.length)
  for (var i = 0; i < items.length; i++) arr.push(items[i])
}

// 应用一份配置（来自后端或缓存）。只收 code+label 都非空的标签、正整数分值，
// 脏数据宁缺勿滥 —— 把抽屉弄空比少一个标签糟糕得多。
// 返回标签是否真的被应用（全脏时 false，调用方据此决定要不要写缓存）。
function applyConfig(tags, presets) {
  let tagsApplied = false
  if (Array.isArray(tags) && tags.length) {
    const clean = []
    for (var i = 0; i < tags.length; i++) {
      var t = tags[i]
      if (t && t.code && t.label) clean.push({ code: t.code, label: t.label })
    }
    if (clean.length) {
      replaceInPlace(SCORE_TAGS, clean)
      LABEL_BY_CODE = buildLabels(clean)
      tagsApplied = true
    }
  }
  if (Array.isArray(presets) && presets.length) {
    const nums = []
    for (var j = 0; j < presets.length; j++) {
      var n = Number(presets[j])
      if (n > 0) nums.push(n)
    }
    if (nums.length) replaceInPlace(PRESETS, nums)
  }
  return tagsApplied
}

/**
 * 确保配置就绪：24h 内有缓存 → 直接应用缓存并 resolve(false)，不发请求；
 * 否则拉后端接口，成功后写缓存并 resolve(true)；任何失败都静默回退内置默认。
 * 页面 onLoad 时调一次即可，后续打开抽屉读到的就是最新配置。
 * @returns {Promise<boolean>} true = 本次实际发了请求并成功更新
 */
function ensureConfig() {
  return new Promise(function (resolve) {
    let cached = null
    try { cached = wx.getStorageSync(CACHE_KEY) } catch (e) {}
    if (cached && cached.ts && Array.isArray(cached.tags) && cached.tags.length &&
        Date.now() - cached.ts < CACHE_TTL) {
      applyConfig(cached.tags, cached.presets)
      resolve(false)
      return
    }
    // require 放函数内：api.js 顶层不依赖本模块，模块级互引没有环，
    // 但放函数内可以让本模块在无网络环境的纯 stub 测试里也能单独加载。
    const api = require('./api')
    api.get('/adjustments/config', {}, { silent: true }).then(function (res) {
      if (res && Array.isArray(res.tags) && res.tags.length) {
        // 全脏配置（清洗后一个不剩）不应用也不缓存 —— 缓存了等于把重试推迟 24h
        const applied = applyConfig(res.tags, res.presets)
        if (applied) {
          try {
            wx.setStorageSync(CACHE_KEY, { ts: Date.now(), tags: SCORE_TAGS.slice(), presets: PRESETS.slice() })
          } catch (e) {}
          resolve(true)
          return
        }
      }
      resolve(false)
    }).catch(function () {
      // 网络失败：内置默认兜底，不写缓存（下次再试），不打扰用户
      resolve(false)
    })
  })
}

// codes → labels：未知 code 直接丢弃（旧数据/端上落后于后端时不要显示成乱码）
function labelsOf(codes) {
  if (!codes || !codes.length) return []
  var out = []
  for (var i = 0; i < codes.length; i++) {
    var label = LABEL_BY_CODE[codes[i]]
    if (label) out.push(label)
  }
  return out
}

module.exports = {
  SCORE_TAGS,
  PRESETS,
  ensureConfig,
  labelsOf
}

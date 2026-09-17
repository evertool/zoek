// utils/score-tags.js — 给分标签（自摸 / 明杠 / 暗杠 / 放杠 / 杠爆 / 抢杠）
// code 落库：score_adjustments.tags 存逗号串，后端白名单见 internal/handler/adjustment.go
// 的 allowedAdjustmentTags。中文只活在展示层（这里），改文案不用洗库。
// 注意：新增标签要同时改后端白名单，否则接口会返回 INVALID_TAG。
// 顺序 = 弹窗里的排列顺序，也是提交时 tags 的顺序。

const SCORE_TAGS = [
  { code: 'zimo', label: '自摸' },
  { code: 'minggang', label: '明杠' },
  { code: 'angang', label: '暗杠' },
  { code: 'fanggang', label: '放杠' },
  { code: 'gangbao', label: '杠爆' },
  { code: 'qianggang', label: '抢杠' }
]

const LABEL_BY_CODE = SCORE_TAGS.reduce(function(acc, t) {
  acc[t.code] = t.label
  return acc
}, {})

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
  labelsOf
}

// utils/util.js — 通用工具函数

/**
 * 格式化分数显示
 * @param {number} score
 * @returns {string}
 */
function formatScore(score) {
  if (score > 0) return '+' + score
  return '' + score
}

/**
 * 状态文本（粤语）
 * @param {string} status
 * @returns {string}
 */
function statusText(status) {
  const map = {
    forming: '组台中',
    active: '计分中',
    ended: '已结束',
    expired: '已失效',
    cancelled: '已取消',
    open: '等紧入分',
    review: '核对中',
    locked: '已锁定',
    ready_for_next: '已完成',
    pending: '等紧确认',
    accepted: '已生效',
    rejected: '已拒绝',
    cancelled_ajd: '已取消',
    expired_adj: '已失效'
  }
  return map[status] || status
}

/**
 * 状态对应的 CSS class
 */
function statusClass(status) {
  const map = {
    forming: 'tag-forming',
    active: 'tag-active',
    ended: 'tag-ended',
    expired: 'tag-ended',
    cancelled: 'tag-cancelled',
    pending: 'tag-pending',
    accepted: 'tag-accepted'
  }
  return map[status] || 'tag-ended'
}

/**
 * 格式化时间
 * @param {string} dateStr — ISO 时间
 * @returns {string}
 */
function formatTime(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''
  const now = new Date()
  const diff = (now - d) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return Math.floor(diff / 60) + '分钟前'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
  if (diff < 86400 * 7) return Math.floor(diff / 86400) + '日前'
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  const hh = String(d.getHours()).padStart(2, '0')
  const mi = String(d.getMinutes()).padStart(2, '0')
  return `${mm}-${dd} ${hh}:${mi}`
}

/**
 * 调整类型文本
 */
function adjustmentTypeText(type) {
  return type === 'supplement' ? '补分' : '退分'
}

module.exports = {
  formatScore,
  statusText,
  statusClass,
  formatTime,
  adjustmentTypeText
}

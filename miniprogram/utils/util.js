// utils/util.js — 通用工具函数

function formatScore(score) {
  if (score > 0) return '+' + score
  return '' + score
}

function statusText(status) {
  const map = {
    forming: '等待中',
    active: '进行中',
    ended: '已结束',
    expired: '已失效',
    cancelled: '已取消',
    open: '待入分',
    review: '核对中',
    locked: '已锁定',
    ready_for_next: '已完成',
    pending: '待确认',
    accepted: '已生效',
    rejected: '已拒绝',
    cancelled_adj: '已取消',
    expired_adj: '已失效'
  }
  return map[status] || status
}

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

function formatTime(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''
  const now = new Date()
  const diff = (now - d) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return Math.floor(diff / 60) + '分钟前'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
  if (diff < 86400 * 7) return Math.floor(diff / 86400) + '天前'
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${mm}-${dd}`
}

function formatDateTime(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  const hh = String(d.getHours()).padStart(2, '0')
  const mi = String(d.getMinutes()).padStart(2, '0')
  return `${mm}-${dd} ${hh}:${mi}`
}

function adjustmentTypeText(type) {
  return type === 'supplement' ? '补分' : '退分'
}

/** 根据用户名生成头像背景色 */
function avatarColor(name) {
  const colors = ['#D4A24C', '#3B5998', '#3D6B3D', '#B33A3A', '#7B68EE', '#4A90A4', '#C44569', '#574B90']
  if (!name) return colors[0]
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return colors[Math.abs(hash) % colors.length]
}

/**
 * 将微信授权头像的临时文件转为 base64 数据 URL。
 * 临时路径重启后失效，转 base64 才能持久保存（MVP 无对象存储）。
 * @param {string} filePath — chooseAvatar 返回的临时路径
 * @returns {Promise<string>} — data:image/jpeg;base64,... 形式的数据 URL
 */
function avatarToDataUrl(filePath) {
  return new Promise((resolve, reject) => {
    if (!filePath) {
      reject(new Error('no avatar file'))
      return
    }
    if (filePath.indexOf('data:') === 0) {
      resolve(filePath)
      return
    }
    const fs = wx.getFileSystemManager()
    const readAsBase64 = (path) => {
      fs.readFile({
        filePath: path,
        encoding: 'base64',
        success: (res) => resolve('data:image/jpeg;base64,' + res.data),
        fail: (err) => reject(err)
      })
    }
    // 先压缩控制体积；压缩不支持时（如 png）退回原图
    wx.compressImage({
      src: filePath,
      quality: 60,
      compressedWidth: 240,
      success: (res) => readAsBase64(res.tempFilePath),
      fail: () => readAsBase64(filePath)
    })
  })
}

module.exports = {
  formatScore,
  statusText,
  statusClass,
  formatTime,
  formatDateTime,
  adjustmentTypeText,
  avatarColor,
  avatarToDataUrl
}

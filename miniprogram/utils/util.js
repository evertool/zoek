// utils/util.js — 通用工具函数

function formatScore(score) {
  if (score > 0) return '+' + score
  return '' + score
}

function statusText(status) {
  const map = {
    forming: '凑紧脚',
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
 * 将头像文件压缩到 1MB 以内。
 * - 超过 5MB 拒绝上传
 * - 超过 1MB 逐步降低质量压缩
 * - 压缩不支持时退回原图
 * @param {string} filePath — 临时文件路径
 * @returns {Promise<string>} — 压缩后的临时文件路径
 */
function compressAvatar(filePath) {
  return new Promise((resolve, reject) => {
    if (!filePath) {
      reject(new Error('no file'))
      return
    }
    // 微信头像授权返回的是网络路径，不压缩
    if (filePath.indexOf('http://') === 0 || filePath.indexOf('https://') === 0) {
      resolve(filePath)
      return
    }
    wx.getFileInfo({
      filePath: filePath,
      success: (info) => {
        var sizeKB = info.size / 1024
        // 小于 1MB 直接返回
        if (sizeKB <= 1024) {
          resolve(filePath)
          return
        }
        // 大于 5MB 拒绝
        if (sizeKB > 5 * 1024) {
          reject(new Error('FILE_TOO_LARGE'))
          return
        }
        // 逐步压缩：从 quality 80 开始递减
        var tryCompress = function (quality) {
          wx.compressImage({
            src: filePath,
            quality: quality,
            compressedWidth: 480,
            success: function (res) {
              wx.getFileInfo({
                filePath: res.tempFilePath,
                success: function (info2) {
                  if (info2.size / 1024 <= 1024) {
                    resolve(res.tempFilePath)
                  } else if (quality > 20) {
                    tryCompress(quality - 20)
                  } else {
                    // 已尽最大压缩，返回当前结果
                    resolve(res.tempFilePath)
                  }
                },
                fail: function () {
                  resolve(res.tempFilePath)
                }
              })
            },
            fail: function () {
              // 压缩失败，退回原图
              resolve(filePath)
            }
          })
        }
        tryCompress(80)
      },
      fail: function () {
        // 无法获取文件信息，直接尝试压缩
        resolve(filePath)
      }
    })
  })
}

/**
 * 下载网络头像到本地临时文件。
 * 仅在 chooseAvatar 返回 https:// 开头的微信头像 URL 时使用。
 * @param {string} url — 网络头像 URL
 * @returns {Promise<string>} — 本地临时文件路径
 */
function downloadAvatar(url) {
  return new Promise(function (resolve, reject) {
    wx.downloadFile({
      url: url,
      success: function (res) {
        if (res.statusCode === 200) {
          resolve(res.tempFilePath)
        } else {
          reject(new Error('download failed: ' + res.statusCode))
        }
      },
      fail: function (err) {
        reject(err)
      }
    })
  })
}

/**
 * 上传头像到服务器并返回相对路径。
 * 内部自动执行压缩逻辑。
 * - 微信授权头像（网络 URL）：先下载到本地再上传（不压缩）
 * - 本地临时文件：直接压缩后上传
 * @param {string} filePath — chooseAvatar 返回的临时路径或网络 URL
 * @returns {Promise<string>} — 服务器返回的相对路径（如 /uploads/avatars/xxx.jpg）
 */
function uploadAvatar(filePath) {
  return new Promise(function (resolve, reject) {
    // 微信头像授权返回的是网络 URL，需要先下载到本地
    var prepare = (filePath.indexOf('http://') === 0 || filePath.indexOf('https://') === 0)
      ? downloadAvatar(filePath)
      : Promise.resolve(filePath)
    prepare.then(function (localPath) {
      return compressAvatar(localPath)
    }).then(function (compressedPath) {
      var app = getApp()
      wx.uploadFile({
        url: app.globalData.baseURL + '/user/avatar',
        filePath: compressedPath,
        name: 'file',
        header: {
          'Authorization': 'Bearer ' + app.globalData.token
        },
        success: function (res) {
          if (res.statusCode >= 200 && res.statusCode < 300) {
            var data = JSON.parse(res.data)
            resolve(data.avatar_url)
          } else {
            reject(new Error('upload failed: ' + res.statusCode))
          }
        },
        fail: function (err) {
          reject(err)
        }
      })
    }).catch(function (err) {
      reject(err)
    })
  })
}

/**
 * 将后端返回的相对路径头像 URL 转为完整可访问的 URL。
 * 如果已经是完整 URL（https://）或 data URL，直接返回。
 * @param {string} url — 后端返回的 avatar_url
 * @returns {string} — 前端可直接使用的完整 URL
 */
function resolveAvatarURL(url) {
  if (!url) return ''
  if (url.indexOf('http://') === 0 || url.indexOf('https://') === 0 || url.indexOf('data:') === 0) {
    return url
  }
  // 相对路径，拼接服务器基地址（去掉 /api/v1 后缀）
  var app = getApp()
  var base = (app && app.globalData && app.globalData.baseURL) || ''
  var origin = base.replace(/\/api\/v\d+$/, '')
  return origin + url
}

/**
 * 自定义导航页面顶部让位高度（px）：状态栏 + 胶囊按钮 + 少量间距。
 * 用于去掉自定义 Header 后，内容不被系统胶囊遮挡。
 */
function navPadding() {
  try {
    return wx.getMenuButtonBoundingClientRect().bottom + 8
  } catch (e) {
    const info = wx.getWindowInfo ? wx.getWindowInfo() : wx.getSystemInfoSync()
    return (info.statusBarHeight || 20) + 48
  }
}

/**
 * 胶囊按钮盒（px）：top 为胶囊上沿（自绘返回键应与它同一水平线），height 为胶囊高度。
 */
function capsuleBox() {
  try {
    const rect = wx.getMenuButtonBoundingClientRect()
    return { top: rect.top, height: rect.height }
  } catch (e) {
    const info = wx.getWindowInfo ? wx.getWindowInfo() : wx.getSystemInfoSync()
    return { top: (info.statusBarHeight || 20) + 4, height: 32 }
  }
}

/** 兼容 unix 秒 / ISO 字符串两种时间输入 */
function toDate(ts) {
  if (!ts) return null
  if (typeof ts === 'string') {
    const d = new Date(ts.replace(/-/g, '/').replace('T', ' ').split('.')[0])
    return isNaN(d.getTime()) ? new Date(ts) : d
  }
  if (ts < 1e12) return new Date(ts * 1000)
  return new Date(ts)
}

module.exports = {
  formatScore,
  statusText,
  statusClass,
  formatTime,
  formatDateTime,
  adjustmentTypeText,
  avatarColor,
  compressAvatar,
  downloadAvatar,
  uploadAvatar,
  resolveAvatarURL,
  navPadding
}

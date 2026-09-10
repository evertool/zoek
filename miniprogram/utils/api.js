// utils/api.js — 后端 API 请求封装
const app = getApp()

/**
 * 统一请求方法
 * @param {string} method — GET / POST / PUT
 * @param {string} path — API 路径（不含 baseURL）
 * @param {object} data — 请求数据
 * @returns {Promise<object>} — 成功时 resolve data 字段
 */
function request(method, path, data = {}) {
  return new Promise((resolve, reject) => {
    const app = getApp()
    const header = {
      'Content-Type': 'application/json'
    }
    if (app.globalData.token) {
      header['Authorization'] = 'Bearer ' + app.globalData.token
    }

    wx.request({
      url: app.globalData.baseURL + path,
      method,
      data,
      header,
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data)
        } else if (res.statusCode === 401) {
          // token 过期，清除登录状态
          app.logout()
          // 不在请求层自动 reLaunch，由页面守卫处理
          const err = res.data || {}
          err.code = 'UNAUTHORIZED'
          reject(err)
        } else {
          // 业务错误：返回 { code, message, action }
          const err = res.data || {}
          const msg = err.message || '请求失败'
          wx.showToast({ title: msg, icon: 'none', duration: 2500 })
          reject(err)
        }
      },
      fail: (err) => {
        wx.showToast({ title: '服务器出咗啲问题', icon: 'none' })
        reject(err)
      }
    })
  })
}

/** GET 请求 */
function get(path, data = {}) {
  // 将 data 作为 query 参数
  const query = Object.keys(data)
    .filter(k => data[k] !== undefined && data[k] !== null)
    .map(k => `${encodeURIComponent(k)}=${encodeURIComponent(data[k])}`)
    .join('&')
  const pathWithQuery = query ? `${path}?${query}` : path
  return request('GET', pathWithQuery)
}

/** POST 请求 */
function post(path, data = {}) {
  return request('POST', path, data)
}

/** PUT 请求 */
function put(path, data = {}) {
  return request('PUT', path, data)
}

/**
 * 生成唯一 request_id（用于幂等）
 */
function genRequestID() {
  return 'mp-' + Date.now() + '-' + Math.random().toString(36).slice(2, 10)
}

module.exports = {
  request,
  get,
  post,
  put,
  genRequestID
}

// utils/api.js — 后端 API 请求封装
// 注意：这里不要在模块顶层调用 getApp() —— app.js 顶层 require 本文件时 App 尚未创建，
// 拿到的是 undefined。所有请求函数内部都各自 getApp() 取最新实例。

/**
 * 统一请求方法
 * @param {string} method — GET / POST / PUT
 * @param {string} path — API 路径（不含 baseURL）
 * @param {object} data — 请求数据
 * @param {object} opts — 可选：{ silent: true } 不自动弹 toast（页面自行展示错误）
 * @returns {Promise<object>} — 成功时 resolve data 字段
 */
function request(method, path, data = {}, opts = {}) {
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
          const err = res.data || {}
          // 后端把「微信登录失败」也归到 401，但那是登录动作本身失败（code 换 token 失败），
          // 不是凭证过期：不能顺手登出、也不能把 code 改写成 UNAUTHORIZED，
          // 否则真实失败原因（WX_LOGIN_FAILED + action:RETRY）会被吃掉，排查时只剩「未登录」。
          // 这里也不弹 toast —— 调用方（app.login(silent)）自己决定要不要提示，
          // 静默登录失败必须无打扰，否则首页一进就蹦报错弹窗。
          if (err.code === 'WX_LOGIN_FAILED') {
            reject(err)
            return
          }
          // token 过期，清除登录状态
          app.logout()
          // 不在请求层自动 reLaunch，由页面守卫处理
          err.code = 'UNAUTHORIZED'
          reject(err)
        } else {
          // 业务错误：返回 { code, message, action }
          const err = res.data || {}
          const msg = err.message || '请求失败'
          if (!opts.silent) {
            wx.showToast({ title: msg, icon: 'none', duration: 2500 })
          }
          reject(err)
        }
      },
      fail: (err) => {
        if (!opts.silent) {
          wx.showToast({ title: '服务器出咗啲问题', icon: 'none' })
        }
        reject(err)
      }
    })
  })
}

/** GET 请求 */
function get(path, data = {}, opts = {}) {
  // 将 data 作为 query 参数
  const query = Object.keys(data)
    .filter(k => data[k] !== undefined && data[k] !== null)
    .map(k => `${encodeURIComponent(k)}=${encodeURIComponent(data[k])}`)
    .join('&')
  const pathWithQuery = query ? `${path}?${query}` : path
  return request('GET', pathWithQuery, {}, opts)
}

/** POST 请求 */
function post(path, data = {}, opts = {}) {
  return request('POST', path, data, opts)
}

/** PUT 请求 */
function put(path, data = {}, opts = {}) {
  return request('PUT', path, data, opts)
}

/**
 * 二进制 GET 请求（图片 / 文件类接口，如小程序码）
 * 与 request 的区别：responseType 为 arraybuffer，成功时 resolve ArrayBuffer
 * 之所以不复用 request：默认 responseType 会把 PNG 当文本解析，图片必然损坏
 * @param {string} path — API 路径（不含 baseURL）
 * @param {object} opts — 可选：{ silent: true } 不自动弹 toast
 * @returns {Promise<ArrayBuffer>}
 */
function getBinary(path, opts = {}) {
  return new Promise((resolve, reject) => {
    const app = getApp()
    const header = {}
    if (app.globalData.token) {
      header['Authorization'] = 'Bearer ' + app.globalData.token
    }
    wx.request({
      url: app.globalData.baseURL + path,
      method: 'GET',
      responseType: 'arraybuffer',
      header,
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300 && res.data && res.data.byteLength) {
          resolve(res.data)
          return
        }
        if (res.statusCode === 401) {
          app.logout()
          const err = { code: 'UNAUTHORIZED', message: '登录已过期' }
          if (!opts.silent) wx.showToast({ title: err.message, icon: 'none' })
          reject(err)
          return
        }
        const err = { code: 'BINARY_FAILED', message: opts.errMsg || '图片加载失败' }
        if (!opts.silent) wx.showToast({ title: err.message, icon: 'none', duration: 2500 })
        reject(err)
      },
      fail: (err) => {
        if (!opts.silent) wx.showToast({ title: '服务器出咗啲问题', icon: 'none' })
        reject(err)
      }
    })
  })
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
  getBinary,
  genRequestID
}

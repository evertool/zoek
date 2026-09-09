// utils/guard.js — 登录与资料完善全局守卫
// 规则：未登录 → 首页登录页；已登录但缺头像/昵称 → 首页强制完善资料页。
// 通过分享/邀请链接进入的页面被拦下时记录原路径，完成登录和资料后自动回去继续。

/** 取当前页面完整路由（含参数） */
function currentRoute() {
  const pages = getCurrentPages()
  const cur = pages[pages.length - 1]
  if (!cur || !cur.route) return ''
  const opts = cur.options || {}
  const qs = Object.keys(opts)
    .filter((k) => opts[k] !== undefined && opts[k] !== '')
    .map((k) => k + '=' + encodeURIComponent(opts[k]))
    .join('&')
  return '/' + cur.route + (qs ? '?' + qs : '')
}

/** 是否已登录且资料完善 */
function pass() {
  const app = getApp()
  return !!app.globalData.token && !app.checkProfileNeeded()
}

/**
 * 非首页守卫：未通过时弹回首页登录/完善资料。
 * @param {boolean} remember — 是否记录当前页，登录完善后自动回来
 * （join/room/score/adjustment/detail/settlement 等带上下文的页面传 true，
 *   tab 页不传，登录后直接落在牌局页）
 * 返回 false 时调用方页面逻辑必须立即 return。
 */
function ensure(remember) {
  if (pass()) return true
  const app = getApp()
  const route = remember ? currentRoute() : ''
  if (route && route.indexOf('/pages/index/index') !== 0) {
    app.globalData.pendingRoute = route
  }
  wx.reLaunch({ url: '/pages/index/index' })
  return false
}

module.exports = { pass, ensure, currentRoute }

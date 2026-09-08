// pages/leaderboard/leaderboard.js — 雀友榜 + 个人数据
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

// 安全加载 lottie
let lottie = null
try {
  lottie = require('lottie-miniprogram')
} catch (e) {
  console.warn('lottie-miniprogram not available, trend chart disabled')
}
const chart = require('../../utils/lottie-chart')

Page({
  data: {
    loading: true,
    isLoggedIn: false,
    stats: null,
    entries: [],
    days: 30,
    minGames: 3,
    trendCount: 0
  },

  onShow() {
    const isLoggedIn = !!app.globalData.token
    this.setData({ isLoggedIn })
    if (isLoggedIn) {
      this.loadAll()
    } else {
      this.setData({ loading: false })
    }
  },

  onHide() {
    this.destroyTrendAnim()
  },

  onUnload() {
    this.destroyTrendAnim()
  },

  onPullDownRefresh() {
    if (!app.globalData.token) {
      wx.stopPullDownRefresh()
      return
    }
    this.loadAll().then(() => wx.stopPullDownRefresh())
  },

  loadAll() {
    this.setData({ loading: true })
    return Promise.all([
      api.get('/user/stats'),
      api.get('/leaderboard')
    ]).then(([stats, lb]) => {
      // 排行榜：服务端已按合格在前排序，前端只补展示字段
      let rank = 0
      const entries = (lb.leaderboard || []).map(e => {
        if (e.qualified) rank++
        return {
          ...e,
          displayRank: e.qualified ? rank : 0,
          winRateText: Math.round(e.win_rate) + '%',
          top3RateText: Math.round(e.top3_rate) + '%',
          avgRankText: e.avg_rank.toFixed(1),
          avatarColor: util.avatarColor(e.nickname)
        }
      })
      this.setData({
        stats,
        entries,
        days: lb.days || 30,
        minGames: lb.min_games || 3,
        trendCount: (stats.trend || []).length,
        loading: false
      })
      // 等 canvas 挂载后再初始化动画
      setTimeout(() => this.renderTrendChart(), 150)
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  // ===== Lottie 得分走势图 =====
  renderTrendChart() {
    if (!lottie) return
    const totals = (this.data.stats && this.data.stats.trend || []).map(t => t.total)
    const animationData = chart.buildLineChart(totals, { width: 320, height: 150 })
    if (!animationData) return
    const query = wx.createSelectorQuery().in(this)
    query.select('#lottie-trend').fields({ node: true, size: true }).exec(res => {
      if (!res || !res[0] || !res[0].node) return
      const canvas = res[0].node
      const ctx = canvas.getContext('2d')
      const dpr = wx.getSystemInfoSync().pixelRatio
      canvas.width = res[0].width * dpr
      canvas.height = res[0].height * dpr
      ctx.scale(dpr, dpr)
      this.destroyTrendAnim()
      this._trendAnim = lottie.loadAnimation({
        loop: false,
        autoplay: true,
        animationData,
        rendererSettings: {
          context: ctx,
          clearCanvas: true
        }
      })
    })
  },

  destroyTrendAnim() {
    if (this._trendAnim && this._trendAnim.destroy) {
      try { this._trendAnim.destroy() } catch (e) {}
      this._trendAnim = null
    }
  },

  goLogin() {
    this.doLogin()
  },

  doLogin() {
    wx.showLoading({ title: '登录中...' })
    app.login().then(() => {
      wx.hideLoading()
      this.setData({ isLoggedIn: true })
      this.loadAll()
    }).catch(() => {
      wx.hideLoading()
    })
  }
})

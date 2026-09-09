// pages/leaderboard/leaderboard.js — 雀友榜 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    loading: true,
    isLoggedIn: false,
    stats: null,
    entries: [],
    days: 30,
    currentPeriod: 30,
    minGames: 3,
    nickname: '',
    avatarURL: '',
    showToast: false,
    toastMsg: ''
  },

  onShow() {
    const isLoggedIn = !!app.globalData.token
    this.setData({
      isLoggedIn,
      nickname: app.globalData.nickname || '',
      avatarURL: app.globalData.avatarURL || ''
    })
    if (isLoggedIn) {
      this.loadAll()
    } else {
      this.setData({ loading: false })
    }
  },

  onPullDownRefresh() {
    if (!app.globalData.token) {
      wx.stopPullDownRefresh()
      return
    }
    this.loadAll().then(() => wx.stopPullDownRefresh())
  },

  switchPeriod(e) {
    const period = Number(e.currentTarget.dataset.period)
    this.setData({ currentPeriod: period, loading: true })
    this.loadAll()
    this.showToast('已切换至: ' + (period === 30 ? '近 30 天' : period === 7 ? '近 7 天' : '全部'))
  },

  loadAll() {
    this.setData({ loading: true })
    return Promise.all([
      api.get('/user/stats'),
      api.get('/leaderboard')
    ]).then(([stats, lb]) => {
      let rank = 0
      const entries = (lb.leaderboard || []).map(e => {
        if (e.qualified) rank++
        const totalScore = e.total_score || 0
        const scoreClass = totalScore >= 0 ? 'positive' : 'negative'
        return {
          ...e,
          displayRank: e.qualified ? rank : 0,
          winRateText: Math.round(e.win_rate) + '%',
          top3RateText: Math.round(e.top3_rate) + '%',
          avgRankText: e.avg_rank ? e.avg_rank.toFixed(1) : '0.0',
          avatarColor: util.avatarColor(e.nickname),
          total_score: totalScore
        }
      })
      const myStats = {
        ...stats,
        my_rank: entries.find(e => e.is_self)?.displayRank || 0,
        total_score: stats.total_score || 0,
        active_text: stats.games > 0 ? '本周期活跃 · 雀艺渐入佳境' : '未参与牌局',
        best_streak: stats.best_streak || 0,
        badges: stats.badges || []
      }
      this.setData({
        stats: myStats,
        entries,
        days: lb.days || 30,
        minGames: lb.min_games || 3,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  inviteFriend(e) {
    const name = e.currentTarget.dataset.name
    this.showToast('已向 ' + name + ' 发起开台通知')
  },

  showToast(msg) {
    this.setData({ showToast: true, toastMsg: msg })
    if (this._toastTimer) clearTimeout(this._toastTimer)
    this._toastTimer = setTimeout(() => {
      this.setData({ showToast: false })
    }, 2200)
  },

  doLogin() {
    wx.showLoading({ title: '登录中...' })
    app.login().then(() => {
      wx.hideLoading()
      this.setData({
        isLoggedIn: true,
        nickname: app.globalData.nickname,
        avatarURL: app.globalData.avatarURL
      })
      this.loadAll()
    }).catch(() => {
      wx.hideLoading()
    })
  }
})

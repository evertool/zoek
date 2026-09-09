// pages/leaderboard/leaderboard.js — 雀友榜 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

Page({
  data: {
    loading: true,
    isLoggedIn: false,
    stats: null,
    entries: [],
    boardTab: 'score', // score=积分榜 / rank=排位榜（按段位星级排序）
    days: 30,
    currentPeriod: 30,
    minGames: 3,
    nickname: '',
    avatarURL: '',
    showToast: false,
    toastMsg: '',
    navPadding: 0
  },

  onLoad() {
    // 顶部无导航条，内容需让出状态栏 + 胶囊按钮高度
    this.setData({ navPadding: util.navPadding() })
  },

  onShow() {
    // 未登录/资料不全时弹回首页登录或完善资料
    if (!guard.ensure()) return
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

  // 时间筛选（近30天/近7天/全部）——真实传参给后端
  switchPeriod(e) {
    const period = Number(e.currentTarget.dataset.period)
    if (period === this.data.currentPeriod) return
    this.setData({ currentPeriod: period, loading: true })
    this.loadAll()
  },

  // 积分榜 / 排位榜切换（排位榜按累计星级排序，本地排序）
  switchBoard(e) {
    const tab = e.currentTarget.dataset.tab
    if (tab === this.data.boardTab) return
    this.setData({ boardTab: tab })
    this.applyBoard(tab)
  },

  applyBoard(tab) {
    var entries = this.data.entries.slice()
    if (tab === 'rank') {
      entries.sort(function(a, b) { return (b.rank_stars || 0) - (a.rank_stars || 0) })
    }
    entries = entries.map(function(e, idx) {
      return { ...e, displayRank: idx + 1 }
    })
    var myRank = 0
    entries.forEach(function(e) { if (e.is_self) myRank = e.displayRank })
    var stats = this.data.stats ? { ...this.data.stats, my_rank: myRank } : this.data.stats
    this.setData({ entries: entries, stats: stats })
  },

  loadAll() {
    this.setData({ loading: true })
    return Promise.all([
      api.get('/user/stats'),
      api.get('/leaderboard', { days: this.data.currentPeriod })
    ]).then(([stats, lb]) => {
      const entries = (lb.leaderboard || []).map(e => {
        const totalScore = e.total_score || 0
        return {
          ...e,
          displayRank: 0,
          winRateText: Math.round(e.win_rate) + '%',
          top3RateText: Math.round(e.top3_rate) + '%',
          avgRankText: e.avg_rank ? e.avg_rank.toFixed(1) : '0.0',
          avatarColor: util.avatarColor(e.nickname),
          avatar_url: util.resolveAvatarURL(e.avatar_url || ''),
          total_score: totalScore
        }
      })
      this.setData({
        stats: { ...stats, active_text: stats.games > 0 ? '本周期活跃 · 雀艺渐入佳境' : '未参与牌局' },
        entries: entries,
        days: lb.days || 0,
        minGames: lb.min_games || 3,
        loading: false
      })
      this.applyBoard(this.data.boardTab)
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  // ===== 约开台（任务7）：点击先确保有一张自己的台，再分享邀请链接 =====
  onInviteTap() {
    this._inviteReady = null
    if (this._preparing) return
    this._preparing = true
    this._inviteReady = api.get('/games/active').then(res => {
      const games = res.games || []
      if (games.length) {
        // 已有台：直接分享现有台（join 页支持 game_id 入台）
        return { gameId: games[0].game_id }
      }
      return api.post('/games', { name: '', request_id: api.genRequestID() }).then(created => {
        wx.showToast({ title: '已为你开好新台', icon: 'success' })
        return { gameId: created.game_id, inviteToken: created.invite_token }
      })
    }).catch(err => {
      if (err && err.code === 'ALREADY_IN_GAME' && err.game_id) {
        return { gameId: err.game_id }
      }
      wx.showToast({ title: (err && err.message) || '开台失败，请重试', icon: 'none' })
      return null
    }).then(game => {
      this._preparing = false
      return game
    })
  },

  onShareAppMessage() {
    const ready = this._inviteReady || Promise.resolve(null)
    return ready.then(game => {
      if (game && game.gameId) {
        return {
          title: '约起！开咗张台，等你上桌',
          path: '/pages/join/join?invite_token=' + (game.inviteToken || game.gameId)
        }
      }
      return { title: '得闲开台 — 粤语麻雀记分神器', path: '/pages/index/index' }
    })
  },

  goRankPage() {
    wx.navigateTo({ url: '/pages/rank/rank' })
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

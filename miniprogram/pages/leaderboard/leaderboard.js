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
    boardTab: 'score', // score=积分榜 / rank=段位榜（按段位星级排序）
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

  // 积分榜 / 段位榜切换：列表从原始榜单重建；我的卡片切换统计口径
  // （段位榜只算满 4 人的排位局，积分榜算同窗口全部对局）
  switchBoard(e) {
    const tab = e.currentTarget.dataset.tab
    if (tab === this.data.boardTab) return
    this.setData({ boardTab: tab })
    this.applyBoard(tab)
  },

  applyBoard(tab) {
    var entries = (this._rawEntries || []).slice()
    if (tab === 'rank') {
      // 段位榜：按排位累计星级降序，星级相同按胜场
      entries.sort(function(a, b) {
        return (b.rank_stars || 0) - (a.rank_stars || 0) || (b.wins || 0) - (a.wins || 0)
      })
    }
    entries = entries.map(function(e, idx) {
      return { ...e, displayRank: idx + 1 }
    })
    var myRank = 0
    entries.forEach(function(e) { if (e.is_self) myRank = e.displayRank })
    var stats = this.data.stats ? { ...this.data.stats, my_rank: myRank } : this.data.stats
    this.setData({ entries: entries, stats: stats })

    if (tab === 'rank') {
      this.applyRankedCard()
    } else {
      this.applyScoreCard()
    }
  },

  /** 积分榜口径：榜单中「我」的条目（同窗口全部对局），未入榜回退全量统计 */
  applyScoreCard() {
    var mine = (this._rawEntries || []).find(function(e) { return e.is_self })
    var base = mine || this._userStats || {}
    this.setData({
      stats: {
        ...this.data.stats,
        my_rank: (this.data.stats && this.data.stats.my_rank) || 0,
        games: base.games || 0,
        wins: base.wins || 0,
        win_rate: Math.round(base.win_rate || 0),
        best_streak: base.best_streak || 0,
        best_score: base.best_score || 0,
        total_score: base.total_score || 0,
        active_text: (base.games || 0) > 0 ? '本周期活跃 · 雀艺渐入佳境' : '未参与牌局'
      }
    })
  },

  /** 段位榜口径：/rank/me（仅满 4 人排位局） */
  applyRankedCard() {
    api.get('/rank/me').then(res => {
      if (this.data.boardTab !== 'rank') return // 用户已切回积分榜，丢弃
      this.setData({
        stats: {
          ...this.data.stats,
          games: res.total_games || 0,
          wins: res.wins || 0,
          win_rate: Math.round(res.win_rate || 0),
          best_streak: res.best_streak || 0,
          best_score: res.best_score || 0,
          total_score: res.points || 0,
          active_text: '段位赛绩 · 满4人局计入排位'
        }
      })
    }).catch(function() {})
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
      this._rawEntries = entries // 保存服务端原始排序，切换榜单时从这里重建
      this._userStats = stats   // 全量个人统计（积分榜未入榜时兜底）

      this.setData({
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

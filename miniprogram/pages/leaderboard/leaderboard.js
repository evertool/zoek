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
    minGames: 1,
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
    // 等待 app onLaunch 异步校验完成
    guard.ensureAsync().then(ok => {
      if (!ok) return
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
    })
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
  // （两个榜的列表口径一致：仅计满 4 人排位局；积分榜卡片取榜单内自己条目，段位榜卡片走 /rank/me）
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

  /**
   * 卡片战绩数字：两个榜统一取榜单内「我」的条目（同窗口、满4人口径），
   * 保证积分榜/段位榜切换时场次/胜场/胜率/连胜/单场最高完全一致；
   * 未入榜（窗口内无满4人同台局）显示 0，不回退 /user/stats 全量统计。
   */
  buildCardStats() {
    var mine = (this._rawEntries || []).find(function(e) { return e.is_self })
    var myRank = 0
    ;(this._rawEntries || []).forEach(function(e, i) { if (e.is_self) myRank = i + 1 })
    var has = !!mine
    return {
      my_rank: myRank,
      games: has ? (mine.games || 0) : 0,
      wins: has ? (mine.wins || 0) : 0,
      draws: has ? (mine.draws || 0) : 0,
      win_rate: has ? Math.round(mine.win_rate || 0) : 0,
      best_streak: has ? (mine.best_streak || 0) : 0,
      best_score: has ? (mine.best_score || 0) : 0,
      total_score: has ? (mine.total_score || 0) : 0,
      active_text: has ? '本周期活跃 · 雀艺渐入佳境' : '暂无满4人同台局'
    }
  },

  /** 积分榜卡片：净胜分为主体，段位 chip 用榜单条目自带的段位 */
  applyScoreCard() {
    var mine = (this._rawEntries || []).find(function(e) { return e.is_self }) || {}
    var st = this.buildCardStats()
    this.setData({
      stats: {
        ...this.data.stats,
        ...st,
        tier_short: mine.tier_short || '',
        tier_icon: mine.tier_icon || '',
        stars: mine.stars || 0,
        scoreText: util.formatWan(st.total_score)
      }
    })
  },

  /** 段位榜卡片：战绩数字与积分榜完全一致；仅段位图标/星级/排位积分来自 /rank/me */
  applyRankedCard() {
    var mine = (this._rawEntries || []).find(function(e) { return e.is_self }) || {}
    var st = this.buildCardStats()
    this.setData({
      stats: {
        ...this.data.stats,
        ...st,
        tier_short: mine.tier_short || '',
        tier_icon: mine.tier_icon || '',
        stars: mine.stars || 0,
        scoreText: util.formatWan(st.total_score)
      }
    })
    api.get('/rank/me').then(res => {
      if (this.data.boardTab !== 'rank') return // 用户已切回积分榜，丢弃
      this.setData({
        stats: {
          ...this.data.stats,
          tier_short: (res.tier && res.tier.tier_short) || '',
          tier_icon: res.tier ? util.tierIcon(res.tier.tier_index, res.tier.is_peak) : '',
          stars: (res.tier && res.tier.stars_in_tier) || 0,
          scoreText: util.formatWan(res.points || 0) // 主体数字换成排位积分
        }
      })
    }).catch(function() {})
  },

  loadAll() {
    this.setData({ loading: true })
    // 卡片与列表统一用 /leaderboard 满4人口径（未入榜时卡片显示 0 + 原因说明）
    return api.get('/leaderboard', { days: this.data.currentPeriod }).then(lb => {
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
          tier_icon: util.tierIcon(e.tier_index, e.is_peak),
          scoreText: util.formatWan(totalScore),
          total_score: totalScore
        }
      })
      this._rawEntries = entries // 保存服务端原始排序，切换榜单时从这里重建

      this.setData({
        days: lb.days || 0,
        minGames: lb.min_games || 1,
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
          title: '走起！开咗张台，就等你喇',
          path: '/pages/join/join?invite_token=' + (game.inviteToken || game.gameId)
        }
      }
      return { title: '得闲开台 — 粤式麻雀记分助手', path: '/pages/index/index' }
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

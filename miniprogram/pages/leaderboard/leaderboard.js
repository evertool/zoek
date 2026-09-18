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
    // 从「可约台」的分享面板返回：进自己的台房间
    // （微信对 open-type="share" 没有分享成功回调，只能在面板关闭、页面重新 onShow 时兜底跳转）
    if (this._pendingEnterRoom) {
      this._pendingEnterRoom = false
      this.enterRoom()
    }
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

  // 积分榜 / 段位榜切换：列表从原始榜单重建；卡片只换名次（战绩数字与积分两榜一致）
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
    // 名次必须跟着「当前榜」的排序数出来，再交给卡片——卡片里不能自己按原始顺序再算一次
    var myRank = 0
    entries.forEach(function(e) { if (e.is_self) myRank = e.displayRank })
    this.setData({ entries: entries })
    this.applyCard(myRank)
  },

  /**
   * 卡片战绩：两个榜完全一致（场次/胜场/胜率/连胜/单场最高/净胜分），
   * 只有「我的名次」随榜单排序变（积分榜按净胜分 / 段位榜按排位星级）。
   * 未入榜（窗口内无满4人同台局）显示 0，不回退 /user/stats 全量统计。
   */
  buildCardStats(myRank) {
    var mine = (this._rawEntries || []).find(function(e) { return e.is_self })
    var has = !!mine
    return {
      my_rank: myRank || 0,
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

  /** 卡片：主体数字固定为「净胜分」（窗口内），段位 chip 用榜单条目自带的段位 */
  applyCard(myRank) {
    var mine = (this._rawEntries || []).find(function(e) { return e.is_self }) || {}
    var st = this.buildCardStats(myRank)
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

  // ===== 约开台：点击先确保有一张自己的台 → 拉起分享邀请 → 分享面板关闭后进房间（onShow 兜底） =====
  onInviteTap() {
    // 标记「这次点击是为了约台」：分享面板关闭、页面 onShow 时据此跳进房间
    this._pendingEnterRoom = true
    if (this._preparing) return // 连点：沿用上一次的 _inviteReady，别把它清空
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

  // 进自己的台房间：台可能还在创建中，等 _inviteReady 就绪再跳
  enterRoom() {
    const ready = this._inviteReady || Promise.resolve(null)
    Promise.resolve(ready).then(game => {
      if (game && game.gameId) {
        wx.navigateTo({ url: '/pages/room/room?game_id=' + game.gameId })
      }
    }).catch(function() {})
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

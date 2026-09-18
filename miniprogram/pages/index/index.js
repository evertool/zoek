// pages/index/index.js — 首页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

const WINDS = ['東', '南', '西', '北']
const WIND_CLASSES = ['east', 'south', 'west', 'north']

// 最近战绩的名次徽章（1~4 名：金/银/铜/铁）
const RANK_BADGE_ICONS = {
  1: '/assets/icons/rank_1_gold_badge.svg',
  2: '/assets/icons/rank_2_silver_badge.svg',
  3: '/assets/icons/rank_3_bronze_badge.svg',
  4: '/assets/icons/rank_4_iron_badge.svg'
}

Page({
  data: {
    games: [],
    recent: [],
    loading: true,
    loginFailed: false,
    profileSheet: false,
    navPadding: 0
  },

  onLoad() {
    // 顶部无导航条，内容需让出状态栏 + 胶囊按钮高度
    this.setData({ navPadding: util.navPadding() })
  },

  onShow() {
    // 等待 app onLaunch 中的静默登录/校验完成；打开即可浏览，不再有全屏登录/资料闸门
    app.ready().then(() => {
      // 守卫记下的目标页（分享/扫码直入被登录拦下的场景）优先回去
      if (app.globalData.pendingRoute) {
        this.goPendingRoute()
        return
      }
      if (app.globalData.token) {
        this.setData({ loginFailed: false })
        this.loadGames()
        this.loadRecent()
        this.startPolling()
      } else {
        // 静默登录失败（网络波动）：展示内容 + 兜底重试条，不阻塞浏览
        this.setData({ loading: false, loginFailed: true })
      }
    })
  },

  /** 当前牌局实时刷新：雀友进来后头像自动更新 */
  startPolling() {
    this.stopPolling()
    this._poll = setInterval(() => this.loadGames(), 5000)
  },

  stopPolling() {
    if (this._poll) {
      clearInterval(this._poll)
      this._poll = null
    }
  },

  onHide() {
    this.stopPolling()
  },

  /** 静默登录失败后的手动重试 */
  retryLogin() {
    var that = this
    this.setData({ loginFailed: false, loading: true })
    app.login(true).then(function() {
      that.loadGames()
      that.loadRecent()
      that.startPolling()
    }).catch(function() {
      that.setData({ loading: false, loginFailed: true })
    })
  },

  onPullDownRefresh() {
    if (app.globalData.token) {
      this.loadGames().then(() => {
        wx.stopPullDownRefresh()
      })
    } else {
      wx.stopPullDownRefresh()
    }
  },

  loadGames() {
    return api.get('/games/active').then(res => {
    const games = (res.games || []).map(g => {
      // 风位必须按真实 seat 推导（后端只返回 wind 文字 + seat，无 windClass）；
      // 按数组下标分配会在座位不连续时与房间页东南西北错位
      const players = (g.players || []).map(p => {
        const sIdx = (p.seat || 0) - 1
        const wIdx = (sIdx >= 0 && sIdx < 4) ? sIdx : -1
        return {
          ...p,
          wind: wIdx >= 0 ? WINDS[wIdx] : (p.wind || ''),
          windClass: wIdx >= 0 ? WIND_CLASSES[wIdx] : '',
          avatar_url: util.resolveAvatarURL(p.avatar_url || '')
        }
      })
      // 按 seat 落位成 4 格，空位留占位，与房间页座位布局一致
      const seatPlayers = [null, null, null, null]
      let anySeated = false
      players.forEach(p => {
        const sIdx = (p.seat || 0) - 1
        if (sIdx >= 0 && sIdx < 4) {
          seatPlayers[sIdx] = { ...p, empty: false }
          anySeated = true
        }
      })
      // 兜底：后端未返回任何 seat 时按下标铺排，避免玩家消失
      if (!anySeated) {
        players.slice(0, 4).forEach((p, i) => { seatPlayers[i] = { ...p, empty: false } })
      }
      const slots = seatPlayers.map((p, i) => p || {
        seat: i + 1,
        seatIdx: i,
        empty: true,
        wind: WINDS[i],
        windClass: WIND_CLASSES[i]
      })
      var durationText = ''
      if (g.duration_minutes > 0) {
        durationText = g.duration_minutes >= 60
          ? '已打 ' + Math.floor(g.duration_minutes / 60) + ' 小时 ' + (g.duration_minutes % 60) + ' 分'
          : '已打 ' + g.duration_minutes + ' 分钟'
      } else {
        durationText = '刚开台'
      }
      return {
        ...g,
        players,
        seatPlayers: slots,
        statusText: util.statusText(g.status),
        statusClass: util.statusClass(g.status),
        durationText: durationText,
        canInvite: (g.player_count || 0) < 4 && (g.completed_rounds || 0) === 0
      }
    })
      this.setData({ games, loading: false })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  /** 最近战绩（真实数据，最近 3 场） */
  loadRecent() {
    api.get('/games/history', { page: 1, page_size: 3 }).then(res => {
      const recent = (res.games || []).map(g => ({
        game_id: g.game_id,
        name: g.name || '未命名牌局',
        statusText: util.statusText(g.status),
        timeText: this.formatRecentTime(g.ended_at || g.created_at),
        playersText: (g.players || []).filter(function(p) { return !p.is_me }).map(function(p) { return p.nickname }).slice(0, 4).join('、'),
        my_score: g.my_score || 0,
        my_rank: g.my_rank || 0,
        // 名次徽章：1~4 名对应 金/银/铜/铁，其余走中性兜底
        rankIcon: RANK_BADGE_ICONS[g.my_rank] || '/assets/icons/rank-9-novice.svg',
        resultText: g.result === 'win' ? '胜' : (g.result === 'lose' ? '负' : '平')
      }))
      this.setData({ recent })
    }).catch(function() {})
  },

  // 最近战绩只展示时间（HH:MM），不再带日期
  formatRecentTime(ts) {
    var d = util.toDate(ts)
    if (!d) return ''
    return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
  },

  goHistory() {
    wx.switchTab({ url: '/pages/history/history' })
  },

  goRecentDetail(e) {
    wx.navigateTo({ url: '/pages/game-detail/game-detail?game_id=' + e.currentTarget.dataset.id })
  },

  /** 卡上邀请雀友：分享当前台的邀请链接 */
  onCardInvite(e) {
    this._shareGameId = e.currentTarget.dataset.id
  },

  onShareAppMessage() {
    if (this._shareGameId) {
      return {
        title: '开咗张台，快啲埋位！',
        path: '/pages/join/join?invite_token=' + this._shareGameId
      }
    }
    return { title: '得闲开台 — 粤语麻雀记分神器', path: '/pages/index/index' }
  },

  // ===== 登录兜底（静默登录已在 app 启动时自动完成，此处仅守卫回跳） =====
  /** 登录/完善资料完成后回到进入前的页面；返回 false 表示没有待跳页 */
  goPendingRoute() {
    const target = app.globalData.pendingRoute
    app.globalData.pendingRoute = ''
    if (target) {
      wx.reLaunch({ url: target })
      return true
    }
    return false
  },

  // ===== 完善资料弹窗（仅开台动作触发，可关闭） =====
  onProfileSaved() {
    this.setData({ profileSheet: false })
    if (this._pendingCreate) {
      this._pendingCreate = false
      this.doCreate()
    }
  },

  onProfileClose() {
    // 用户拒绝完善：关闭弹窗留在首页，不打扰
    this._pendingCreate = false
    this.setData({ profileSheet: false })
  },

  // ===== 列表操作 =====
  // PRD v1.0 §4.2-A: 开台零摩擦——点按钮直接创建牌桌并进入房间，不填台名
  // 资料未完善时弹出可关闭的完善弹窗，保存后自动继续开台
  goCreate() {
    var that = this
    if (!app.globalData.token) {
      // 静默登录兜底（正常情况下启动时已完成）
      app.login(true).then(function() {
        that.goCreate()
      }).catch(function() {
        wx.showToast({ title: '网络开小差，请重试', icon: 'none' })
      })
      return
    }
    if (app.checkProfileNeeded()) {
      this._pendingCreate = true
      this.setData({ profileSheet: true })
      return
    }
    this.doCreate()
  },

  doCreate() {
    if (this._creating) return
    this._creating = true
    wx.showLoading({ title: '开台中...' })
    // silent：ALREADY_IN_GAME 时页面自己弹「去看看」的 modal，api 层别再叠一个 toast（会双提示）
    api.post('/games', {
      name: '',
      request_id: api.genRequestID()
    }, { silent: true }).then(res => {
      wx.hideLoading()
      this._creating = false
      wx.navigateTo({
        url: `/pages/room/room?game_id=${res.game_id}&invite_token=${res.invite_token}`
      })
    }).catch(err => {
      wx.hideLoading()
      this._creating = false
      if (err && err.code === 'ALREADY_IN_GAME' && err.game_id) {
        wx.showModal({
          title: '你已有一张进行中的牌台',
          content: '同时只能开一张台，先去处理当前牌台',
          showCancel: false,
          confirmText: '去看看',
          success: () => {
            wx.navigateTo({ url: '/pages/room/room?game_id=' + err.game_id })
          }
        })
      } else {
        wx.showToast({ title: (err && err.message) || '开台失败，请重试', icon: 'none' })
      }
    })
  },

  goRoom(e) {
    const gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/room/room?game_id=${gameID}` })
  },

  doDelete(e) {
    const gameID = e.currentTarget.dataset.id
    wx.showModal({
      title: '删除房间',
      content: '确定要删除这个房间吗？',
      confirmColor: '#B33A3A',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${gameID}/cancel`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已删除', icon: 'success' })
            this.loadGames()
          })
        }
      }
    })
  }
})

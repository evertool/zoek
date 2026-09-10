// pages/history/history.js — 开台手账 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

Page({
  data: {
    games: [],
    groups: [],
    overview: { games: 0, score: 0, win_rate: 0 },
    total: 0,
    loading: true,
    isLoggedIn: false,
    page: 1,
    pageSize: 20,
    hasMore: true,
    // 日期筛选（设计稿 30：近30 / 近7天 / 全部）
    days: 0,
    dayTabs: [
      { label: '近30天', days: 30 },
      { label: '近7天', days: 7 },
      { label: '全部', days: 0 }
    ],
    // 标签筛选：按本场名次 胜/平/负
    result: '',
    resultTabs: [
      { label: '全部对局', value: '' },
      { label: '胜', value: 'win' },
      { label: '平', value: 'draw' },
      { label: '负', value: 'lose' }
    ],
    overviewLabel: '全部概览',
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
      this.setData({ isLoggedIn: !!app.globalData.token })
      if (app.globalData.token && !this.data.games.length) {
        this.reload()
      } else if (!app.globalData.token) {
        this.setData({ loading: false })
      }
    })
  },

  onPullDownRefresh() {
    this.reload()
    wx.stopPullDownRefresh()
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.loadHistory()
    }
  },

  // 重置分页并按当前筛选重新加载
  reload() {
    this.setData({ games: [], groups: [], page: 1, hasMore: true, loading: true })
    this.loadHistory()
  },

  onDayFilter(e) {
    var days = Number(e.currentTarget.dataset.days)
    if (days === this.data.days) return
    var label = this.data.dayTabs.filter(function(t) { return t.days === days })[0]
    this.setData({ days: days, overviewLabel: label ? label.label + '概览' : '全部概览' })
    this.reload()
  },

  onResultFilter(e) {
    var result = e.currentTarget.dataset.result
    if (result === this.data.result) return
    this.setData({ result: result })
    this.reload()
  },

  loadHistory() {
    if (!app.globalData.token) return Promise.resolve()
    this.setData({ loading: true })
    return api.get('/games/history', {
      page: this.data.page,
      page_size: this.data.pageSize,
      days: this.data.days,
      result: this.data.result
    }).then(res => {
      const games = (res.games || []).map(g => this.decorate(g))
      const allGames = this.data.games.concat(games)
      const hasMore = !!res.has_more && allGames.length < res.total
      this.setData({
        games: allGames,
        total: res.total || 0,
        hasMore: hasMore,
        page: this.data.page + 1,
        loading: false
      })
      this.buildGroups(allGames)
      // 概览用整个筛选结果集的汇总（分页不影响数字）
      const sum = res.summary || {}
      this.setData({
        overview: {
          games: sum.games || 0,
          score: sum.net || 0,
          win_rate: sum.win_rate || 0
        }
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  // 把接口数据转成视图字段
  decorate(g) {
    var resultMap = { win: '胜', draw: '平', lose: '负' }
    var duration = '—'
    if (g.duration_minutes > 0) {
      duration = g.duration_minutes >= 60
        ? Math.floor(g.duration_minutes / 60) + '时' + (g.duration_minutes % 60) + '分'
        : g.duration_minutes + '分钟'
    }
    return {
      ...g,
      statusText: this.statusText(g.status),
      timeText: this.formatTime(g.ended_at || g.created_at),
      dateText: this.formatDate(g.ended_at || g.created_at),
      my_score: g.my_score || 0,
      my_rank: g.my_rank || 0,
      duration: duration,
      playersText: (g.players || []).map(function(p) { return p.nickname }).join('、') || '—',
      resultText: resultMap[g.result] || '平',
      has_adjustment: g.has_adjustment || false,
      footerText: g.has_adjustment ? '含改分记录已确认' : '已平账 · 无争议调整',
      detailText: '查看详细手账'
    }
  },

  buildGroups(games) {
    var groups = []
    var currentDate = ''
    var currentGroup = null
    for (var i = 0; i < games.length; i++) {
      var g = games[i]
      var dateStr = g.dateText || this.formatDate(g.ended_at || g.created_at)
      if (dateStr !== currentDate) {
        currentGroup = {
          date: dateStr,
          dotClass: groups.length === 0 ? 'today' : (groups.length < 2 ? 'recent' : 'old'),
          tagText: groups.length === 0 ? '今日' : '',
          subText: '',
          items: []
        }
        groups.push(currentGroup)
        currentDate = dateStr
      }
      if (currentGroup) {
        currentGroup.items.push(g)
      }
    }
    // 计算 subText
    groups.forEach(function(group) {
      var netScore = group.items.reduce(function(sum, item) { return sum + (item.my_score || 0) }, 0)
      group.subText = group.items.length + ' 场对局 · 净胜 ' + (netScore >= 0 ? '+' : '') + netScore
    })
    this.setData({ groups: groups })
  },

  buildOverview(games) {
    var totalGames = games.length
    var totalScore = games.reduce(function(sum, g) { return sum + (g.my_score || 0) }, 0)
    var wins = games.filter(function(g) { return (g.my_rank || 0) === 1 }).length
    var winRate = totalGames > 0 ? Math.round((wins / totalGames) * 100 * 10) / 10 : 0
    this.setData({
      overview: { games: totalGames, score: totalScore, win_rate: winRate }
    })
  },

  statusText(status) {
    var map = { forming: '等紧人', active: '进行中', ended: '散台圆满', cancelled: '已取消' }
    return map[status] || status
  },

  formatTime(ts) {
    var d = util.toDate(ts)
    if (!d) return ''
    return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
  },

  formatDate(ts) {
    var d = util.toDate(ts)
    if (!d) return ''
    return (d.getMonth() + 1) + '月' + d.getDate() + '日'
  },

  // 卡片 → 每局详情页；「查看详细手账」→ 手帐明细页
  goDetail(e) {
    var gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/game-detail/game-detail?game_id=' + gameID })
  },

  goLedger(e) {
    var gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/game-ledger/game-ledger?game_id=' + gameID })
  },

  doHide(e) {
    var gameID = e.currentTarget.dataset.id
    wx.showModal({
      title: '删除记录',
      content: '确定要从对局记录中删除这场牌局吗？',
      confirmColor: '#B33A3A',
      success: (res) => {
        if (!res.confirm) return
        api.post('/games/' + gameID + '/hide', {
          request_id: api.genRequestID()
        }).then(() => {
          this.reload()
          wx.showToast({ title: '已删除', icon: 'success' })
        })
      }
    })
  },

  doLogin() {
    wx.showLoading({ title: '登录中...' })
    app.login().then(() => {
      wx.hideLoading()
      this.setData({ isLoggedIn: true })
      this.onShow()
    }).catch(() => {
      wx.hideLoading()
    })
  }
})

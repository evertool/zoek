// pages/history/history.js — 开台手账 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

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
    currentMonthText: ''
  },

  onShow() {
    this.setData({ isLoggedIn: !!app.globalData.token })
    if (app.globalData.token) {
      this.setData({ games: [], page: 1, hasMore: true })
      this.updateMonthText()
      this.loadHistory()
    } else {
      this.setData({ loading: false })
    }
  },

  onPullDownRefresh() {
    this.setData({ games: [], page: 1, hasMore: true })
    this.loadHistory().then(() => {
      wx.stopPullDownRefresh()
    })
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.loadHistory()
    }
  },

  updateMonthText() {
    const now = new Date()
    this.setData({ currentMonthText: now.getFullYear() + '年' + (now.getMonth() + 1) + '月' })
  },

  toggleMonthFilter() {
    const current = this.data.currentMonthText
    if (current.indexOf('9月') >= 0) {
      this.setData({ currentMonthText: current.replace('9月', '8月') })
    } else {
      this.setData({ currentMonthText: current.replace('8月', '9月') })
    }
  },

  loadHistory() {
    if (!app.globalData.token) return Promise.resolve()
    this.setData({ loading: true })
    return api.get('/games/history', {
      page: this.data.page,
      page_size: this.data.pageSize
    }).then(res => {
      const games = (res.games || []).map(g => {
        return {
          ...g,
          statusText: this.statusText(g.status),
          timeText: this.formatTime(g.ended_at || g.created_at),
          dateText: this.formatDate(g.ended_at || g.created_at),
          my_score: g.my_score || 0,
          my_rank: g.my_rank || 0,
          duration: g.duration || '—',
          playersText: (g.players || []).map(function(p) { return p.nickname }).join('、') || '—',
          has_adjustment: g.has_adjustment || false,
          footerText: g.has_adjustment ? '含1笔调整已确认' : '已平账 · 无争议调整',
          detailText: '查看详细手账'
        }
      })
      const allGames = this.data.games.concat(games)
      const hasMore = allGames.length < res.total
      this.setData({
        games: allGames,
        total: res.total,
        hasMore: hasMore,
        page: this.data.page + 1,
        loading: false
      })
      this.buildGroups(allGames)
      this.buildOverview(allGames)
    }).catch(() => {
      this.setData({ loading: false })
    })
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
    if (!ts) return ''
    var d = new Date(ts * 1000)
    return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
  },

  formatDate(ts) {
    if (!ts) return ''
    var d = new Date(ts * 1000)
    return (d.getMonth() + 1) + '月' + d.getDate() + '日'
  },

  goDetail(e) {
    var gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/detail/detail?game_id=' + gameID })
  },

  goSettlement(e) {
    var gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/settlement/settlement?game_id=' + gameID })
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
          var games = this.data.games.filter(function(g) { return g.game_id !== gameID })
          this.setData({
            games: games,
            total: Math.max(0, this.data.total - 1),
            hasMore: games.length < this.data.total - 1
          })
          this.buildGroups(games)
          this.buildOverview(games)
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

// pages/history/history.js — 对局记录列表
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    games: [],
    loading: true,
    page: 1,
    pageSize: 20,
    total: 0,
    hasMore: true
  },

  onShow() {
    if (app.globalData.token) {
      this.setData({ games: [], page: 1, hasMore: true })
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
          statusText: util.statusText(g.status),
          statusClass: util.statusClass(g.status),
          timeText: util.formatTime(g.ended_at || g.created_at)
        }
      })
      const allGames = [...this.data.games, ...games]
      const hasMore = allGames.length < res.total
      this.setData({
        games: allGames,
        total: res.total,
        hasMore,
        page: this.data.page + 1,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  goDetail(e) {
    const gameID = e.currentTarget.dataset.id
    wx.navigateTo({
      url: `/pages/detail/detail?game_id=${gameID}`
    })
  },

  goSettlement(e) {
    const gameID = e.currentTarget.dataset.id
    wx.navigateTo({
      url: `/pages/settlement/settlement?game_id=${gameID}`
    })
  }
})

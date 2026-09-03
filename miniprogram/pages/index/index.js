// pages/index/index.js — 首页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    games: [],
    loading: true,
    isLoggedIn: false
  },

  onShow() {
    this.setData({
      isLoggedIn: !!app.globalData.token
    })
    if (app.globalData.token) {
      this.loadGames()
    } else {
      this.setData({ loading: false })
    }
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
        return {
          ...g,
          statusText: util.statusText(g.status),
          statusClass: util.statusClass(g.status),
          roundInfo: g.current_round_number
            ? `第${g.current_round_number}局 · 已完成${g.completed_rounds}局`
            : `已完成${g.completed_rounds}局`
        }
      })
      this.setData({ games, loading: false })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  goCreate() {
    if (!app.globalData.token) {
      this.doLogin()
      return
    }
    wx.navigateTo({ url: '/pages/create/create' })
  },

  goRoom(e) {
    const gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/room/room?game_id=${gameID}` })
  },

  goScore(e) {
    const gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/score/score?game_id=${gameID}` })
  },

  doLogin() {
    wx.showLoading({ title: '登入中...' })
    app.login().then(() => {
      wx.hideLoading()
      this.setData({ isLoggedIn: true })
      this.loadGames()
    }).catch(() => {
      wx.hideLoading()
    })
  },

  goProfile() {
    wx.switchTab({ url: '/pages/profile/profile' })
  }
})

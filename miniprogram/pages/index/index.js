// pages/index/index.js — 首页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    games: [],
    loading: true,
    isLoggedIn: false,
    needProfile: false,
    tempAvatar: '',
    tempNickname: ''
  },

  onShow() {
    const isLoggedIn = !!app.globalData.token
    const needProfile = isLoggedIn && app.checkProfileNeeded()

    this.setData({ isLoggedIn, needProfile })

    if (isLoggedIn && !needProfile) {
      this.loadGames()
    } else {
      this.setData({ loading: false })
    }
  },

  onPullDownRefresh() {
    if (app.globalData.token && !app.checkProfileNeeded()) {
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

  // ===== 登录流程 =====
  doLogin() {
    wx.showLoading({ title: '登录中...' })
    app.login().then(() => {
      wx.hideLoading()
      const needProfile = app.checkProfileNeeded()
      this.setData({
        isLoggedIn: true,
        needProfile
      })
      if (!needProfile) {
        this.loadGames()
      }
    }).catch(() => {
      wx.hideLoading()
    })
  },

  // ===== 头像昵称授权 =====
  onChooseAvatar(e) {
    this.setData({ tempAvatar: e.detail.avatarUrl })
  },

  onNicknameInput(e) {
    this.setData({ tempNickname: e.detail.value })
  },

  doSaveProfile() {
    const nickname = this.data.tempNickname.trim()
    const avatarURL = this.data.tempAvatar

    if (!nickname) {
      wx.showToast({ title: '请输入昵称', icon: 'none' })
      return
    }
    if (!avatarURL) {
      wx.showToast({ title: '请选择头像', icon: 'none' })
      return
    }

    wx.showLoading({ title: '保存中...' })
    // 先上传头像（这里简化：直接用微信临时路径，实际生产需上传到 OSS）
    // 临时方案：直接保存 URL
    app.saveProfile(nickname, avatarURL).then(() => {
      wx.hideLoading()
      wx.showToast({ title: '已完善', icon: 'success' })
      this.setData({ needProfile: false })
      this.loadGames()
    }).catch(() => {
      wx.hideLoading()
    })
  },

  // ===== 列表操作 =====
  goCreate() {
    if (!app.globalData.token) {
      this.doLogin()
      return
    }
    if (app.checkProfileNeeded()) {
      this.setData({ needProfile: true })
      return
    }
    wx.navigateTo({ url: '/pages/create/create' })
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

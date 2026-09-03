// pages/profile/profile.js — 我的页
const app = getApp()
const api = require('../../utils/api')

Page({
  data: {
    isLoggedIn: false,
    nickname: '',
    avatarURL: '',
    editing: false,
    newNickname: ''
  },

  onShow() {
    this.setData({
      isLoggedIn: !!app.globalData.token,
      nickname: app.globalData.nickname || '',
      avatarURL: app.globalData.avatarURL || ''
    })
  },

  doLogin() {
    wx.showLoading({ title: '登入中...' })
    app.login().then(() => {
      wx.hideLoading()
      this.setData({
        isLoggedIn: true,
        nickname: app.globalData.nickname
      })
      wx.showToast({ title: '登入成功', icon: 'success' })
    }).catch(() => {
      wx.hideLoading()
    })
  },

  doLogout() {
    wx.showModal({
      title: '退出登入',
      content: '确定要退出登入吗？',
      success: (res) => {
        if (res.confirm) {
          app.logout()
          this.setData({ isLoggedIn: false, nickname: '' })
        }
      }
    })
  },

  startEdit() {
    this.setData({
      editing: true,
      newNickname: this.data.nickname
    })
  },

  onNicknameInput(e) {
    this.setData({ newNickname: e.detail.value })
  },

  saveNickname() {
    const name = this.data.newNickname.trim()
    if (!name) {
      wx.showToast({ title: '昵称不能为空', icon: 'none' })
      return
    }
    api.put('/user/profile', { nickname: name }).then(res => {
      app.globalData.nickname = res.nickname
      wx.setStorageSync('nickname', res.nickname)
      this.setData({
        nickname: res.nickname,
        editing: false
      })
      wx.showToast({ title: '已更新', icon: 'success' })
    })
  },

  cancelEdit() {
    this.setData({ editing: false })
  }
})

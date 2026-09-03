// pages/profile/profile.js — 我的页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    isLoggedIn: false,
    nickname: '',
    avatarURL: '',
    avatarColor: '',
    editing: false,
    tempNickname: '',
    tempAvatar: ''
  },

  onShow() {
    this.setData({
      isLoggedIn: !!app.globalData.token,
      nickname: app.globalData.nickname || '',
      avatarURL: app.globalData.avatarURL || '',
      avatarColor: util.avatarColor(app.globalData.nickname || '')
    })
  },

  doLogin() {
    wx.showLoading({ title: '登录中...' })
    app.login().then(() => {
      wx.hideLoading()
      this.setData({
        isLoggedIn: true,
        nickname: app.globalData.nickname,
        avatarURL: app.globalData.avatarURL,
        avatarColor: util.avatarColor(app.globalData.nickname)
      })
    }).catch(() => {
      wx.hideLoading()
    })
  },

  doLogout() {
    wx.showModal({
      title: '退出登录',
      content: '确定要退出吗？',
      confirmColor: '#B33A3A',
      success: (res) => {
        if (res.confirm) {
          app.logout()
          this.setData({ isLoggedIn: false, nickname: '', avatarURL: '' })
        }
      }
    })
  },

  goEditProfile() {
    this.setData({
      editing: true,
      tempNickname: this.data.nickname,
      tempAvatar: this.data.avatarURL
    })
  },

  onChooseAvatar(e) {
    this.setData({ tempAvatar: e.detail.avatarUrl })
  },

  onNicknameInput(e) {
    this.setData({ tempNickname: e.detail.value })
  },

  saveProfile() {
    const nickname = this.data.tempNickname.trim()
    if (!nickname) {
      wx.showToast({ title: '请输入昵称', icon: 'none' })
      return
    }

    wx.showLoading({ title: '保存中...' })
    app.saveProfile(nickname, this.data.tempAvatar).then(() => {
      wx.hideLoading()
      wx.showToast({ title: '已保存', icon: 'success' })
      this.setData({
        nickname: app.globalData.nickname,
        avatarURL: app.globalData.avatarURL,
        avatarColor: util.avatarColor(app.globalData.nickname),
        editing: false
      })
    }).catch(() => {
      wx.hideLoading()
    })
  },

  cancelEdit() {
    this.setData({ editing: false })
  }
})

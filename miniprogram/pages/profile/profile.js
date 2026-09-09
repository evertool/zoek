// pages/profile/profile.js — 我的页 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    isLoggedIn: false,
    nickname: '',
    avatarURL: '',
    avatarColor: '',
    userId: '',
    motto: '',
    editing: false,
    tempNickname: '',
    tempAvatar: '',
    avatarChanged: false,
    stats: null,
    badges: [],
    vibrateEnabled: false,
    showToast: false,
    toastMsg: ''
  },

  onShow() {
    var isLoggedIn = !!app.globalData.token
    this.setData({
      isLoggedIn: isLoggedIn,
      nickname: app.globalData.nickname || '',
      avatarURL: app.globalData.avatarURL || '',
      avatarColor: util.avatarColor(app.globalData.nickname || ''),
      userId: app.globalData.userID ? ('ZM' + String(app.globalData.userID).padStart(6, '0')) : ''
    })
    if (isLoggedIn) {
      this.loadStats()
      this.loadBadges()
    }
  },

  loadStats() {
    api.get('/user/stats').then(res => {
      this.setData({
        stats: {
          games: res.games || 0,
          total_score: res.total_score || 0,
          avg_score: res.avg_score || 0,
          win_rate: res.win_rate || 0,
          recent_wins: res.recent_wins || 0
        }
      })
    }).catch(function() {})
  },

  loadBadges() {
    // Mock badge data — 后端实现后替换
    this.setData({
      badges: [
        { id: 1, name: '雀神', desc: '累计胜场 ≥ 50场 (达成52场)', icon: '🏆', style: 'gold', locked: false, status: '已点亮' },
        { id: 2, name: '连胜王', desc: '连续 3 场排名第 1', icon: '🔥', style: 'red', locked: false, status: '最高4连胜' },
        { id: 3, name: '大翻盘', desc: '单场从负转正 ≥ 30分', icon: '🔄', style: 'green', locked: false, status: '已达成' },
        { id: 4, name: '稳如泰山', desc: '局间积分波动极小', icon: '🛡', style: 'neutral', locked: false, status: '防守高手' },
        { id: 5, name: '常客', desc: '累计参与 28 场牌局', icon: '🪑', style: 'green', locked: false, status: '活跃牌友' },
        { id: 6, name: '铁脚', desc: '累计参与 ≥ 100场', icon: '🔒', style: 'neutral', locked: true, progress: 28, current: 28, target: 100 }
      ]
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
        avatarColor: util.avatarColor(app.globalData.nickname),
        userId: app.globalData.userID ? ('ZM' + String(app.globalData.userID).padStart(6, '0')) : ''
      })
      this.loadStats()
      this.loadBadges()
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
          this.setData({ isLoggedIn: false, nickname: '', avatarURL: '', stats: null, badges: [] })
        }
      }
    })
  },

  goEditProfile() {
    this.setData({
      editing: true,
      tempNickname: this.data.nickname,
      tempAvatar: this.data.avatarURL,
      avatarChanged: false
    })
  },

  onChooseAvatar(e) {
    this.setData({ tempAvatar: e.detail.avatarUrl, avatarChanged: true })
  },

  onNicknameInput(e) {
    this.setData({ tempNickname: e.detail.value })
  },

  saveProfile() {
    var nickname = this.data.tempNickname.trim()
    if (!nickname) {
      wx.showToast({ title: '请输入昵称', icon: 'none' })
      return
    }

    wx.showLoading({ title: '保存中...' })
    var save = (avatarURL) => app.saveProfile(nickname, avatarURL)
    var op = this.data.avatarChanged
      ? util.avatarToDataUrl(this.data.tempAvatar).then(save)
      : save(this.data.tempAvatar)
    op.then(() => {
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
  },

  copyUserId() {
    wx.setClipboardData({
      data: this.data.userId,
      success: () => {
        this.showToast('雀友号已复制: ' + this.data.userId)
      }
    })
  },

  toggleVibrate() {
    this.setData({ vibrateEnabled: !this.data.vibrateEnabled })
    this.showToast(this.data.vibrateEnabled ? '触感振动提醒已开启' : '触感振动提醒已关闭')
  },

  showBlacklist() {
    this.showToast('已载入 0 位屏蔽雀友')
  },

  showSound() {
    this.showToast('粤语原声语音包已启用')
  },

  showSafety() {
    this.showToast('纯休闲记账工具，绝不涉及真钱对付')
  },

  showAbout() {
    this.showToast('得闲开台 v1.2.4 (Build 2026)')
  },

  showToast(msg) {
    this.setData({ showToast: true, toastMsg: msg })
    if (this._toastTimer) clearTimeout(this._toastTimer)
    this._toastTimer = setTimeout(() => {
      this.setData({ showToast: false })
    }, 2200)
  }
})

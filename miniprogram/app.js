// app.js — 雀友记小程序入口
const api = require('./utils/api')

App({
  globalData: {
    token: '',
    userID: 0,
    nickname: '',
    avatarURL: '',
    baseURL: 'http://127.0.0.1:8080/api/v1'
  },

  onLaunch() {
    // 恢复登录状态
    const token = wx.getStorageSync('token')
    if (token) {
      this.globalData.token = token
      this.globalData.userID = wx.getStorageSync('user_id') || 0
      this.globalData.nickname = wx.getStorageSync('nickname') || ''
      // 验证 token 是否仍有效
      api.get('/user/profile').then(res => {
        this.globalData.nickname = res.nickname || ''
        this.globalData.avatarURL = res.avatar_url || ''
        wx.setStorageSync('nickname', this.globalData.nickname)
      }).catch(() => {
        // token 已过期，清除
        this.logout()
      })
    }
  },

  /** 确保已登录，返回 Promise<string token> */
  ensureLogin() {
    if (this.globalData.token) {
      return Promise.resolve(this.globalData.token)
    }
    return this.login()
  },

  /** 微信登录流程 */
  login() {
    return new Promise((resolve, reject) => {
      wx.login({
        success: (res) => {
          if (!res.code) {
            wx.showToast({ title: '登入失败，请再试一次', icon: 'none' })
            reject(new Error('no code'))
            return
          }
          api.post('/auth/login', {
            code: res.code,
            nickname: this.globalData.nickname || ''
          }).then(data => {
            this.globalData.token = data.token
            this.globalData.userID = data.user_id
            this.globalData.nickname = data.nickname
            wx.setStorageSync('token', data.token)
            wx.setStorageSync('user_id', data.user_id)
            wx.setStorageSync('nickname', data.nickname)
            resolve(data.token)
          }).catch(err => {
            wx.showToast({ title: '登入失败，请再试一次', icon: 'none' })
            reject(err)
          })
        },
        fail: () => {
          wx.showToast({ title: '登入失败，请再试一次', icon: 'none' })
          reject(new Error('wx.login failed'))
        }
      })
    })
  },

  /** 退出登录 */
  logout() {
    this.globalData.token = ''
    this.globalData.userID = 0
    this.globalData.nickname = ''
    wx.removeStorageSync('token')
    wx.removeStorageSync('user_id')
    wx.removeStorageSync('nickname')
  }
})

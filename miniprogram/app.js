// app.js — 雀记小程序入口
const api = require('./utils/api')

App({
  globalData: {
    token: '',
    userID: 0,
    nickname: '',
    avatarURL: '',
    baseURL: 'http://127.0.0.1:8080/api/v1',
    needProfile: false
  },

  onLaunch() {
    const token = wx.getStorageSync('token')
    if (token) {
      this.globalData.token = token
      this.globalData.userID = wx.getStorageSync('user_id') || 0
      this.globalData.nickname = wx.getStorageSync('nickname') || ''
      this.globalData.avatarURL = wx.getStorageSync('avatar_url') || ''
      api.get('/user/profile').then(res => {
        this.globalData.nickname = res.nickname || ''
        this.globalData.avatarURL = res.avatar_url || ''
        wx.setStorageSync('nickname', this.globalData.nickname)
        wx.setStorageSync('avatar_url', this.globalData.avatarURL)
      }).catch(() => {
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

  /** 微信登录流程（仅获取 code → 后端换 token） */
  login() {
    return new Promise((resolve, reject) => {
      wx.login({
        success: (res) => {
          if (!res.code) {
            wx.showToast({ title: '登录失败', icon: 'none' })
            reject(new Error('no code'))
            return
          }
          api.post('/auth/login', {
            code: res.code
          }).then(data => {
            this.globalData.token = data.token
            this.globalData.userID = data.user_id
            this.globalData.nickname = data.nickname
            wx.setStorageSync('token', data.token)
            wx.setStorageSync('user_id', data.user_id)
            wx.setStorageSync('nickname', data.nickname)
            // 判断是否需要授权头像昵称
            if (!data.nickname || data.nickname === '玩家' || !this.globalData.avatarURL) {
              this.globalData.needProfile = true
            }
            resolve(data.token)
          }).catch(err => {
            wx.showToast({ title: '登录失败', icon: 'none' })
            reject(err)
          })
        },
        fail: () => {
          wx.showToast({ title: '登录失败', icon: 'none' })
          reject(new Error('wx.login failed'))
        }
      })
    })
  },

  /** 保存头像昵称到后端 */
  saveProfile(nickname, avatarURL) {
    return api.put('/user/profile', {
      nickname: nickname,
      avatar_url: avatarURL
    }).then(res => {
      this.globalData.nickname = res.nickname
      this.globalData.avatarURL = res.avatar_url
      this.globalData.needProfile = false
      wx.setStorageSync('nickname', res.nickname)
      wx.setStorageSync('avatar_url', res.avatar_url)
      return res
    })
  },

  /** 检查是否需要授权头像昵称 */
  checkProfileNeeded() {
    return this.globalData.needProfile ||
      !this.globalData.nickname ||
      this.globalData.nickname === '玩家' ||
      !this.globalData.avatarURL
  },

  /** 退出登录 */
  logout() {
    this.globalData.token = ''
    this.globalData.userID = 0
    this.globalData.nickname = ''
    this.globalData.avatarURL = ''
    this.globalData.needProfile = false
    wx.removeStorageSync('token')
    wx.removeStorageSync('user_id')
    wx.removeStorageSync('nickname')
    wx.removeStorageSync('avatar_url')
  }
})

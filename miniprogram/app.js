// app.js — 得闲开台小程序入口
const api = require('./utils/api')

/** 头像是否已持久化。chooseAvatar 的微信临时路径（http://tmp/、wxfile://）
 *  重启后失效，视同未设置，需重新授权。 */
function isPersistentAvatar(url) {
  return !!url && (url.indexOf('data:image') === 0 || url.indexOf('https://') === 0)
}

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
        // 以服务端为准：资料完整则不再弹出完善资料页
        this.globalData.needProfile = !!res.need_profile
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
            this.globalData.avatarURL = data.avatar_url || ''
            wx.setStorageSync('token', data.token)
            wx.setStorageSync('user_id', data.user_id)
            wx.setStorageSync('nickname', data.nickname)
            wx.setStorageSync('avatar_url', this.globalData.avatarURL)
            // 以服务端返回为准；后端未返回时按本地资料推导
            this.globalData.needProfile = data.need_profile !== undefined
              ? !!data.need_profile
              : (!this.globalData.nickname || !isPersistentAvatar(this.globalData.avatarURL))
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
      // 以服务端判定为准：存入无效头像（如临时路径）时仍视为资料不全
      this.globalData.needProfile = !!res.need_profile
      wx.setStorageSync('nickname', res.nickname)
      wx.setStorageSync('avatar_url', res.avatar_url)
      return res
    })
  },

  /** 检查是否需要授权头像昵称（后端 need_profile 为准，本地推导兜底） */
  checkProfileNeeded() {
    return this.globalData.needProfile ||
      !this.globalData.nickname ||
      !isPersistentAvatar(this.globalData.avatarURL)
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

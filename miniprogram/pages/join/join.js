// pages/join/join.js — 扫码入台页（从分享链接进入）
const app = getApp()
const api = require('../../utils/api')

Page({
  data: {
    inviteToken: '',
    loading: true,
    joining: false,
    error: '',
    game: null
  },

  onLoad(options) {
    // 从分享链接 path 中提取 invite_token
    let token = options.invite_token || options.q

    // 如果是小程序码扫码进入，q 参数可能是 URL 编码
    if (options.q) {
      try {
        const decoded = decodeURIComponent(options.q)
        const url = new URL(decoded)
        token = url.searchParams.get('invite_token') || token
      } catch (e) {
        // 非 URL 格式，直接当 token 用
      }
    }

    if (!token) {
      this.setData({
        loading: false,
        error: 'QR Code 已失效，联络台主重新开台'
      })
      return
    }

    this.setData({ inviteToken: token })

    // 确保登录后查询
    app.ensureLogin().then(() => {
      this.tryJoin()
    }).catch(() => {
      this.setData({
        loading: false,
        error: '登入失败，请再试一次'
      })
    })
  },

  tryJoin() {
    this.setData({ joining: true })
    api.post('/games/join', {
      invite_token: this.data.inviteToken,
      request_id: api.genRequestID()
    }).then(res => {
      // 成功加入，跳转到台间页
      wx.redirectTo({
        url: `/pages/room/room?game_id=${res.game_id}`
      })
    }).catch(err => {
      // 已加入过的直接跳转
      if (err && err.action === 'BACK_TO_ROOM' && err.game_id) {
        wx.redirectTo({
          url: `/pages/room/room?game_id=${err.game_id}`
        })
        return
      }
      // 其他错误
      const msg = (err && err.message) || '入台失败'
      this.setData({
        loading: false,
        joining: false,
        error: msg
      })
    })
  }
})

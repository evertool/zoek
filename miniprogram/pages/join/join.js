// pages/join/join.js — 扫码入台页
const app = getApp()
const api = require('../../utils/api')
const guard = require('../../utils/guard')

Page({
  data: {
    inviteToken: '',
    gameID: 0,
    loading: true,
    joining: false,
    error: '',
    inGameId: 0,
    game: null
  },

  onLoad(options) {
    // 登录/资料完善守卫：未通过弹回首页，完成后带着原参数回来继续入台
    if (!guard.ensure(true)) return

    // 小程序码扫码进入
    if (options.scene) {
      const gameID = Number(decodeURIComponent(options.scene))
      if (gameID) {
        this.setData({ gameID })
        app.ensureLogin().then(() => {
          this.joinByGameID(gameID)
        }).catch(() => {
          this.setData({ loading: false, error: '登录失败，请重试' })
        })
        return
      }
    }

    // 分享链接进入
    let token = options.invite_token || options.q

    if (options.q) {
      try {
        const decoded = decodeURIComponent(options.q)
        const url = new URL(decoded)
        token = url.searchParams.get('invite_token') || token
      } catch (e) {}
    }

    if (!token) {
      this.setData({ loading: false, error: '二维码已失效' })
      return
    }

    this.setData({ inviteToken: token })

    app.ensureLogin().then(() => {
      this.tryJoin()
    }).catch(() => {
      this.setData({ loading: false, error: '登录失败，请重试' })
    })
  },

  joinByGameID(gameID) {
    this.setData({ joining: true })
    api.post('/games/join', {
      game_id: gameID,
      request_id: api.genRequestID()
    }).then(res => {
      wx.redirectTo({ url: `/pages/room/room?game_id=${res.game_id}` })
    }).catch(err => this.handleJoinError(err))
  },

  handleJoinError(err) {
    if (err && err.action === 'BACK_TO_ROOM' && err.game_id) {
      wx.redirectTo({ url: `/pages/room/room?game_id=${err.game_id}` })
      return
    }
    if (err && err.code === 'ALREADY_IN_GAME') {
      // 房间互斥：已在别的牌台，引导回去
      this.setData({ loading: false, joining: false, inGameId: err.game_id || 0, error: err.message || '你已有一张进行中的牌台' })
      return
    }
    const msg = (err && err.message) || '加入失败'
    this.setData({ loading: false, joining: false, error: msg })
  },

  goMyRoom() {
    if (this.data.inGameId) {
      wx.redirectTo({ url: '/pages/room/room?game_id=' + this.data.inGameId })
    }
  },

  tryJoin() {
    this.setData({ joining: true })
    api.post('/games/join', {
      invite_token: this.data.inviteToken,
      request_id: api.genRequestID()
    }).then(res => {
      wx.redirectTo({ url: `/pages/room/room?game_id=${res.game_id}` })
    }).catch(err => {
      if (err && err.action === 'BACK_TO_ROOM' && err.game_id) {
        wx.redirectTo({ url: `/pages/room/room?game_id=${err.game_id}` })
        return
      }
      this.handleJoinError(err)
    })
  },

  retryJoin() {
    this.setData({ loading: true, error: '', joining: false })
    if (this.data.gameID) {
      this.joinByGameID(this.data.gameID)
    } else if (this.data.inviteToken) {
      this.tryJoin()
    }
  }
})

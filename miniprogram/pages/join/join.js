// pages/join/join.js — 扫码入台页
// 审核合规：进入即静默登录并直接尝试入台；资料未完善时弹出「可关闭」的完善弹窗，
// 拒绝后停留在本页给出说明与重开入口，不强制跳转、不重复弹窗。
const app = getApp()
const api = require('../../utils/api')

Page({
  data: {
    inviteToken: '',
    gameID: 0,
    seat: 0,
    loading: true,
    joining: false,
    error: '',
    profileBlocked: false,
    profileSheet: false,
    inGameId: 0,
    game: null
  },

  onLoad(options) {
    // 座位邀请：分享链接带的 seat = 发起邀请的空位，join 成功后透传给房间页自动落座
    this.setData({ seat: Number(options.seat) || 0 })

    // 小程序码扫码进入
    if (options.scene) {
      const gameID = Number(decodeURIComponent(options.scene))
      if (gameID) {
        this.setData({ gameID })
        this.bootstrap()
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
    this.bootstrap()
  },

  /** 等待静默登录完成 → 资料未完善则弹完善窗，否则直接入台 */
  bootstrap() {
    var that = this
    app.ready().then(function() {
      return app.ensureLogin()
    }).then(function() {
      if (app.checkProfileNeeded()) {
        that.setData({ loading: false, profileSheet: true })
        return
      }
      that.proceedJoin()
    }).catch(function() {
      that.setData({ loading: false, error: '登录失败，请重试' })
    })
  },

  proceedJoin() {
    if (this.data.gameID) {
      this.joinByGameID(this.data.gameID)
    } else if (this.data.inviteToken) {
      this.tryJoin()
    } else {
      this.setData({ loading: false, error: '二维码已失效' })
    }
  },

  joinByGameID(gameID) {
    this.setData({ joining: true, loading: false })
    api.post('/games/join', {
      game_id: gameID,
      request_id: api.genRequestID()
    }, { silent: true }).then(res => {
      wx.redirectTo({ url: `/pages/room/room?game_id=${res.game_id}${this.data.seat ? '&seat=' + this.data.seat : ''}` })
    }).catch(err => this.handleJoinError(err))
  },

  handleJoinError(err) {
    // 已完结的台：房间已不存在，带去看对局记录详情（后端附 game_id）
    if (err && err.code === 'GAME_ENDED' && err.game_id) {
      wx.redirectTo({ url: '/pages/game-detail/game-detail?game_id=' + err.game_id })
      return
    }
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

  // 翻去首页（牌局 tab 页）
  goHome() {
    wx.reLaunch({ url: '/pages/index/index' })
  },

  tryJoin() {
    this.setData({ joining: true, loading: false })
    api.post('/games/join', {
      invite_token: this.data.inviteToken,
      request_id: api.genRequestID()
    }, { silent: true }).then(res => {
      wx.redirectTo({ url: `/pages/room/room?game_id=${res.game_id}${this.data.seat ? '&seat=' + this.data.seat : ''}` })
    }).catch(err => {
      if (err && err.action === 'BACK_TO_ROOM' && err.game_id) {
        wx.redirectTo({ url: `/pages/room/room?game_id=${err.game_id}` })
        return
      }
      this.handleJoinError(err)
    })
  },

  // ===== 完善资料弹窗（可关闭，仅入台场景触发） =====
  onProfileSaved() {
    this.setData({ profileSheet: false, profileBlocked: false })
    this.proceedJoin()
  },

  onProfileClose() {
    // 用户拒绝完善：停在当前页给说明 + 重开入口，不骚扰
    this.setData({ profileSheet: false, profileBlocked: true, error: '入台前请先完善头像昵称，方便雀友认得你' })
  },

  /** 失败态里重新打开完善弹窗 */
  goCompleteProfile() {
    this.setData({ error: '', profileBlocked: false, profileSheet: true })
  },

  retryJoin() {
    this.setData({ loading: true, error: '', joining: false, profileBlocked: false })
    this.proceedJoin()
  }
})

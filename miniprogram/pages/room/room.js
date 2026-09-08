// pages/room/room.js — 房间页 v5：常驻邀请 + 成员实时刷新 + 入台欢迎动画
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

// 安全加载 lottie（npm 构建失败时不会阻断页面）
let lottie = null
try {
  lottie = require('lottie-miniprogram')
} catch (e) {
  console.warn('lottie-miniprogram not available, welcome animation disabled')
}

let welcomeAnimData = null
try {
  welcomeAnimData = require('../../assets/animations/welcome-join.js')
} catch (e) {
  console.warn('welcome animation data load failed')
}

const POLL_INTERVAL = 4000

Page({
  data: {
    gameID: 0,
    game: null,
    players: [],
    inviteToken: '',
    qrPath: '',
    qrLoading: false,
    qrError: '',
    loading: true,
    isOwner: false,
    canInvite: false,
    welcome: false,
    welcomeName: ''
  },

  onLoad(options) {
    this.setData({
      gameID: Number(options.game_id) || 0,
      inviteToken: options.invite_token || ''
    })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
    this.loadGame()
  },

  onShow() {
    if (this.data.gameID && !this.data.loading) {
      this.loadGame()
    }
    this.startPolling()
  },

  onHide() {
    this.stopPolling()
  },

  onUnload() {
    this.stopPolling()
    this.destroyWelcomeAnim()
  },

  // ===== 成员实时刷新：停留房间页期间每 4 秒轮询 =====
  startPolling() {
    if (this._poll) return
    this._poll = setInterval(() => this.pollGame(), POLL_INTERVAL)
  },

  stopPolling() {
    if (this._poll) {
      clearInterval(this._poll)
      this._poll = null
    }
  },

  pollGame() {
    if (!this.data.gameID) return
    api.get(`/games/${this.data.gameID}`).then(res => {
      this.applyGame(res, true)
    }).catch(() => {})
  },

  loadGame() {
    this.setData({ loading: true })
    api.get(`/games/${this.data.gameID}`).then(res => {
      this.applyGame(res, false)
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  /** 应用牌局数据；poll=true 时对比成员变化，新雀友入台播欢迎动画 */
  applyGame(res, poll) {
    const players = (res.players || []).map(p => {
      return {
        ...p,
        isOwner: p.role === 'owner',
        avatarColor: util.avatarColor(p.nickname)
      }
    })
    const game = {
      ...res,
      statusText: util.statusText(res.status),
      statusClass: util.statusClass(res.status)
    }
    const canInvite = (game.status === 'forming' || game.status === 'active') &&
      !game.members_locked && players.length < 4

    // 对比新旧成员，找出刚入台的雀友
    let newcomers = []
    if (poll && Array.isArray(this._knownIds)) {
      newcomers = (res.players || []).filter(p => this._knownIds.indexOf(p.player_id) < 0)
    }
    this._knownIds = (res.players || []).map(p => p.player_id)

    this.setData({
      game,
      players,
      isOwner: res.creator_id === app.globalData.userID,
      canInvite,
      loading: false
    })

    if (canInvite && !this.data.qrPath && !this.data.qrLoading) {
      this.loadQRCode()
    }
    if (poll && newcomers.length && this._seenOnce) {
      this.showWelcome(newcomers)
    }
    this._seenOnce = true
  },

  // ===== 入台欢迎动画 =====
  showWelcome(newcomers) {
    const name = newcomers.map(p => p.nickname).join('、')
    if (!name) return
    wx.vibrateShort({ type: 'light' })
    this.setData({ welcome: true, welcomeName: name })
    setTimeout(() => this.playWelcomeLottie(), 80)
    if (this._welcomeTimer) clearTimeout(this._welcomeTimer)
    this._welcomeTimer = setTimeout(() => {
      this.setData({ welcome: false })
      this.destroyWelcomeAnim()
    }, 2600)
  },

  playWelcomeLottie() {
    if (!lottie || !welcomeAnimData) return
    const query = wx.createSelectorQuery().in(this)
    query.select('#lottie-welcome').fields({ node: true, size: true }).exec(res => {
      if (!res || !res[0] || !res[0].node) return
      const canvas = res[0].node
      const ctx = canvas.getContext('2d')
      const dpr = wx.getSystemInfoSync().pixelRatio
      canvas.width = res[0].width * dpr
      canvas.height = res[0].height * dpr
      ctx.scale(dpr, dpr)
      this.destroyWelcomeAnim()
      // 深拷贝 animationData，避免 lottie 运行时改写共享数据
      const data = JSON.parse(JSON.stringify(welcomeAnimData))
      this._welcomeAnim = lottie.loadAnimation({
        loop: false,
        autoplay: true,
        animationData: data,
        rendererSettings: {
          context: ctx,
          clearCanvas: true
        }
      })
    })
  },

  destroyWelcomeAnim() {
    if (this._welcomeAnim && this._welcomeAnim.destroy) {
      try { this._welcomeAnim.destroy() } catch (e) {}
      this._welcomeAnim = null
    }
  },

  loadQRCode() {
    this.setData({ qrLoading: true, qrError: '' })

    wx.request({
      url: app.globalData.baseURL + `/games/${this.data.gameID}/qrcode`,
      method: 'GET',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      responseType: 'arraybuffer',
      success: (res) => {
        if (res.statusCode === 200) {
          const fs = wx.getFileSystemManager()
          const filePath = `${wx.env.USER_DATA_PATH}/qrcode_${this.data.gameID}.png`
          try {
            fs.writeFileSync(filePath, res.data, 'binary')
            this.setData({ qrPath: filePath, qrLoading: false })
          } catch (e) {
            const base64 = wx.arrayBufferToBase64(res.data)
            this.setData({ qrPath: 'data:image/png;base64,' + base64, qrLoading: false })
          }
        } else {
          this.setData({ qrLoading: false, qrError: '生成失败，请检查配置' })
        }
      },
      fail: () => {
        this.setData({ qrLoading: false, qrError: '网络错误，请重试' })
      }
    })
  },

  goScore() {
    wx.navigateTo({ url: `/pages/score/score?game_id=${this.data.gameID}` })
  },

  goSettlement() {
    wx.navigateTo({ url: `/pages/settlement/settlement?game_id=${this.data.gameID}` })
  },

  doCancel() {
    wx.showModal({
      title: '删除房间',
      content: '确定要删除这个房间吗？',
      confirmColor: '#B33A3A',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${this.data.gameID}/cancel`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已删除', icon: 'success' })
            setTimeout(() => { wx.navigateBack() }, 1000)
          })
        }
      }
    })
  },

  doEnd() {
    wx.showModal({
      title: '散台',
      content: '确定要散台吗？结束后进入结算页面。',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${this.data.gameID}/end`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已散台', icon: 'success' })
            wx.redirectTo({ url: `/pages/settlement/settlement?game_id=${this.data.gameID}` })
          })
        }
      }
    })
  },

  onShareAppMessage() {
    return {
      title: `「${this.data.game ? this.data.game.name : '得闲开台'}」等紧你入台！`,
      path: `/pages/join/join?invite_token=${this.data.inviteToken || this.data.gameID}`
    }
  },

  onShareTimeline() {
    return {
      title: `「${this.data.game ? this.data.game.name : '得闲开台'}」等紧你入台！`
    }
  }
})

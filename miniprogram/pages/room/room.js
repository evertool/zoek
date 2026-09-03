// pages/room/room.js — 台间页（房间页）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    gameID: 0,
    game: null,
    players: [],
    inviteToken: '',
    qrPath: '',
    loading: true,
    isOwner: false
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

    // 如果有 invite_token，生成小程序码
    if (this.data.inviteToken) {
      // 使用微信原生接口生成普通二维码（MVP 不接小程序码 API，用 Canvas 画 QR）
      // 简化：用分享功能代替
    }
  },

  onShow() {
    if (this.data.gameID && !this.data.loading) {
      this.loadGame()
    }
  },

  loadGame() {
    this.setData({ loading: true })
    api.get(`/games/${this.data.gameID}`).then(res => {
      const players = (res.players || []).map(p => {
        return { ...p, isOwner: p.role === 'owner' }
      })
      this.setData({
        game: {
          ...res,
          statusText: util.statusText(res.status),
          statusClass: util.statusClass(res.status)
        },
        players,
        isOwner: res.creator_id === app.globalData.userID,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  goScore() {
    wx.navigateTo({
      url: `/pages/score/score?game_id=${this.data.gameID}`
    })
  },

  goSettlement() {
    wx.navigateTo({
      url: `/pages/settlement/settlement?game_id=${this.data.gameID}`
    })
  },

  doStart() {
    wx.showModal({
      title: '开始计分',
      content: '确定要开始计分吗？开始后不能加入新雀友。',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${this.data.gameID}/start`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '开始计分', icon: 'success' })
            this.loadGame()
          })
        }
      }
    })
  },

  doCancel() {
    wx.showModal({
      title: '取消牌桌',
      content: '确定要取消呢个牌桌吗？',
      showCancel: true,
      confirmColor: '#F44336',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${this.data.gameID}/cancel`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已取消', icon: 'success' })
            setTimeout(() => {
              wx.navigateBack()
            }, 1000)
          })
        }
      }
    })
  },

  doEnd() {
    wx.showModal({
      title: '散台',
      content: '确定要散台吗？结束后将进入结算页面。',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${this.data.gameID}/end`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已散台', icon: 'success' })
            wx.redirectTo({
              url: `/pages/settlement/settlement?game_id=${this.data.gameID}`
            })
          })
        }
      }
    })
  },

  onShareAppMessage() {
    return {
      title: `雀友记 — ${this.data.game ? this.data.game.name : '快来打牌！'}`,
      path: `/pages/join/join?invite_token=${this.data.inviteToken}`
    }
  },

  onShareTimeline() {
    return {
      title: `雀友记 — ${this.data.game ? this.data.game.name : '快来打牌！'}`
    }
  }
})

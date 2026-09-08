// pages/room/room.js — 房间页
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
    qrLoading: false,
    qrError: '',
    loading: true,
    starting: false,
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
      this.setData({
        game,
        players,
        isOwner: res.creator_id === app.globalData.userID,
        loading: false
      })

      if (game.status === 'forming' && !this.data.qrPath && !this.data.qrLoading) {
        this.loadQRCode()
      }
    }).catch(() => {
      this.setData({ loading: false })
    })
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

  doStart() {
    if (this.data.starting) return
    this.setData({ starting: true })
    // 创建第 1 局，服务端将牌桌置为 active
    api.post(`/games/${this.data.gameID}/rounds`, {
      request_id: api.genRequestID()
    }).then(() => {
      this.setData({ starting: false })
      wx.showToast({ title: '开始计分', icon: 'success' })
      this.goScore()
    }).catch(() => {
      this.setData({ starting: false })
      this.loadGame()
    })
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
      title: `得闲开台 — ${this.data.game ? this.data.game.name : '快来打牌！'}`,
      path: `/pages/join/join?invite_token=${this.data.inviteToken || this.data.gameID}`
    }
  },

  onShareTimeline() {
    return {
      title: `得闲开台 — ${this.data.game ? this.data.game.name : '快来打牌！'}`
    }
  }
})

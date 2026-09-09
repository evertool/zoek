// pages/settlement/settlement.js — 结算页 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    gameID: 0,
    settlement: null,
    gameName: '',
    loading: true,
    winnerName: '',
    winnerAvatar: '',
    winnerScore: 0,
    showToast: false,
    toastMsg: ''
  },

  onLoad(options) {
    this.setData({ gameID: Number(options.game_id) || 0 })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
    this.loadSettlement()
  },

  onShow() {
    if (this.data.gameID && !this.data.loading) {
      this.loadSettlement()
    }
  },

  loadSettlement() {
    this.setData({ loading: true })
    api.get('/games/' + this.data.gameID + '/settlement').then(res => {
      var players = res.players || []
      
      // 排名
      var sorted = players.map(function(p, idx) {
        return {
          ...p,
          rank: idx + 1,
          score: p.total_score || 0,
          detail: p.games ? (p.games + ' 场 · 胜 ' + p.wins) : '',
          wind: p.wind || ''
        }
      })

      var winner = sorted[0] || null
      var settlement = {
        ...res,
        players: sorted,
        completed_rounds: res.completed_rounds || 0,
        max_round_score: res.max_round_score || 0,
        transfer_count: res.transfer_count || 0
      }

      this.setData({
        settlement: settlement,
        gameName: res.game_name,
        winnerName: winner ? winner.nickname : '',
        winnerAvatar: winner ? winner.avatar_url : '',
        winnerScore: winner ? winner.score : 0,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  goBack() {
    wx.navigateBack()
  },

  goReopen() {
    api.post('/games', {
      name: '',
      request_id: api.genRequestID()
    }).then(function(res) {
      wx.redirectTo({ url: '/pages/room/room?game_id=' + res.game_id + '&invite_token=' + res.invite_token })
    }).catch(function() {})
  },

  goShare() {
    this.showToast('长图生成功能开发中')
  },

  showToast(msg) {
    this.setData({ showToast: true, toastMsg: msg })
    if (this._toastTimer) clearTimeout(this._toastTimer)
    this._toastTimer = setTimeout(() => {
      this.setData({ showToast: false })
    }, 2200)
  },

  onShareAppMessage() {
    return {
      title: '得闲开台 — ' + this.data.gameName + ' 找数结果',
      path: '/pages/settlement/settlement?game_id=' + this.data.gameID
    }
  }
})

// pages/adjustment/adjustment.js — 补分/退分页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    gameID: 0,
    roundID: 0,
    players: [],
    selectedPlayerID: 0,
    adjustmentType: 'supplement',
    amount: '',
    reason: '',
    submitting: false
  },

  onLoad(options) {
    this.setData({
      gameID: Number(options.game_id) || 0,
      roundID: Number(options.round_id) || 0
    })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
    this.loadPlayers()
  },

  loadPlayers() {
    api.get(`/games/${this.data.gameID}`).then(res => {
      const myUserID = app.globalData.userID
      // 过滤掉自己
      const players = (res.players || []).filter(p => p.user_id !== myUserID).map(p => {
        return { ...p, avatarColor: util.avatarColor(p.nickname) }
      })
      this.setData({ players })
    })
  },

  onPlayerChange(e) {
    this.setData({ selectedPlayerID: Number(e.currentTarget.dataset.id) })
  },

  onTypeChange(e) {
    this.setData({ adjustmentType: e.currentTarget.dataset.type })
  },

  onAmountInput(e) {
    // 只允许正整数
    let val = e.detail.value.replace(/[^0-9]/g, '')
    if (val && parseInt(val, 10) > 0) {
      val = String(parseInt(val, 10))
    }
    this.setData({ amount: val })
  },

  onReasonInput(e) {
    this.setData({ reason: e.detail.value })
  },

  doSubmit() {
    if (this.data.submitting) return
    const amount = parseInt(this.data.amount, 10)
    if (!amount || amount <= 0) {
      wx.showToast({ title: '请输入正整数', icon: 'none' })
      return
    }
    if (!this.data.selectedPlayerID) {
      wx.showToast({ title: '请选择雀友', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    api.post(`/games/${this.data.gameID}/rounds/${this.data.roundID}/adjustments`, {
      to_player_id: this.data.selectedPlayerID,
      adjustment_type: this.data.adjustmentType,
      amount,
      reason: this.data.reason,
      request_id: api.genRequestID()
    }).then(res => {
      wx.showToast({ title: res.message || '已发送', icon: 'success' })
      setTimeout(() => {
        wx.navigateBack()
      }, 1500)
    }).catch(() => {
      this.setData({ submitting: false })
    })
  }
})

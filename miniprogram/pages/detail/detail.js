// pages/detail/detail.js — 单场详情页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

Page({
  data: {
    gameID: 0,
    detail: null,
    loading: true
  },

  onLoad(options) {
    // 登录/资料完善守卫：未通过弹回首页，完成后回来继续
    if (!guard.ensure(true)) return
    this.setData({ gameID: Number(options.game_id) || 0 })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
    this.loadDetail()
  },

  loadDetail() {
    this.setData({ loading: true })
    api.get(`/games/${this.data.gameID}/history`).then(res => {
      const rounds = (res.rounds || []).map(r => {
        return {
          ...r,
          submissions: (r.submissions || []).map(s => {
            return {
              ...s,
              scoreText: util.formatScore(s.score),
              scoreClass: s.score > 0 ? 'text-positive' : (s.score < 0 ? 'text-negative' : '')
            }
          })
        }
      })
      const adjustments = (res.adjustments || []).map(a => {
        return {
          ...a,
          typeText: util.adjustmentTypeText(a.adjustment_type),
          statusText: util.statusText(a.status),
          statusClass: util.statusClass(a.status),
          amountText: util.formatScore(a.amount)
        }
      })
      this.setData({
        detail: {
          ...res,
          rounds,
          adjustments
        },
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  goSettlement() {
    wx.navigateTo({
      url: `/pages/settlement/settlement?game_id=${this.data.gameID}`
    })
  }
})

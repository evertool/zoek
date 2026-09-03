// pages/settlement/settlement.js — 结算页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

Page({
  data: {
    gameID: 0,
    settlement: null,
    gameName: '',
    loading: true,
    titleMap: {}
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
    api.get(`/games/${this.data.gameID}/settlement`).then(res => {
      // 计算称号（PRD §3.5）
      const players = res.players || []
      let titleMap = {}
      if (players.length >= 2 && res.completed_rounds >= 1) {
        // 本桌冠军：最终积分最高
        const champion = players[0]
        if (champion) {
          titleMap[champion.player_id] = '本桌冠军'
        }
      }

      const settlement = {
        ...res,
        players: players.map(p => {
          return {
            ...p,
            scoreText: util.formatScore(p.total_score),
            scoreClass: p.total_score > 0 ? 'text-positive' : (p.total_score < 0 ? 'text-negative' : ''),
            title: titleMap[p.player_id] || ''
          }
        })
      }

      this.setData({
        settlement,
        gameName: res.game_name,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  goDetail() {
    wx.navigateTo({
      url: `/pages/detail/detail?game_id=${this.data.gameID}`
    })
  },

  onShareAppMessage() {
    return {
      title: `雀友记 — ${this.data.gameName} 找数结果`,
      path: `/pages/settlement/settlement?game_id=${this.data.gameID}`
    }
  }
})

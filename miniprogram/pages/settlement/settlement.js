// pages/settlement/settlement.js — 结算页 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

Page({
  data: {
    gameID: 0,
    settlement: null,
    gameName: '',
    loading: true,
    winnerName: '',
    winnerAvatar: '',
    winnerScore: 0,
    rankCard: null,
    showToast: false,
    toastMsg: ''
  },

  onLoad(options) {
    // 顶部无导航条，内容需让出状态栏 + 胶囊按钮高度
    this.setData({ navPadding: util.navPadding() })
    // 登录/资料完善守卫：未通过弹回首页，完成后回来继续
    if (!guard.ensure(true)) return
    this.setData({ gameID: Number(options.game_id) || 0 })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
    this.loadSettlement()
  },

  onShow() {
    if (!guard.pass()) return
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
          wind: p.wind || '',
          avatar_url: util.resolveAvatarURL(p.avatar_url || '')
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
      // 4 人局散台后的排位变动卡（MatchResultDialog）
      this.loadRankCard()
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  loadRankCard() {
    api.get(`/games/${this.data.gameID}/history`).then(res => {
      var changes = res.rank_changes || []
      if (!changes.length) return
      var userID = app.globalData.userID
      var mine = changes.filter(function(c) { return c.user_id === userID })[0]
      if (!mine) return
      var resultMap = { win: '胜局结算 · 连胜加成生', draw: '平局结算 · 星数保持', lose: '败局结算 · 保守扣星' }
      var lines = []
      if (mine.result === 'win') {
        lines.push('基础胜场 +1★')
        if (mine.bonus_stars > 0) lines.push('连' + mine.streak_after + '额外奖励 +' + mine.bonus_stars + '★')
      } else if (mine.result === 'lose') {
        lines.push('末位败场 -1★（九品保底不扣穿）')
      } else {
        lines.push('平局不扣星 · 连胜清零')
      }
      this.setData({
        rankCard: {
          title: resultMap[mine.result] || '排位结算',
          lines: lines,
          total: (mine.stars_delta > 0 ? '+' : '') + mine.stars_delta + '★',
          totalClass: mine.stars_delta > 0 ? 'text-positive' : (mine.stars_delta < 0 ? 'text-negative' : ''),
          streakAfter: mine.streak_after
        }
      })
    }).catch(function() {})
  },

  goRank() {
    wx.navigateTo({ url: '/pages/rank/rank' })
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

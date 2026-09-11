// pages/game-detail/game-detail.js — 牌局详情页（全场收支总览 + 转分流水，局概念已移除）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

Page({
  data: {
    capsuleTop: 0,
    capsuleHeight: 32,
    gameID: 0,
    detail: null,
    flow: [],
    loading: true,
    navPadding: 0
  },

  onLoad(options) {
    var cap = util.capsuleBox()
    this.setData({ navPadding: util.navPadding(), capsuleTop: cap.top, capsuleHeight: cap.height })
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
      var myID = Number(app.globalData.userID)
      var players = (res.players || []).map(p => ({
        ...p,
        avatar_url: util.resolveAvatarURL(p.avatar_url || ''),
        avatarColor: util.avatarColor(p.nickname),
        scoreClass: p.total_score > 0 ? 'text-positive' : (p.total_score < 0 ? 'text-negative' : ''),
        scoreText: (p.total_score > 0 ? '+' : '') + p.total_score
      }))
      var me = players.filter(function(p) { return Number(p.user_id) === myID })[0] || null
      var infoByPlayer = {}
      players.forEach(function(p) { infoByPlayer[Number(p.player_id)] = p })

      // 转分流水（新→旧）：「A → B」，头像取出分方，右侧我的净流向
      var list = (res.adjustments || []).filter(function(a) { return a.status === 'accepted' })
      list.sort(function(a, b) {
        var ta = new Date(a.created_at || 0).getTime()
        var tb = new Date(b.created_at || 0).getTime()
        return tb - ta
      })
      var flow = list.map(function(a) {
        var fromId = Number(a.from_player_id)
        var toId = Number(a.to_player_id)
        var from = infoByPlayer[fromId] || { nickname: '雀友', avatarColor: '', avatar_url: '' }
        var to = infoByPlayer[toId] || { nickname: '雀友' }
        var outgoing = fromId === Number(me ? me.player_id : 0)
        var incoming = toId === Number(me ? me.player_id : 0)
        var mine = outgoing || incoming
        var fromName = outgoing ? '我' : from.nickname
        var toName = incoming ? '我' : to.nickname
        var score = mine ? (outgoing ? -a.amount : a.amount) : 0
        return {
          id: a.id,
          fromName: from.nickname,
          fromInitial: (from.nickname || '雀')[0],
          fromColor: from.avatarColor,
          fromAvatar: from.avatar_url || '',
          desc: fromName + ' → ' + toName,
          sub: (a.reason ? a.reason + ' · ' : '') + this.formatHM(a.created_at),
          score: score,
          scoreClass: score > 0 ? 'text-positive' : (score < 0 ? 'text-negative' : 'text-secondary'),
          scoreText: score > 0 ? '+' + score : '' + score,
          timeText: this.formatHM(a.created_at)
        }
      }, this)

      var endedAt = util.toDate(res.ended_at)
      this.setData({
        detail: {
          gameName: res.game_name,
          status: res.status,
          dateText: endedAt ? (endedAt.getMonth() + 1) + '月' + endedAt.getDate() + '日' : '',
          timeText: endedAt ? this.formatHM(res.ended_at) : '',
          myScore: me ? me.total_score : 0,
          myScoreText: me ? ((me.total_score > 0 ? '+' : '') + me.total_score) : '0',
          myScoreClass: me && me.total_score > 0 ? 'text-positive' : (me && me.total_score < 0 ? 'text-negative' : ''),
          isChampion: me ? me.rank === 1 : false,
          isBalanced: players.length === 4 && players.reduce(function(s, p) { return s + p.total_score }, 0) === 0,
          flowCount: flow.length,
          playersText: players.map(function(p) { return p.nickname }).join(' / '),
          players: players
        },
        flow: flow,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  formatHM(ts) {
    var d = util.toDate(ts)
    if (!d) return ''
    return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
  },

  goBack() {
    var pages = getCurrentPages()
    if (pages.length > 1) wx.navigateBack()
    else wx.reLaunch({ url: '/pages/history/history' })
  },

  onShareAppMessage() {
    return {
      title: '「' + (this.data.detail ? this.data.detail.gameName : '得闲开台') + '」对局明细',
      path: '/pages/history/history'
    }
  }
})

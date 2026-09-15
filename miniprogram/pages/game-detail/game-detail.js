// pages/game-detail/game-detail.js — 牌局结算明细页（全场收支总览 + 转分实时流水）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

const FLOW_PAGE_SIZE = 5 // 流水默认展示条数，其余折叠

// 风位文字 → SVG 图标名（assets/icons/seat-wind-*.svg），与首页同套资源
const WIND_CLASS_MAP = { '東': 'east', '东': 'east', '南': 'south', '西': 'west', '北': 'north' }
const SEAT_WINDS = ['東', '南', '西', '北'] // seat 1-4 对应风位，wind 文字缺失时兜底

Page({
  data: {
    capsuleTop: 0,
    capsuleHeight: 32,
    gameID: 0,
    detail: null,
    flow: [],
    flowShown: FLOW_PAGE_SIZE,
    flowPageSize: FLOW_PAGE_SIZE,
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
      var myNick = app.globalData.nickname || '我'
      var myPlayerID = 0

      var sum = 0
      var players = (res.players || []).map(p => {
        var score = Number(p.total_score) || 0
        sum += score
        var isMe = Number(p.user_id) === myID
        if (isMe) myPlayerID = Number(p.player_id)
        var windText = p.wind || SEAT_WINDS[(Number(p.seat) || 1) - 1] || ''
        return {
          ...p,
          nickname: p.nickname || '雀友',
          avatar_url: util.resolveAvatarURL(p.avatar_url || ''),
          avatarColor: util.avatarColor(p.nickname),
          wind: windText,
          windClass: WIND_CLASS_MAP[windText] || '',
          isMe: isMe,
          isChampion: p.rank === 1,
          isNegative: score < 0,
          score: score,
          scoreText: (score > 0 ? '+' : '') + score,
          scoreClass: score > 0 ? 'text-positive' : (score < 0 ? 'text-negative' : 'text-secondary')
        }
      })

      var infoByPlayer = {}
      players.forEach(function(p) { infoByPlayer[Number(p.player_id)] = p })

      // 转分流水（新→旧）
      var list = (res.adjustments || []).filter(function(a) { return a.status === 'accepted' })
      list.sort(function(a, b) {
        var ta = new Date(a.created_at || 0).getTime()
        var tb = new Date(b.created_at || 0).getTime()
        return tb - ta
      })
      var flow = list.map(function(a) {
        var fromId = Number(a.from_player_id)
        var toId = Number(a.to_player_id)
        var from = infoByPlayer[fromId] || { nickname: '雀友', avatarColor: '' }
        var to = infoByPlayer[toId] || { nickname: '雀友' }
        var outgoing = fromId === myPlayerID // 我出分
        var incoming = toId === myPlayerID   // 我得分的
        var mine = outgoing || incoming
        var amount = Number(a.amount) || 0
        var delta = mine ? (outgoing ? -amount : amount) : amount
        var reason = a.reason || '转分'
        var time = this.formatHM(a.created_at)
        return {
          id: a.id,
          fromLabel: outgoing ? ('我 (' + myNick + ')') : from.nickname,
          toLabel: incoming ? ('我 (' + myNick + ')') : to.nickname,
          fromIsMe: outgoing,
          toIsMe: incoming,
          dirClass: incoming ? 'in' : (outgoing ? 'out' : 'neutral'),
          fromInitial: (from.nickname || '雀')[0],
          reasonText: reason,
          timeText: time,
          subText: time + ' · ' + reason,
          score: delta,
          scoreText: mine ? ((delta > 0 ? '+' : '') + delta) : String(amount),
          scoreClass: mine
            ? (delta > 0 ? 'text-positive' : (delta < 0 ? 'text-negative' : 'text-secondary'))
            : 'text-secondary'
        }
      }, this)

      var endedAt = util.toDate(res.ended_at)
      var shown = this.data.flowShown || FLOW_PAGE_SIZE
      if (shown > flow.length) shown = flow.length

      this.setData({
        detail: {
          gameName: res.game_name,
          status: res.status,
          statusText: util.statusText(res.status) || '已散台',
          endedText: endedAt
            ? ((endedAt.getMonth() + 1) + '月' + endedAt.getDate() + '日 ' + this.formatHM(res.ended_at) + ' 完结')
            : '',
          isBalanced: players.length > 0 && sum === 0,
          sumText: '总和 Σ = ' + (sum > 0 ? '+' : '') + sum + ' 分',
          flowCount: flow.length,
          players: players
        },
        flow: flow,
        flowShown: shown,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  loadMoreFlow() {
    var shown = (this.data.flowShown || FLOW_PAGE_SIZE) + FLOW_PAGE_SIZE
    if (shown > this.data.flow.length) shown = this.data.flow.length
    this.setData({ flowShown: shown })
  },

  collapseFlow() {
    this.setData({ flowShown: FLOW_PAGE_SIZE })
  },

  formatHM(ts) {
    var d = util.toDate(ts)
    if (!d) return ''
    return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
  },

  /** 积分走势曲线：跳转图表分析页 */
  goChart() {
    wx.navigateTo({ url: '/pages/game-chart/game-chart?game_id=' + this.data.gameID })
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

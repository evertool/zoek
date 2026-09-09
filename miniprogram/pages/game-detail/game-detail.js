// pages/game-detail/game-detail.js — 每局详情页（全场收支总览 + 逐局分值流向流水）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

const CN_NUMS = ['一', '二', '三', '四', '五', '六', '七', '八', '九', '十']

Page({
  data: {
    gameID: 0,
    detail: null,
    flow: [],
    loading: true,
    navPadding: 0
  },

  onLoad(options) {
    this.setData({ navPadding: util.navPadding() })
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
      const userID = app.globalData.userID
      var players = (res.players || []).map(p => ({
        ...p,
        avatar_url: util.resolveAvatarURL(p.avatar_url || ''),
        avatarColor: util.avatarColor(p.nickname),
        scoreClass: p.total_score > 0 ? 'text-positive' : (p.total_score < 0 ? 'text-negative' : ''),
        scoreText: (p.total_score > 0 ? '+' : '') + p.total_score
      }))
      var me = players.filter(function(p) { return p.user_id === userID })[0] || null

      // 逐局分值流向流水（新→旧），含改分修正，右侧展示我的净流向
      var events = []
      ;(res.rounds || []).forEach(r => {
        var subs = r.submissions || []
        var mySub = subs.filter(function(s) { return s.user_id === userID })[0]
        var myScore = mySub ? mySub.score : 0
        if (!subs.length) {
          events.push({
            type: 'round', time: r.locked_at || r.created_at || '',
            numCn: CN_NUMS[r.round_number - 1] || r.round_number,
            title: '第 ' + r.round_number + ' 局 · 未开局', sub: '—',
            score: 0, timeText: this.formatHM(r.locked_at || r.created_at)
          })
          return
        }
        var winner = subs.slice().sort(function(a, b) { return b.score - a.score })[0]
        var zero = subs.every(function(s) { return s.score === 0 })
        var sub
        if (zero) {
          sub = '流局 · 全家无得失'
        } else {
          var losers = subs.filter(function(s) { return s.score < 0 })
          sub = losers.length === 1
            ? (losers[0].nickname + ' 付 · ' + winner.nickname + ' 收')
            : (winner.nickname + ' 收 · ' + losers.length + '家各付 ' + Math.abs(losers[0] ? losers[0].score : 0))
        }
        events.push({
          type: 'round',
          time: r.locked_at || r.created_at || '',
          numCn: CN_NUMS[r.round_number - 1] || r.round_number,
          title: '第 ' + r.round_number + ' 局 · ' + (zero ? '流局荒庄' : winner.nickname + (losers.length === 1 ? '胡' : '自摸')),
          sub: sub,
          score: myScore,
          timeText: this.formatHM(r.locked_at || r.created_at)
        })
      }, this)
      ;(res.adjustments || []).forEach(a => {
        if (a.status !== 'accepted') return
        var fromName = this.nameOf(players, a.from_user_id)
        var toName = this.nameOf(players, a.to_user_id)
        var myDelta = 0
        if (a.to_user_id === userID) myDelta = a.amount
        else if (a.from_user_id === userID) myDelta = -a.amount
        events.push({
          type: 'adjust',
          time: a.created_at || '',
          numCn: CN_NUMS[(a.round_number || 1) - 1] || '',
          title: '第' + (a.round_number || '-') + '局 补分结算',
          sub: fromName + ' 补偿转入 ' + toName + (a.reason ? ' · ' + a.reason : ''),
          score: myDelta,
          timeText: this.formatHM(a.created_at)
        })
      })
      events.sort(function(a, b) { return (b.time || '') < (a.time || '') ? -1 : 1 })
      var flow = events.map(ev => ({
        ...ev,
        scoreClass: ev.score > 0 ? 'text-positive' : (ev.score < 0 ? 'text-negative' : 'text-secondary'),
        scoreText: ev.score > 0 ? '+' + ev.score : '' + ev.score
      }))

      var endedAt = res.ended_at ? new Date(res.ended_at) : null
      this.setData({
        detail: {
          gameName: res.game_name,
          status: res.status,
          completedRounds: res.completed_rounds || 0,
          dateText: endedAt ? (endedAt.getMonth() + 1) + '月' + endedAt.getDate() + '日' : '',
          timeText: endedAt ? this.formatHM(res.ended_at) : '',
          myScore: me ? me.total_score : 0,
          myScoreText: me ? ((me.total_score > 0 ? '+' : '') + me.total_score) : '0',
          myScoreClass: me && me.total_score > 0 ? 'text-positive' : (me && me.total_score < 0 ? 'text-negative' : ''),
          isChampion: me ? me.rank === 1 : false,
          isBalanced: players.length === 4 && players.reduce(function(s, p) { return s + p.total_score }, 0) === 0,
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

  nameOf(players, userID) {
    var p = players.filter(function(x) { return x.user_id === userID })[0]
    return p ? p.nickname : '雀友'
  },

  formatHM(ts) {
    if (!ts) return ''
    var d = new Date(ts)
    if (isNaN(d.getTime())) return ''
    return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
  },

  goBack() {
    var pages = getCurrentPages()
    if (pages.length > 1) wx.navigateBack()
    else wx.reLaunch({ url: '/pages/history/history' })
  },

  goChart() {
    wx.navigateTo({ url: '/pages/game-chart/game-chart?game_id=' + this.data.gameID })
  },

  goLedger() {
    wx.navigateTo({ url: '/pages/game-ledger/game-ledger?game_id=' + this.data.gameID })
  },

  onShareAppMessage() {
    return {
      title: '「' + (this.data.detail ? this.data.detail.gameName : '得闲开台') + '」对局明细',
      path: '/pages/history/history'
    }
  }
})

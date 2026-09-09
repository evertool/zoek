// pages/game-ledger/game-ledger.js — 手帐明细页（雀王功名录 + 逐局流水手帐）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

const TILE_TAGS = ['', '一筒', '二筒', '三筒', '四筒', '五筒', '六筒', '七筒', '八筒', '九筒', '十筒']

Page({
  data: {
    gameID: 0,
    detail: null,
    rounds: [],
    awards: [],
    photoPath: '',
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
      const players = (res.players || []).map(p => ({
        ...p,
        avatar_url: util.resolveAvatarURL(p.avatar_url || ''),
        avatarColor: util.avatarColor(p.nickname),
        scoreClass: p.total_score > 0 ? 'text-positive' : (p.total_score < 0 ? 'text-negative' : ''),
        scoreText: (p.total_score > 0 ? '+' : '') + p.total_score
      }))
      const playerByUser = {}
      players.forEach(function(p) { playerByUser[p.user_id] = p })

      var rounds = (res.rounds || []).map(r => {
        var subs = (r.submissions || []).slice().sort(function(a, b) {
          return (playerByUser[a.user_id] ? playerByUser[a.user_id].seat : 9) - (playerByUser[b.user_id] ? playerByUser[b.user_id].seat : 9)
        })
        var winner = subs.slice().sort(function(a, b) { return b.score - a.score })[0]
        var zero = subs.every(function(s) { return s.score === 0 })
        var losers = subs.filter(function(s) { return s.score < 0 })
        var desc = zero ? '流局荒庄' : (winner.nickname + (losers.length === 1 ? '胡' : '自摸自立'))
        var lockedText = ''
        if (r.locked_at) {
          var d = new Date(r.locked_at)
          lockedText = (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes() + ' 鎖定'
        }
        return {
          roundId: r.round_id,
          roundNumber: r.round_number,
          tileTag: TILE_TAGS[r.round_number] || r.round_number + '筒',
          desc: '第 ' + r.round_number + ' 局 · ' + desc,
          metaText: lockedText + (losers.length === 1 ? ' · ' + losers[0].nickname + '出銃' : zero ? ' · 海底無牌' : ' · 三家賠付'),
          subs: subs.map(s => ({
            ...s,
            scoreClass: s.score > 0 ? 'text-positive' : (s.score < 0 ? 'text-negative' : 'text-secondary'),
            scoreText: (s.score > 0 ? '+' : '') + s.score
          }))
        }
      })

      this.setData({
        detail: this.buildMeta(res, players),
        rounds: rounds,
        awards: this.buildAwards(players, rounds),
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  buildMeta(res, players) {
    var endedAt = res.ended_at ? new Date(res.ended_at) : null
    var timeText = ''
    if (endedAt) {
      timeText = (endedAt.getHours() < 10 ? '0' : '') + endedAt.getHours() + ':' + (endedAt.getMinutes() < 10 ? '0' : '') + endedAt.getMinutes()
    }
    var zeroSum = players.reduce(function(s, p) { return s + p.total_score }, 0) === 0
    return {
      dateText: endedAt ? (endedAt.getMonth() + 1) + '月' + endedAt.getDate() + '日' : '',
      timeText: timeText,
      roundsText: (res.completed_rounds || 0) + ' 局',
      zeroSum: zeroSum,
      players: players
    }
  },

  // 雀王功名录：本台冠军 / 单局食大茶饭 / 稳如泰山
  buildAwards(players, rounds) {
    var awards = []
    var champion = players.slice().sort(function(a, b) { return b.total_score - a.total_score })[0]
    if (champion) {
      awards.push({
        icon: '👑', name: '本台冠军', winner: champion.nickname,
        detail: (champion.total_score > 0 ? '+' : '') + champion.total_score + '分', hero: true
      })
    }
    // 单局食大茶饭：全场单局最高得分
    var best = { score: -1, nickname: '', round: 0 }
    rounds.forEach(function(r) {
      r.subs.forEach(function(s) {
        if (s.score > best.score) best = { score: s.score, nickname: s.nickname, round: r.roundNumber }
      })
    })
    if (best.score > 0) {
      awards.push({ icon: '⚡', name: '单局食大茶饭', winner: best.nickname, detail: '+' + best.score + '分 (第' + best.round + '局)' })
    }
    // 稳如泰山：最长连续不败（≥0）局数
    var streaks = {}
    players.forEach(function(p) { streaks[p.user_id] = { nickname: p.nickname, cur: 0, best: 0 } })
    rounds.forEach(function(r) {
      Object.keys(streaks).forEach(function(uid) {
        var sub = r.subs.filter(function(s) { return String(s.user_id) === String(uid) })[0]
        if (sub && sub.score >= 0) {
          streaks[uid].cur++
          if (streaks[uid].cur > streaks[uid].best) streaks[uid].best = streaks[uid].cur
        } else {
          streaks[uid].cur = 0
        }
      })
    })
    var stable = null
    Object.keys(streaks).forEach(function(uid) {
      if (!stable || streaks[uid].best > stable.best) stable = streaks[uid]
    })
    if (stable && stable.best > 0) {
      awards.push({ icon: '🛡', name: '稳如泰山', winner: stable.nickname, detail: '连续' + stable.best + '局不败' })
    }
    return awards
  },

  goRound(e) {
    wx.navigateTo({ url: '/pages/game-detail/game-detail?game_id=' + this.data.gameID })
  },

  goBack() {
    var pages = getCurrentPages()
    if (pages.length > 1) wx.navigateBack()
    else wx.reLaunch({ url: '/pages/history/history' })
  },

  // 合影留念：本机选图临时预览（不上传）
  pickPhoto() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      success: (res) => {
        if (res.tempFiles && res.tempFiles[0]) {
          this.setData({ photoPath: res.tempFiles[0].tempFilePath })
        }
      }
    })
  },

  // 原班雀友再开一局：开新台后原桌友扫码/邀请再加入
  rematch() {
    api.post('/games', {
      name: '',
      request_id: api.genRequestID()
    }).then(res => {
      wx.redirectTo({ url: '/pages/room/room?game_id=' + res.game_id })
    })
  },

  onShareAppMessage() {
    return {
      title: '「' + (this.data.detail ? this.data.detail.dateText : '') + '」手帐明细',
      path: '/pages/history/history'
    }
  }
})

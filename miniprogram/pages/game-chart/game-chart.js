// pages/game-chart/game-chart.js — 图表分析页（积分走势 + 战局复盘 + 逐局净胜负）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

const LINE_COLORS = ['#1b6b4a', '#0284c7', '#d97706', '#b91c1c']

Page({
  data: {
    capsuleTop: 0,
    capsuleHeight: 32,
    gameID: 0,
    detail: null,
    players: [],
    insights: [],
    diffText: '0.00',
    pressureText: '',
    loading: true,
    navPadding: 0,
    // 图例筛选（点击隐藏/显示某玩家折线）
    hidden: {}
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
  },

  onReady() {
    if (this.data.gameID) this.loadDetail()
  },

  loadDetail() {
    this.setData({ loading: true })
    api.get(`/games/${this.data.gameID}/history`).then(res => {
      var players = (res.players || []).map(function(p, idx) {
        return { ...p, color: LINE_COLORS[idx % 4], show: true }
      })
      var byUser = {}
      players.forEach(function(p) { byUser[p.user_id] = p })

      // 按局号升序，取每位玩家每局得分 → 累计序列
      var rounds = (res.rounds || []).slice().sort(function(a, b) { return a.round_number - b.round_number })
      var series = {}
      players.forEach(function(p) { series[p.user_id] = [0] })
      var roundScores = []
      rounds.forEach(function(r) {
        var rs = { round: r.round_number, scores: {} }
        ;(r.submissions || []).forEach(function(s) {
          if (!series[s.user_id]) return
          var next = series[s.user_id][series[s.user_id].length - 1] + s.score
          series[s.user_id].push(next)
          rs.scores[s.user_id] = s.score
        })
        roundScores.push(rs)
      })
      players.forEach(function(p) {
        p.cum = series[p.user_id] || [0]
        p.finalScore = p.cum[p.cum.length - 1] || 0
        p.scoreClass = p.finalScore > 0 ? 'text-positive' : (p.finalScore < 0 ? 'text-negative' : '')
        p.tag = this.playerTag(p, roundScores)
      }, this)

      var totalFinal = players.reduce(function(s, p) { return s + p.finalScore }, 0)
      var winners = players.filter(function(p) { return p.finalScore === Math.max.apply(null, players.map(function(x) { return x.finalScore })) })
      var pressure = ''
      if (winners.length && winners[0].finalScore > 0) {
        var leadRounds = this.leadingRounds(winners[0], players)
        pressure = '水上赢面：' + winners[0].nickname + '（全场压制 ' + leadRounds + '/' + rounds.length + ' 局）'
      }

      this.setData({
        detail: {
          gameName: res.game_name || '得闲开台',
          dateText: this.dateText(res.ended_at || res.created_at),
          roundsText: (res.completed_rounds || rounds.length) + '局满编',
          zeroSum: totalFinal === 0
        },
        players: players,
        insights: this.buildInsights(players, roundScores),
        diffText: Math.abs(totalFinal).toFixed(2),
        pressureText: pressure,
        loading: false
      })
      // 等节点渲染完成后画图
      setTimeout(() => this.drawCharts(), 100)
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  // 玩家小卡标签：全场压制 / 后盘逆袭 / 逆风韧性 / 尾盘吃炮 / 峰值
  playerTag(p, roundScores) {
    var cum = p.cum
    var maxCum = Math.max.apply(null, cum)
    var minCum = Math.min.apply(null, cum)
    var finalScore = cum[cum.length - 1]
    if (minCum >= 0 && finalScore > 0) return '全场压制'
    var crossed = false
    for (var i = 1; i < cum.length; i++) {
      if (cum[i - 1] < 0 && cum[i] >= 0) crossed = true
    }
    if (crossed && finalScore > 0) return '后盘逆袭'
    if (finalScore < 0) {
      var lastRound = null
      for (var j = roundScores.length - 1; j >= 0; j--) {
        if (roundScores[j].scores[p.user_id] !== undefined) { lastRound = roundScores[j].scores[p.user_id]; break }
      }
      if (lastRound !== null && lastRound < 0) return '尾盘吃炮'
      return '逆风韧性'
    }
    return '峰值 +' + maxCum
  },

  // 控盘王的领先局数（累计分严格第一的局数）
  leadingRounds(p, players) {
    var count = 0
    for (var i = 0; i < p.cum.length; i++) {
      var lead = true
      for (var j = 0; j < players.length; j++) {
        if (players[j].user_id !== p.user_id && players[j].cum[i] > p.cum[i]) lead = false
      }
      if (lead) count++
    }
    return count
  },

  buildInsights(players, roundScores) {
    var insights = []
    // 1. 单局最大爆发
    var burst = { score: -999, user: null, round: 0 }
    roundScores.forEach(function(rs) {
      Object.keys(rs.scores).forEach(function(uid) {
        if (rs.scores[uid] > burst.score) burst = { score: rs.scores[uid], user: uid, round: rs.round }
      })
    })
    if (burst.user) {
      var bp = players.filter(function(p) { return String(p.user_id) === String(burst.user) })[0]
      if (bp) {
        insights.push({
          icon: '⚡', title: '单局最大爆发',
          highlight: '第' + burst.round + '局 · ' + bp.nickname + ' (+' + burst.score + ')',
          desc: '单局斩获三家进账，直接扭转开局赤字。'
        })
      }
    }
    // 2. 全场控盘王：最终赢家连续正收益局数
    var champion = players.slice().sort(function(a, b) { return b.finalScore - a.finalScore })[0]
    if (champion && champion.finalScore > 0) {
      var streak = 0, best = 0
      for (var i = 1; i < champion.cum.length; i++) {
        if (champion.cum[i] >= champion.cum[i - 1]) { streak++; if (streak > best) best = streak }
        else streak = 0
      }
      insights.push({
        icon: '📈', title: '全场控盘王',
        highlight: champion.nickname + ' (连续' + best + '局正收益)',
        desc: '全程稳居水面零轴之上，保持绝对优势跑赢全场。'
      })
    }
    // 3. 局间振幅极差分析：单局得分摆幅最小者最稳健
    var swings = players.map(function(p) {
      var mine = roundScores.map(function(rs) { return rs.scores[p.user_id] }).filter(function(s) { return s !== undefined })
      if (!mine.length) return { p: p, swing: 0 }
      return { p: p, swing: Math.max.apply(null, mine) - Math.min.apply(null, mine) }
    }).filter(function(x) { return x.p.finalScore !== 0 || true })
    swings.sort(function(a, b) { return a.swing - b.swing })
    if (swings.length) {
      var maxSwing = Math.max.apply(null, swings.map(function(x) { return x.swing }))
      insights.push({
        icon: '🎴', title: '局间振幅极差分析',
        highlight: '极差 ' + maxSwing + '分 / 稳健度首位 ' + swings[0].p.nickname,
        desc: swings[0].p.nickname + '：局间振幅仅 ±' + Math.round(swings[0].swing / 2) + ' 分，打法防守滴水不漏。'
      })
    }
    return insights
  },

  dateText(ts) {
    var d = util.toDate(ts)
    if (!d) return ''
    return (d.getMonth() + 1) + '月' + d.getDate() + '日 散台圆满'
  },

  onLegendTap(e) {
    var uid = e.currentTarget.dataset.uid
    var players = this.data.players.map(p => (String(p.user_id) === String(uid) ? { ...p, show: !p.show } : p))
    this.setData({ players: players })
    this.drawCharts()
  },

  // Canvas 2D 折线图 + 柱状图
  drawCharts() {
    const query = wx.createSelectorQuery().in(this)
    query.select('#line-chart').fields({ node: true, size: true }).exec(res => {
      if (!res || !res[0] || !res[0].node) return
      this.drawLineChart(res[0].node, res[0].width, res[0].height)
    })
    query.select('#bar-chart').fields({ node: true, size: true }).exec(res2 => {
      if (!res2 || !res2[0] || !res2[0].node) return
      this.drawBarChart(res2[0].node, res2[0].width, res2[0].height)
    })
  },

  chartContext(node, width, height) {
    const dpr = wx.getWindowInfo ? wx.getWindowInfo().pixelRatio : 2
    node.width = width * dpr
    node.height = height * dpr
    const ctx = node.getContext('2d')
    ctx.scale(dpr, dpr)
    return ctx
  },

  drawLineChart(node, width, height) {
    const ctx = this.chartContext(node, width, height)
    var players = this.data.players.filter(function(p) { return p.show !== false })
    if (!players.length) return
    var padL = 34, padR = 34, padT = 16, padB = 22
    var w = width - padL - padR, h = height - padT - padB
    var n = players[0].cum.length // 局数 + 1（含起手 0）
    var all = []
    players.forEach(function(p) { all = all.concat(p.cum) })
    var maxV = Math.max.apply(null, all), minV = Math.min.apply(null, all)
    if (maxV === minV) { maxV += 10; minV -= 10 }
    var span = maxV - minV
    maxV += span * 0.1; minV -= span * 0.1
    var x = function(i) { return padL + (n <= 1 ? w / 2 : (i / (n - 1)) * w) }
    var y = function(v) { return padT + (1 - (v - minV) / (maxV - minV)) * h }

    var drawStatic = function() {
      // 0 基线虚线
      ctx.strokeStyle = '#bfc9c0'; ctx.setLineDash([4, 4]); ctx.lineWidth = 1
      ctx.beginPath(); ctx.moveTo(padL, y(0)); ctx.lineTo(width - padR, y(0)); ctx.stroke()
      ctx.setLineDash([])
      if (minV < 0 && maxV > 0) {
        ctx.fillStyle = '#6f7a72'; ctx.font = '10px sans-serif'; ctx.textAlign = 'right'
        ctx.fillText('0基准', padL - 4, y(0) + 3)
      }
      // x 轴标签
      ctx.fillStyle = '#6f7a72'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center'
      ctx.fillText('起手', x(0), height - 6)
      for (var i = 1; i < n; i++) ctx.fillText(i + '局', x(i), height - 6)
    }

    var drawLines = function() {
      players.forEach(function(p) {
        ctx.strokeStyle = p.color; ctx.lineWidth = 2; ctx.lineJoin = 'round'
        ctx.beginPath()
        p.cum.forEach(function(v, i) { i === 0 ? ctx.moveTo(x(i), y(v)) : ctx.lineTo(x(i), y(v)) })
        ctx.stroke()
        // 终点圆点 + 数值
        var lx = x(n - 1), ly = y(p.cum[n - 1])
        ctx.fillStyle = p.color
        ctx.beginPath(); ctx.arc(lx, ly, 3, 0, Math.PI * 2); ctx.fill()
        ctx.font = 'bold 10px sans-serif'
        ctx.fillText((p.cum[n - 1] > 0 ? '+' : '') + p.cum[n - 1], lx + 5, ly + 3)
      })
    }

    // 线性递画动画：折线从左到右匀速绘出
    var duration = 900
    var start = Date.now()
    var tick = function() {
      var t = Math.min(1, (Date.now() - start) / duration)
      ctx.clearRect(0, 0, width, height)
      drawStatic()
      ctx.save()
      ctx.beginPath()
      ctx.rect(0, 0, padL + (w + padR) * t + 2, height)
      ctx.clip()
      drawLines()
      ctx.restore()
      if (t < 1 && node.requestAnimationFrame) {
        node.requestAnimationFrame(tick)
      }
    }
    if (node.requestAnimationFrame) {
      node.requestAnimationFrame(tick)
    } else {
      drawStatic(); drawLines()
    }
  },

  drawBarChart(node, width, height) {
    const ctx = this.chartContext(node, width, height)
    var players = this.data.players
    var rounds = this.data.players[0] ? players[0].cum.length - 1 : 0
    if (rounds <= 0) return
    var padL = 6, padR = 6, padT = 10, padB = 26
    var w = width - padL - padR, h = height - padT - padB
    var allScores = []
    // 每局各玩家单局分
    var grid = []
    for (var r = 1; r <= rounds; r++) {
      var row = players.map(function(p) { return p.cum[r] - p.cum[r - 1] })
      grid.push(row)
      allScores = allScores.concat(row)
    }
    var maxAbs = Math.max.apply(null, allScores.map(function(s) { return Math.abs(s) })) || 1
    var zeroY = padT + h / 2
    var groupW = w / rounds
    var barW = Math.min(6, groupW / (players.length + 1))

    // 0 轴
    ctx.strokeStyle = '#e5e7eb'; ctx.lineWidth = 1
    ctx.beginPath(); ctx.moveTo(padL, zeroY); ctx.lineTo(width - padR, zeroY); ctx.stroke()

    grid.forEach(function(row, ri) {
      var gx = padL + ri * groupW + groupW / 2
      row.forEach(function(score, pi) {
        var p = players[pi]
        var bx = gx - (players.length * barW) / 2 + pi * barW
        var bh = (Math.abs(score) / maxAbs) * (h / 2 - 4)
        ctx.fillStyle = score >= 0 ? '#1b6b4a' : '#b91c1c'
        if (score >= 0) ctx.fillRect(bx, zeroY - bh, barW - 1.5, bh)
        else ctx.fillRect(bx, zeroY, barW - 1.5, bh)
      })
      // 每局赢家分值
      var best = Math.max.apply(null, row)
      if (best > 0) {
        ctx.fillStyle = '#1b6b4a'; ctx.font = 'bold 9px sans-serif'; ctx.textAlign = 'center'
        ctx.fillText('+' + best, gx, height - 14)
      } else {
        ctx.fillStyle = '#6f7a72'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center'
        ctx.fillText('荒庄', gx, height - 14)
      }
      ctx.fillStyle = '#6f7a72'; ctx.font = '8px sans-serif'
      ctx.fillText('G' + (ri + 1), gx, height - 4)
    })
  },

  goBack() {
    var pages = getCurrentPages()
    if (pages.length > 1) wx.navigateBack()
    else wx.reLaunch({ url: '/pages/history/history' })
  }
})

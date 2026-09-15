// pages/game-chart/game-chart.js — 图表分析页（累计走势 + 战局复盘 + 逐笔流水条形图）
// 局概念已移除：走势与条形图全部以「逐笔转分流水」为步进，不再按局取数
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

const LINE_COLORS = ['#1b6b4a', '#0284c7', '#d97706', '#b91c1c']
const SEAT_WINDS = ['東', '南', '西', '北']
// 风位文字 → SVG 图标名（assets/icons/seat-wind-*.svg），与首页/明细页同套资源
const WIND_CLASS_MAP = { '東': 'east', '东': 'east', '南': 'south', '西': 'west', '北': 'north' }

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
    flowCount: 0,
    flowBarHeight: 240, // 逐笔条形图 canvas 高度（px，随笔数伸缩）
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
        var windText = p.wind || SEAT_WINDS[(Number(p.seat) || 1) - 1] || ''
        return { ...p, wind: windText, windClass: WIND_CLASS_MAP[windText] || '', color: LINE_COLORS[idx % 4], show: true }
      })
      var byPlayerID = {}
      players.forEach(function(p) { byPlayerID[Number(p.player_id)] = p })

      // 逐笔流水（旧→新）：每笔转分是走势图的一个步进
      var entries = (res.adjustments || []).filter(function(a) { return a.status === 'accepted' })
      entries.sort(function(a, b) {
        var ta = new Date(a.created_at || 0).getTime() || (a.id || 0)
        var tb = new Date(b.created_at || 0).getTime() || (b.id || 0)
        return ta - tb
      })

      var cum = {}
      players.forEach(function(p) { cum[Number(p.player_id)] = [0] })
      var flowSteps = []
      entries.forEach(function(a) {
        var fromP = byPlayerID[Number(a.from_player_id)]
        var toP = byPlayerID[Number(a.to_player_id)]
        var amount = Number(a.amount) || 0
        // 每个玩家都推进一个点（未参与的保持原值），保证各序列等长、x 轴对齐
        Object.keys(cum).forEach(function(pid) {
          var v = cum[pid][cum[pid].length - 1]
          if (fromP && Number(pid) === Number(fromP.player_id)) v -= amount
          if (toP && Number(pid) === Number(toP.player_id)) v += amount
          cum[pid].push(v)
        })
        flowSteps.push({ fromP: fromP, toP: toP, amount: amount })
      })

      players.forEach(function(p) {
        p.cum = cum[Number(p.player_id)] || [0]
        p.finalScore = p.cum[p.cum.length - 1] || 0
        p.scoreClass = p.finalScore > 0 ? 'text-positive' : (p.finalScore < 0 ? 'text-negative' : '')
        p.tag = this.playerTag(p, flowSteps)
      }, this)
      // 条形图绘制时直接取用（不进 data，避免 setData 大对象）
      this._flowSteps = flowSteps

      var totalFinal = players.reduce(function(s, p) { return s + p.finalScore }, 0)
      var winners = players.filter(function(p) { return p.finalScore === Math.max.apply(null, players.map(function(x) { return x.finalScore })) })
      var pressure = ''
      if (winners.length && winners[0].finalScore > 0) {
        var leadSteps = this.leadingSteps(winners[0], players)
        pressure = '水上赢面：' + winners[0].nickname + '（全程压制 ' + leadSteps + '/' + flowSteps.length + ' 笔）'
      }

      this.setData({
        detail: {
          gameName: res.game_name || '未命名牌局',
          dateText: this.dateText(res.ended_at || res.created_at),
          flowText: '共 ' + flowSteps.length + ' 笔流水',
          zeroSum: totalFinal === 0
        },
        players: players,
        insights: this.buildInsights(players, flowSteps),
        diffText: Math.abs(totalFinal).toFixed(2),
        pressureText: pressure,
        flowCount: flowSteps.length,
        flowBarHeight: Math.min(640, Math.max(240, flowSteps.length * 30 + 50)),
        loading: false
      })
      // 等节点渲染完成后画图
      setTimeout(() => this.drawCharts(), 100)
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  // 玩家小卡标签：全场压制 / 后盘逆袭 / 逆风韧性 / 尾盘失血 / 峰值
  playerTag(p) {
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
      var lastStep = null
      for (var j = cum.length - 1; j >= 1; j--) {
        if (cum[j] !== cum[j - 1]) { lastStep = cum[j] - cum[j - 1]; break }
      }
      if (lastStep !== null && lastStep < 0) return '尾盘失血'
      return '逆风韧性'
    }
    return '峰值 +' + maxCum
  },

  // 控盘王的领先笔数（累计分严格第一的步数）
  leadingSteps(p, players) {
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

  buildInsights(players, flowSteps) {
    var insights = []
    if (!flowSteps.length) return insights
    // 1. 单笔最大进账
    var burst = { amount: -1, step: null }
    flowSteps.forEach(function(fs, i) {
      if (fs.amount > burst.amount) burst = { amount: fs.amount, step: fs, idx: i }
    })
    if (burst.step && burst.step.toP) {
      insights.push({
        icon: '⚡', title: '单笔最大进账',
        highlight: '第' + (burst.idx + 1) + '笔 · ' + burst.step.toP.nickname + ' (+' + burst.amount + ')',
        desc: '最大单笔转分进账，账面直接被这一笔拉起。'
      })
    }
    // 2. 全场控盘王：最终赢家连续正收益笔数
    var champion = players.slice().sort(function(a, b) { return b.finalScore - a.finalScore })[0]
    if (champion && champion.finalScore > 0) {
      var streak = 0, best = 0
      for (var i = 1; i < champion.cum.length; i++) {
        if (champion.cum[i] >= champion.cum[i - 1]) { streak++; if (streak > best) best = streak }
        else streak = 0
      }
      insights.push({
        icon: '📈', title: '全场控盘王',
        highlight: champion.nickname + ' (连续' + best + '笔正收益)',
        desc: '全程稳居水面零轴之上，保持绝对优势跑赢全场。'
      })
    }
    // 3. 笔间振幅极差分析：单笔得失摆幅最小者最稳健
    var swings = players.map(function(p) {
      var mine = []
      for (var i = 1; i < p.cum.length; i++) mine.push(p.cum[i] - p.cum[i - 1])
      if (!mine.length) return { p: p, swing: 0 }
      return { p: p, swing: Math.max.apply(null, mine) - Math.min.apply(null, mine) }
    })
    swings.sort(function(a, b) { return a.swing - b.swing })
    if (swings.length) {
      var maxSwing = Math.max.apply(null, swings.map(function(x) { return x.swing }))
      insights.push({
        icon: '🎴', title: '笔间振幅极差分析',
        highlight: '极差 ' + maxSwing + '分 / 稳健度首位 ' + swings[0].p.nickname,
        desc: swings[0].p.nickname + '：笔间振幅仅 ±' + Math.round(swings[0].swing / 2) + ' 分，打法防守滴水不漏。'
      })
    }
    return insights
  },

  dateText(ts) {
    var d = util.toDate(ts)
    if (!d) return ''
    return (d.getMonth() + 1) + '月' + d.getDate() + '日 已散台'
  },

  onLegendTap(e) {
    var uid = e.currentTarget.dataset.uid
    var players = this.data.players.map(p => (String(p.user_id) === String(uid) ? { ...p, show: !p.show } : p))
    this.setData({ players: players })
    this.drawCharts()
  },

  // Canvas 2D 折线图 + 逐笔条形图（节点可能晚于首查渲染，重试兜底）
  // 注意：两张画布各建独立的 SelectorQuery——同一 query 复用 exec 在真机上会静默失败
  drawCharts(retry) {
    retry = retry || 0
    const query = wx.createSelectorQuery().in(this)
    query.select('#line-chart').fields({ node: true, size: true }).exec(res => {
      if (res && res[0] && res[0].node) {
        this.drawLineChart(res[0].node, res[0].width, res[0].height)
      } else if (retry < 5) {
        setTimeout(() => this.drawCharts(retry + 1), 200)
      }
    })
    const barQuery = wx.createSelectorQuery().in(this)
    barQuery.select('#bar-chart').fields({ node: true, size: true }).exec(res2 => {
      if (res2 && res2[0] && res2[0].node) {
        this.drawFlowBarChart(res2[0].node, res2[0].width, res2[0].height)
      }
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
    var padL = 34, padR = 34, padT = 16, padB = 12
    var w = width - padL - padR, h = height - padT - padB
    var n = players[0].cum.length // 笔数 + 1（含起手 0）
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
      // x 轴不再标注笔数
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

  // 逐笔流水条形图（水平条，旧→新自上而下）：
  // 我收到 → 绿条向右；我转出 → 红条向左；他人互转 → 灰条向右弱显示
  drawFlowBarChart(node, width, height) {
    const ctx = this.chartContext(node, width, height)
    var myID = Number(app.globalData.userID)
    var steps = this._flowSteps || []
    if (!steps.length) {
      ctx.fillStyle = '#6f7a72'; ctx.font = '13px sans-serif'; ctx.textAlign = 'center'
      ctx.fillText('暂无转分流水', width / 2, height / 2)
      return
    }
    var padL = 14, padR = 52, padT = 8, padB = 8
    var w = width - padL - padR
    var rows = steps.length
    var rowH = (height - padT - padB) / rows
    var maxAbs = Math.max.apply(null, steps.map(function(s) { return s.amount })) || 1
    // 0 轴位置：给左向红条留 1/3 空间；条长上限取两侧剩余宽度的较小者，避免画出画布
    var x0 = padL + w * 0.62
    var maxW = w * 0.34

    // 0 轴
    ctx.strokeStyle = '#e5e7eb'; ctx.lineWidth = 1
    ctx.beginPath(); ctx.moveTo(x0, padT); ctx.lineTo(x0, height - padB); ctx.stroke()

    steps.forEach(function(s, i) {
      var cy = padT + rowH * i + rowH / 2
      var barH = Math.min(14, rowH - 6)
      var outgoing = s.fromP && Number(s.fromP.user_id) === myID
      var incoming = s.toP && Number(s.toP.user_id) === myID
      // 行号
      ctx.fillStyle = '#9ca3af'; ctx.font = '9px sans-serif'; ctx.textAlign = 'left'
      ctx.fillText(String(i + 1), padL, cy + 3)
      // 条
      var bw = Math.max(2, (s.amount / maxAbs) * maxW)
      if (incoming) {
        ctx.fillStyle = '#1b6b4a'
        ctx.fillRect(x0, cy - barH / 2, bw, barH)
      } else if (outgoing) {
        ctx.fillStyle = '#b91c1c'
        ctx.fillRect(x0 - bw, cy - barH / 2, bw, barH)
      } else {
        ctx.fillStyle = '#cbd5e1'
        ctx.fillRect(x0, cy - barH / 2, bw, barH)
      }
      // 金额
      ctx.font = 'bold 10px sans-serif'; ctx.textAlign = 'right'
      if (incoming) { ctx.fillStyle = '#1b6b4a'; ctx.fillText('+' + s.amount, width - padR + 46, cy + 3) }
      else if (outgoing) { ctx.fillStyle = '#b91c1c'; ctx.fillText('-' + s.amount, width - padR + 46, cy + 3) }
      else { ctx.fillStyle = '#6f7a72'; ctx.fillText(String(s.amount), width - padR + 46, cy + 3) }
    })
  },

  goBack() {
    var pages = getCurrentPages()
    if (pages.length > 1) wx.navigateBack()
    else wx.reLaunch({ url: '/pages/history/history' })
  }
})

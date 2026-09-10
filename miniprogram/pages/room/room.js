// pages/room/room.js — 台间页 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

// 安全加载 lottie（npm 构建失败时不会阻断页面）
let lottie = null
try {
  lottie = require('lottie-miniprogram')
} catch (e) {
  console.warn('lottie-miniprogram not available, swap animation disabled')
}

const POLL_INTERVAL = 4000
const LEDGER_PAGE_SIZE = 5 // 流水账单每页条数
const WINDS = ['東', '南', '西', '北']
const WIND_CLASSES = ['east', 'south', 'west', 'north']

Page({
  data: {
    gameID: 0,
    game: null,
    players: [],
    seats: [],
    ledger: [],
    inviteToken: '',
    qrPath: '',
    qrLoading: false,
    loading: true,
    loadError: false,
    isOwner: false,
    hasScores: false,
    showQrModal: false,
    showScoreModal: false,
    ledgerShown: LEDGER_PAGE_SIZE,
    ledgerPageSize: LEDGER_PAGE_SIZE,
    showSwapModal: false,
    swapTargetSeat: 0,
    showIncomingSwap: false,
    incomingSwap: null,
    scoreTargetSeat: '',
    scoreTargetName: '',
    currentScore: 0,
    swapTargetText: '',
    showToast: false,
    toastMsg: '',
    showSwapAnim: false
  },

  onLoad(options) {
    // 顶部无导航条，内容需让出状态栏 + 胶囊按钮高度
    var cap = util.capsuleBox()
    this.setData({ navPadding: util.navPadding(), capsuleTop: cap.top, capsuleHeight: cap.height })
    // 登录/资料完善守卫：未通过弹回首页，完成后回来继续进台
    if (!guard.ensure(true)) return
    this.setData({
      gameID: Number(options.game_id) || 0,
      inviteToken: options.invite_token || ''
    })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
    wx.showShareMenu({ menus: ['shareAppMessage'] })
    this.loadGame()
  },

  onShow() {
    // 未登录/资料不全时（onLoad 已触发弹回），不再发起轮询
    if (!guard.pass()) return
    if (this.data.gameID && !this.data.loading) {
      this.loadGame()
    }
    this.startPolling()
  },

  onHide() {
    this.stopPolling()
  },

  onUnload() {
    this.stopPolling()
  },

  startPolling() {
    if (this._poll) return
    this._poll = setInterval(() => this.pollGame(), POLL_INTERVAL)
  },

  stopPolling() {
    if (this._poll) {
      clearInterval(this._poll)
      this._poll = null
    }
  },

  pollGame() {
    if (!this.data.gameID) return
    api.get('/games/' + this.data.gameID).then(res => {
      this.applyGame(res, true)
      // 已有弹窗时不打断用户操作
      if (!this.data.showIncomingSwap && !this.data.showSwapModal) this.checkIncomingSwap()
    }).catch(function() {})
  },

  loadGame() {
    this.setData({ loading: true, loadError: false })
    api.get('/games/' + this.data.gameID).then(res => {
      this.applyGame(res, false)
    }).catch(() => {
      this.setData({ loading: false, loadError: true })
    })
  },

  applyGame(res, poll) {
    var players = (res.players || []).map(function(p) {
      return {
        ...p,
        isOwner: p.role === 'owner',
        isSelf: p.user_id === app.globalData.userID,
        avatarColor: util.avatarColor(p.nickname),
        avatar_url: util.resolveAvatarURL(p.avatar_url || '')
      }
    })

    // 构建 2×2 座位：优先按玩家真实 seat 落位；后端未返回 seat 时回退按下标铺排，避免玩家消失
    var seatMap = [null, null, null, null]
    var hasSeat = false
    for (var i = 0; i < players.length; i++) {
      var sp = players[i]
      var sIdx = (sp.seat || 0) - 1
      if (sIdx >= 0 && sIdx < 4) {
        seatMap[sIdx] = sp
        hasSeat = true
      }
    }
    if (!hasSeat) {
      for (var i = 0; i < players.length; i++) seatMap[i] = players[i]
    }
    var seats = []
    for (var i = 0; i < 4; i++) {
      var player = seatMap[i]
      var isOwner = player && player.isOwner
      var isSelf = player && player.isSelf
      var score = player ? (player.total_score || 0) : 0
      seats.push({
        seat: i + 1,
        wind: WINDS[i],
        windClass: WIND_CLASSES[i],
        player: player,
        isOwner: isOwner,
        isSelf: isSelf,
        score: score
      })
    }

    var game = {
      ...res,
      statusText: util.statusText(res.status)
    }

    this.setData({
      game: game,
      players: players,
      seats: seats,
      isOwner: res.creator_id === app.globalData.userID,
      hasScores: (res.completed_rounds || 0) > 0 || this.data.ledger.length > 0,
      roundNeedFix: !!res.round_need_fix,
      roundSumDiff: Math.abs(res.round_sum || 0),
      loading: false,
      loadError: false
    }, () => {
      // 账单对所有雀友实时可见：每次数据刷新都拉一次（台主/雀友一致）
      this.loadLedger()
    })
  },

  // 流水账单：取自转分（adjustment）记录，展示"我转给谁 / 谁转给我"，按局标注
  loadLedger() {
    var myPlayerID = 0
    var infoByPlayer = {}
    var infoByRound = {}
    var players = this.data.players || []
    for (var i = 0; i < players.length; i++) {
      var p = players[i]
      infoByPlayer[p.player_id] = { name: p.nickname, seat: p.seat || 0 }
      if (p.user_id === app.globalData.userID) myPlayerID = p.player_id
    }
    var rounds = (this.data.game && this.data.game.rounds) || []
    for (var j = 0; j < rounds.length; j++) {
      infoByRound[rounds[j].round_id] = rounds[j].round_number
    }

    api.get('/games/' + this.data.gameID + '/adjustments').then(res => {
      var list = (res.adjustments || []).slice()
      // 最新的排最上面（不依赖后端返回顺序）
      list.sort(function(a, b) {
        var ta = new Date(a.created_at || 0).getTime() || a.id || 0
        var tb = new Date(b.created_at || 0).getTime() || b.id || 0
        return tb - ta
      })
      var ledger = list.map(function(a, idx) {
        var from = infoByPlayer[a.from_player_id] || { name: '雀友', seat: 0 }
        var to = infoByPlayer[a.to_player_id] || { name: '雀友', seat: 0 }
        var outgoing = a.from_player_id === myPlayerID
        var incoming = a.to_player_id === myPlayerID
        var mine = outgoing || incoming
        var peerSeat = outgoing ? to.seat : from.seat
        var windIdx = peerSeat > 0 ? (peerSeat - 1) % 4 : 0
        var statusText = a.status === 'accepted' ? '已入账'
          : a.status === 'pending' ? '待确认'
          : a.status === 'rejected' ? '已拒绝' : '已取消'
        var desc = outgoing ? ('转给 ' + to.name)
          : incoming ? (from.name + ' 转给我')
          : (from.name + ' 转给 ' + to.name)
        var roundNum = infoByRound[a.round_id]
        var roundLabel = roundNum ? ('第' + roundNum + '局 · ') : ''
        return {
          id: a.id || idx,
          wind: WINDS[windIdx],
          windClass: WIND_CLASSES[windIdx],
          desc: desc,
          sub: (roundLabel ? roundLabel : '') + (a.reason ? a.reason + ' · ' : '') + util.formatTime(a.created_at),
          score: mine ? (outgoing ? -a.amount : a.amount) : 0,
          amount: a.amount,
          mine: mine,
          statusText: statusText,
          timeText: util.formatTime(a.created_at)
        }
      })
      // 分页：保留用户已展开的条数，避免轮询刷新后被收起
      var shown = this.data.ledgerShown || LEDGER_PAGE_SIZE
      if (shown < LEDGER_PAGE_SIZE) shown = LEDGER_PAGE_SIZE
      if (shown > ledger.length) shown = ledger.length
      this.setData({
        ledger: ledger,
        ledgerShown: shown,
        hasScores: this.data.hasScores || ledger.length > 0
      })
    }).catch(() => {})
  },

  loadMoreLedger() {
    var shown = (this.data.ledgerShown || LEDGER_PAGE_SIZE) + LEDGER_PAGE_SIZE
    if (shown > this.data.ledger.length) shown = this.data.ledger.length
    this.setData({ ledgerShown: shown })
  },

  collapseLedger() {
    this.setData({ ledgerShown: LEDGER_PAGE_SIZE })
  },

  // 手动「开下一局」：局边界由人标记（转分恒为 0 和，无法自动推断）
  startNextRound() {
    wx.showModal({
      title: '开下一局',
      content: '结束当前局并开始新一局？',
      success: (res) => {
        if (!res.confirm) return
        api.post('/games/' + this.data.gameID + '/rounds/manual-next', {}).then(res2 => {
          this.showToast(res2.message || '已开始新一局')
          this.loadGame()
        }).catch(() => {})
      }
    })
  },

  loadQRCode() {
    this.setData({ qrLoading: true })
    wx.request({
      url: app.globalData.baseURL + '/games/' + this.data.gameID + '/qrcode',
      method: 'GET',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      responseType: 'arraybuffer',
      success: (res) => {
        if (res.statusCode === 200) {
          var fs = wx.getFileSystemManager()
          var filePath = wx.env.USER_DATA_PATH + '/qrcode_' + this.data.gameID + '.png'
          try {
            fs.writeFileSync(filePath, res.data, 'binary')
            this.setData({ qrPath: filePath, qrLoading: false })
          } catch (e) {
            var base64 = wx.arrayBufferToBase64(res.data)
            this.setData({ qrPath: 'data:image/png;base64,' + base64, qrLoading: false })
          }
        } else {
          this.setData({ qrLoading: false })
        }
      },
      fail: () => {
        this.setData({ qrLoading: false })
      }
    })
  },

  toggleQrModal() {
    var opening = !this.data.showQrModal
    this.setData({ showQrModal: opening })
    if (opening && !this.data.qrPath && !this.data.qrLoading) {
      this.loadQRCode()
    }
  },

  openScoringModal(e) {
    var seat = e.currentTarget.dataset.seat
    var name = e.currentTarget.dataset.name
    var playerId = e.currentTarget.dataset.playerId
    this.setData({
      showScoreModal: true,
      scoreTargetSeat: seat,
      scoreTargetName: name,
      scoreTargetId: playerId,
      currentScore: 0
    })
  },

  closeScoringModal() {
    this.setData({ showScoreModal: false })
  },

  setScoreValue(e) {
    var val = Number(e.currentTarget.dataset.val)
    this.setData({ currentScore: this.data.currentScore + val })
  },

  adjustScore(e) {
    var delta = Number(e.currentTarget.dataset.delta)
    this.setData({ currentScore: this.data.currentScore + delta })
  },

  submitScore() {
    var amount = this.data.currentScore
    if (amount === 0) {
      wx.showToast({ title: '分数不能为0', icon: 'none' })
      return
    }
    // 转分语义：我出分、对方得分。扣分（对方出分）须由对方在其页面发起，后端不支持反向
    if (amount < 0) {
      wx.showToast({ title: '转记需为正数，扣分请由对方操作', icon: 'none', duration: 2500 })
      return
    }
    api.get('/games/' + this.data.gameID + '/rounds/current').then(round => {
      if (!round || !round.round_id) {
        wx.showToast({ title: '暂无进行中的局', icon: 'none' })
        return
      }
      return api.post('/games/' + this.data.gameID + '/rounds/' + round.round_id + '/adjustments', {
        to_player_id: this.data.scoreTargetId,
        adjustment_type: 'supplement',
        amount: amount,
        auto_accept: true,
        request_id: api.genRequestID()
      }).then(res2 => {
        this.setData({ showScoreModal: false })
        // 台间记分无需对方确认，后端返回"已转记 X 分给 XX"
        this.showToast(res2.message || ('已转记 ' + amount + ' 分给 ' + this.data.scoreTargetName))
        setTimeout(() => this.loadGame(), 500)
      })
    }).catch(() => {
      // 业务错误信息已由 api 层 toast
    })
  },

  onSeatLongPress(e) {
    var seat = Number(e.currentTarget.dataset.seat)
    var seatInfo = this.data.seats.find(function(s) { return s.seat === seat })
    if (!seatInfo) return
    // 空位：立即换过去，无需申请
    if (!seatInfo.player) {
      this.swapToEmptySeat(seat)
      return
    }
    // 自己的座位：无需换位
    if (seatInfo.player.user_id === app.globalData.userID) {
      this.showToast('这是你的座位')
      return
    }
    // 已有玩家的座位：发起换位申请
    this.setData({
      showSwapModal: true,
      swapTargetSeat: seat,
      swapTargetText: '与【' + seat + '位 · ' + seatInfo.player.nickname + '】互换座位'
    })
  },

  // 发起换位申请（对方确认后才互换）
  sendSwapRequest() {
    var seat = this.data.swapTargetSeat
    this.setData({ showSwapModal: false })
    api.post('/games/' + this.data.gameID + '/swap_requests', {
      target_seat: seat
    }).then(res => {
      var id = res.request && res.request.id
      this._mySwapReqId = id || 0
      this._mySwapStatus = id ? 'pending' : ''
      this.showToast(res.message || '换位申请已发送，等待对方确认')
    }).catch(() => {})
  },

  // 轮询：①是否有发给我的换位申请；②我发出的申请被同意/拒绝/超时
  checkIncomingSwap() {
    api.get('/games/' + this.data.gameID + '/swap_requests/pending').then(res => {
      var req = res.request
      if (req && req.id && !(this._swapHandled && this._swapHandled[req.id])) {
        var fromWind = WINDS[(req.from_seat - 1) % 4] || ''
        var toWind = WINDS[(req.to_seat - 1) % 4] || ''
        this.setData({
          showIncomingSwap: true,
          incomingSwap: Object.assign({}, req, {
            fromWind: fromWind,
            toWind: toWind,
            swapTip: fromWind + '位 ⇄ ' + toWind + '位'
          })
        })
      }
      this.checkMySwapResult(res.outgoing)
    }).catch(() => {})
  },

  // 我发出的换位申请的结果回传（对方拒绝 / 同意 / 超时）
  checkMySwapResult(out) {
    if (!this._mySwapReqId || !out || out.id !== this._mySwapReqId) return
    if (out.status === this._mySwapStatus) return
    var status = out.status
    this._mySwapStatus = status
    if (status === 'pending') {
      // 后端不会主动改写为 expired，前端按过期时间判定超时
      var exp = new Date(out.expires_at || 0).getTime()
      if (exp && Date.now() > exp) {
        this._mySwapReqId = 0
        this.showToast('换位申请已超时')
      }
      return
    }
    this._mySwapReqId = 0
    var name = out.to_nickname || '对方'
    if (status === 'accepted') {
      this.showToast('已与' + name + '互换座位')
      this.loadGame()
    } else if (status === 'rejected') {
      this.showToast(name + '拒绝了你的换位申请')
    } else if (status === 'expired') {
      this.showToast('换位申请已超时')
    }
  },

  acceptIncomingSwap() {
    var req = this.data.incomingSwap
    if (!req) return
    this.setData({ showIncomingSwap: false })
    api.post('/games/' + this.data.gameID + '/swap_requests/' + req.id + '/accept', {}).then(res => {
      this._swapHandled = this._swapHandled || {}
      this._swapHandled[req.id] = true
      this.showToast(res.message || '已互换座位')
      this.loadGame()
    }).catch(() => {})
  },

  rejectIncomingSwap() {
    var req = this.data.incomingSwap
    if (!req) return
    this.setData({ showIncomingSwap: false })
    api.post('/games/' + this.data.gameID + '/swap_requests/' + req.id + '/reject', {}).then(res => {
      this._swapHandled = this._swapHandled || {}
      this._swapHandled[req.id] = true
      this.showToast(res.message || '已拒绝换位申请')
    }).catch(() => {})
  },

  swapToEmptySeat(seat) {
    this.playSwapAnim()
    api.post('/games/' + this.data.gameID + '/swap_seat', {
      target_seat: seat,
      request_id: api.genRequestID()
    }).then(() => {
      this.loadGame()
    }).catch(() => {
      this.showToast('换座失败，请稍后再试')
    })
  },

  /** 长按换座时的 Lottie 反馈动画（两个玩家色点沿弧线互换） */
  playSwapAnim() {
    if (!lottie) return
    if (this._swapAnim) {
      this.setData({ showSwapAnim: true })
      this._swapAnim.goToAndPlay(0, true)
      this._scheduleHideSwap()
      return
    }
    this.setData({ showSwapAnim: true })
    var animData = null
    try {
      animData = require('../../assets/animations/seat-swap.js')
    } catch (e) {
      this.setData({ showSwapAnim: false })
      return
    }
    // 等覆盖层渲染出 canvas 后再取 node（避免尺寸为 0）
    setTimeout(() => {
      wx.createSelectorQuery().select('#lottie-swap').fields({ node: true, size: true }).exec((res) => {
        if (!res || !res[0] || !res[0].node) {
          this.setData({ showSwapAnim: false })
          return
        }
        var canvas = res[0].node
        var ctx = canvas.getContext('2d')
        var dpr = wx.getSystemInfoSync().pixelRatio
        canvas.width = res[0].width * dpr
        canvas.height = res[0].height * dpr
        ctx.scale(dpr, dpr)
        try {
          this._swapAnim = lottie.loadAnimation({
            loop: false,
            autoplay: false,
            animationData: animData,
            rendererSettings: { context: ctx, dpr: dpr }
          })
          this._swapAnim.goToAndPlay(0, true)
          this._scheduleHideSwap()
        } catch (err) {
          console.error('swap lottie load failed:', err)
          this.setData({ showSwapAnim: false })
        }
      })
    }, 60)
  },

  _scheduleHideSwap() {
    if (this._swapTimer) clearTimeout(this._swapTimer)
    this._swapTimer = setTimeout(() => {
      this.setData({ showSwapAnim: false })
    }, 1100)
  },

  /** 呼叫雀友：分享房间链接给微信好友 */
  onShareAppMessage() {
    var path = '/pages/room/room?game_id=' + this.data.gameID
    if (this.data.inviteToken) path += '&invite_token=' + this.data.inviteToken
    return {
      title: '速来入座！一起搓一桌',
      path: path
    }
  },

  closeSwapModal() {
    this.setData({ showSwapModal: false })
  },

  goBack() {
    // 分享/扫码直接进入本页时页面栈只有一层，回首页兜底
    var pages = getCurrentPages()
    if (pages.length > 1) {
      wx.navigateBack()
    } else {
      wx.reLaunch({ url: '/pages/index/index' })
    }
  },

  goSettlement() {
    wx.navigateTo({ url: '/pages/settlement/settlement?game_id=' + this.data.gameID })
  },

  doCancel() {
    wx.showModal({
      title: '取消开台',
      content: '确定要取消这个牌台吗？',
      confirmColor: '#B33A3A',
      success: (res) => {
        if (res.confirm) {
          api.post('/games/' + this.data.gameID + '/cancel', {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已取消', icon: 'success' })
            setTimeout(function() { wx.navigateBack() }, 1000)
          })
        }
      }
    })
  },

  doEnd() {
    wx.showModal({
      title: '散台',
      content: '确定要散台吗？结束后进入结算页面。',
      success: (res) => {
        if (res.confirm) {
          api.post('/games/' + this.data.gameID + '/end', {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已散台', icon: 'success' })
            wx.redirectTo({ url: '/pages/settlement/settlement?game_id=' + this.data.gameID })
          })
        }
      }
    })
  },

  showToast(msg) {
    this.setData({ showToast: true, toastMsg: msg })
    if (this._toastTimer) clearTimeout(this._toastTimer)
    this._toastTimer = setTimeout(() => {
      this.setData({ showToast: false })
    }, 2200)
  },

  stopPropagation() {}
})

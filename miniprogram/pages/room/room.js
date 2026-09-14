// pages/room/room.js — 台间页 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')
const tts = require('../../utils/tts')
const prefs = require('../../utils/prefs')

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
// 座位 → 桌面方位：与 WINDS 同序（東 南 西 北）→ 左 上 右 下
// 即 南在上、東在左、西在右、北在下（沿用设计稿的方位，不要按通用罗盘翻成「北在上」）
const SEAT_POS = ['left', 'top', 'right', 'bottom']

// ArrayBuffer → base64：小程序没有全局 btoa，优先用官方能力，缺失时手工分块编码兜底
function arrayBufferToBase64(buffer) {
  if (wx.arrayBufferToBase64) {
    try {
      return wx.arrayBufferToBase64(buffer)
    } catch (e) {}
  }
  var bytes = new Uint8Array(buffer)
  var binary = ''
  var CHUNK = 0x8000
  for (var i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode.apply(null, bytes.subarray(i, i + CHUNK))
  }
  return btoa(binary)
}

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
    qrError: false,
    loading: true,
    loadError: false,
    isOwner: false,
    hasScores: false,
    // 是否出示台码（拉人入台）：牌局还在进行 + 成员没锁 + 没满 4 人，见 applyGame
    canInvite: false,
    showQrModal: false,
    showScoreModal: false,
    ledgerShown: LEDGER_PAGE_SIZE,
    ledgerPageSize: LEDGER_PAGE_SIZE,
    showSwapModal: false,
    swapTargetSeat: 0,
    swapTargetName: '',
    showIncomingSwap: false,
    incomingSwap: null,
    scoreTargetSeat: '',
    scoreTargetName: '',
    scoreTargetWind: '',
    currentScore: 0,
    scoreText: '0',
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
    if (this.data.inviteToken) {
      // 邀请链接进入：先入台再加载。GET /games/:id 仅局内玩家可读，
      // 受邀者未入台直接加载会 403「没有权限执行此操作」（join 幂等，已在局内直接返回成功）
      this.joinThenLoad()
    } else {
      this.loadGame()
    }
  },

  // 邀请链接进入的入台流程
  joinThenLoad() {
    api.post('/games/join', {
      invite_token: this.data.inviteToken,
      request_id: api.genRequestID()
    }, { silent: true }).then(() => {
      this.loadGame()
    }).catch(err => {
      // 房间互斥：已在别的牌台 → 跳去那局
      var inGameID = err && err.game_id
      if (inGameID && Number(inGameID) !== Number(this.data.gameID)) {
        this.setData({ gameID: Number(inGameID) })
        wx.redirectTo({ url: '/pages/room/room?game_id=' + inGameID })
        return
      }
      // 已散台/已满员等原因加入失败：提示具体原因，再尝试加载（非玩家会显示加载失败）
      if (err && err.message) {
        wx.showToast({ title: err.message, icon: 'none' })
      }
      this.loadGame()
    })
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
    // userID 统一转数字比较：本地缓存可能恢复出字符串类型，=== 会永远不相等
    var myID = Number(app.globalData.userID) || 0
    // players 保留**全部**（含已离座）：流水账单要按 player_id 反查昵称头像
    var players = (res.players || []).map(function(p) {
      return {
        ...p,
        status: p.status || 'active',
        isOwner: p.role === 'owner',
        isSelf: Number(p.user_id) === myID,
        avatarColor: util.avatarColor(p.nickname),
        avatar_url: util.resolveAvatarURL(p.avatar_url || '')
      }
    })

    // 我已经不在台上（被台主移出，或自己的退出请求已生效）→ 提示后回首页
    for (var mi = 0; mi < players.length; mi++) {
      if (players[mi].isSelf && players[mi].status !== 'active') {
        this.handleRemoved()
        return
      }
    }

    // 座位只铺「在座」的人；空出来的座位显示为虚位以待
    var seated = players.filter(function(p) { return p.status === 'active' })

    var seatMap = [null, null, null, null]
    var hasSeat = false
    for (var i = 0; i < seated.length; i++) {
      var sp = seated[i]
      var sIdx = (sp.seat || 0) - 1
      if (sIdx >= 0 && sIdx < 4) {
        seatMap[sIdx] = sp
        hasSeat = true
      }
    }
    if (!hasSeat) {
      for (var i = 0; i < seated.length; i++) seatMap[i] = seated[i]
    }
    var seats = []
    for (var i = 0; i < 4; i++) {
      var player = seatMap[i]
      var isOwner = player && player.isOwner
      var isSelf = player && player.isSelf
      var score = player ? (player.total_score || 0) : 0
      seats.push({
        seat: i + 1,
        pos: SEAT_POS[i],
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
      isOwner: Number(res.creator_id) === myID,
      hasScores: (res.completed_rounds || 0) > 0 || this.data.ledger.length > 0,
      // 台码（拉人入台）出示条件，与后端 GetGameQRCode / JoinGame 的守卫保持一致：
      //   牌局还在进行（forming=组桌中 / active=已开局）+ 成员没锁 + 还没满 4 人。
      // 注意**不能用 hasScores**：那是「本局有没有流水账单」。房间页的「给分」走转分接口，
      // 不锁成员（只有 legacy 的首次逐人提交第 1 局才会 LockMembers），
      // 所以给过分之后照样能继续凑脚——拿 hasScores 当判据会让台码提前消失。
      canInvite: (res.status === 'forming' || res.status === 'active') &&
        !res.members_locked &&
        (res.player_count || 0) < 4,
      loading: false,
      loadError: false
    }, () => {
      // 账单对所有雀友实时可见：每次数据刷新都拉一次（台主/雀友一致）
      this.loadLedger()
      // 牌局进行中突然变为结束（非本机操作）→ 5 小时无新账自动结算
      if (this._prevStatus === 'active' && res.status === 'ended') {
        this.showToast('超过 5 小时无新账，牌局已自动结算')
      }
      this._prevStatus = res.status
    })
  },

  // 流水账单：取自转分（adjustment）记录，展示"我转给谁 / 谁转给我"（局概念已移除，按时间自然排列）
  loadLedger() {
    // ID 统一转数字：避免缓存恢复出字符串导致「我」识别失败、头像取错人
    var myPlayerID = 0
    var infoByPlayer = {}
    var players = this.data.players || []
    for (var i = 0; i < players.length; i++) {
      var p = players[i]
      infoByPlayer[Number(p.player_id)] = { name: p.nickname, seat: p.seat || 0, avatar: p.avatar_url || '' }
      if (Number(p.user_id) === Number(app.globalData.userID)) myPlayerID = Number(p.player_id)
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
        var fromId = Number(a.from_player_id)
        var toId = Number(a.to_player_id)
        var from = infoByPlayer[fromId] || { name: '雀友', avatar: '' }
        var to = infoByPlayer[toId] || { name: '雀友', avatar: '' }
        var outgoing = fromId === myPlayerID
        var incoming = toId === myPlayerID
        var mine = outgoing || incoming
        // 头像取「→ 左边的人」（出分方）：如「我 → B」显示我的头像
        var peer = from
        // 文案统一「A → B」，自己显示为「我」
        var fromName = outgoing ? '我' : from.name
        var toName = incoming ? '我' : to.name
        return {
          id: a.id || idx,
          avatar: peer.avatar || '',
          peerInitial: (peer.name || '雀')[0],
          desc: fromName + ' → ' + toName,
          sub: (a.reason ? a.reason + ' · ' : '') + util.formatTime(a.created_at),
          score: mine ? (outgoing ? -a.amount : a.amount) : 0,
          amount: a.amount,
          mine: mine
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

      // 得分提示：只对「收到分」（转入我的新入账）触发，转出去的不提示
      // 首次打开只记录水位不提示（避免进场把整段历史念一遍/震一遍）
      var mineAccepted = list.filter(function(a) {
        if (a.status !== 'accepted') return false
        return Number(a.to_player_id) === myPlayerID
      }).sort(function(a, b) { return (a.id || 0) - (b.id || 0) })
      var maxMineId = mineAccepted.length ? mineAccepted[mineAccepted.length - 1].id : 0
      if (this._lastVoiceAdjId === undefined) {
        this._lastVoiceAdjId = maxMineId
      } else if (this.data.game && this.data.game.status === 'active') {
        var voiceOn = tts.isEnabled() // 开关在「我的」→ 牌局偏好设置，读全局 storage
        for (var vi = 0; vi < mineAccepted.length; vi++) {
          var va = mineAccepted[vi]
          if ((va.id || 0) <= this._lastVoiceAdjId) continue
          this._lastVoiceAdjId = va.id || this._lastVoiceAdjId
          // 震动与语音是两个独立开关：语音关着也照样震
          prefs.vibrateScore()
          if (!voiceOn) continue
          // 统一文案：收到N分
          tts.speak('收到' + va.amount + '分')
        }
      }
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

  // 台码弹窗：打开 / 关闭拆成两个方法。
  // 注意：bindtap 会把「事件对象」当作第一个实参传进来（恒为 truthy），
  // 所以绝不能写成 toggle(opening) { setData({ show: opening }) } ——
  // 那样无论点遮罩还是点 ✕ 都只会把它设成 true，弹窗永远关不掉。
  openQrModal() {
    this.setData({ showQrModal: true })
    if (!this.data.qrPath && !this.data.qrLoading) {
      this.loadQRCode()
    }
  },

  closeQrModal() {
    this.setData({ showQrModal: false })
  },

  // 拉取小程序码（后端返回 image/png 二进制；getBinary 走 arraybuffer，不能复用普通 api.get）
  loadQRCode() {
    if (!this.data.gameID || this.data.qrLoading) return
    var self = this

    // 同局二维码内容固定（scene = game_id），本次启动内已生成过就直接复用，不再重复请求
    this._qrCached = this._qrCached || {}
    if (this._qrCached[this.data.gameID]) {
      this.setData({ qrPath: this._qrCached[this.data.gameID], qrError: false })
      return
    }
    var cachePath = wx.env.USER_DATA_PATH + '/game-qr-' + this.data.gameID + '.png'

    this.setData({ qrLoading: true, qrError: false })
    // 台码要打开哪个版本的小程序：按当前运行环境带上（develop / trial / release）。
    // 不带这个参数微信默认给「正式版」的码 —— 在体验版里扫码会跳到正式版，
    // 那边是另一套 baseURL，联调时会很迷惑。后端会白名单校验。
    var envVersion = (app.globalData && app.globalData.envVersion) || 'release'
    api.getBinary('/games/' + this.data.gameID + '/qrcode?env_version=' + encodeURIComponent(envVersion), {
      errMsg: '台码生成失败'
    }).then(function(buf) {
      if (!buf || !buf.byteLength) throw new Error('empty qrcode')
      // 优先写临时文件：<image src> 可直接读本地路径，且避免 base64 撑大 setData
      var fs = wx.getFileSystemManager()
      try {
        try { fs.unlinkSync(cachePath) } catch (e) {}
        fs.writeFileSync(cachePath, buf)
        self._qrCached[self.data.gameID] = cachePath
        self.setData({ qrPath: cachePath, qrLoading: false, qrError: false })
      } catch (e) {
        // 写入失败时退回 base64 data URI
        self.setData({
          qrPath: 'data:image/png;base64,' + arrayBufferToBase64(buf),
          qrLoading: false,
          qrError: false
        })
      }
    }).catch(function() {
      self.setData({ qrLoading: false, qrError: true })
    })
  },

  // 台码加载失败后的重试入口：先清缓存再重新拉取
  retryQRCode() {
    if (this._qrCached) delete this._qrCached[this.data.gameID]
    this.setData({ qrPath: '', qrError: false })
    this.loadQRCode()
  },

  openScoringModal(e) {
    var seat = e.currentTarget.dataset.seat
    var name = e.currentTarget.dataset.name
    var playerId = e.currentTarget.dataset.playerId
    this.setData({
      showScoreModal: true,
      scoreTargetSeat: seat,
      scoreTargetName: name,
      // 座位 → 风向（座位 1-4 依次为 東南西北，与 applyGame 的 seats 构造一致）
      scoreTargetWind: WINDS[Number(seat) - 1] || '',
      scoreTargetId: playerId,
      currentScore: 0,
      scoreText: '0'
    })
  },

  closeScoringModal() {
    this.setData({ showScoreModal: false })
  },

  // 快捷预设：直接「设为」该分值（不是累加）
  setScoreValue(e) {
    var val = Number(e.currentTarget.dataset.val) || 0
    this.setData({ currentScore: val, scoreText: String(val) })
  },

  adjustScore(e) {
    var delta = Number(e.currentTarget.dataset.delta)
    var next = (this.data.currentScore || 0) + delta
    if (next < 0) next = 0
    this.setData({ currentScore: next, scoreText: String(next) })
  },

  // 直接手输分数：只留数字，空输入按 0 处理
  onScoreInput(e) {
    var raw = String(e.detail.value || '').replace(/[^0-9]/g, '')
    this.setData({ scoreText: raw, currentScore: Number(raw) || 0 })
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

  // 本局是否已产生流水账单。口径与后端 CountAdjustments / 房间页流水列表一致
  // （rejected / cancelled 的转分也算，列表里有多少笔就是多少笔）。
  // 有流水 = 牌局已经开打，人不能单独走（账单会挂在半空），只能「结束散台」统一结算。
  // 注意：只拦「离座」（自己退出 / 台主移出），**换位任何时候都放行**。
  hasLedger() {
    return (this.data.ledger || []).length > 0
  },

  // 长按座位：空位=即时换座（无需申请）/ 自己=退出牌台 / 他人=申请换位
  // 长按他人座位一律走「申请换位」（台主也一样，见 PRD §8.7）；
  // 台主额外能在换位弹窗里把对方「移出牌台」，那才是受流水账单限制的离座动作。
  onSeatLongPress(e) {
    var seat = Number(e.currentTarget.dataset.seat)
    var seatInfo = this.data.seats.find(function(s) { return s.seat === seat })
    if (!seatInfo) return
    // 空位：立即换过去，无需申请
    if (!seatInfo.player) {
      this.swapToEmptySeat(seat)
      return
    }

    // 自己的座位：退出牌台（离座 → 有流水账单就得走「结束散台」结算）
    if (seatInfo.player.isSelf) {
      if (this.hasLedger()) {
        this.showToast('已有流水账单，要用「结束散台」结算')
        return
      }
      this.confirmLeave()
      return
    }

    // 他人座位：任何身份、任何阶段都能申请换位，不受流水账单影响
    this.setData({
      showSwapModal: true,
      swapTargetSeat: seat,
      swapTargetName: seatInfo.player.nickname,
      swapTargetText: '与【' + seat + '位 · ' + seatInfo.player.nickname + '】互换座位'
    })
  },

  // ── 退出牌台（长按自己的座位）──
  confirmLeave() {
    var self = this
    wx.showModal({
      title: '退出牌台',
      content: '确定要退出这张牌台吗？退出后座位会让出来给其他雀友。',
      confirmText: '退出',
      confirmColor: '#c0392b',
      success: function(res) {
        if (res.confirm) self.doLeave()
      }
    })
  },

  doLeave() {
    if (this._leaving) return
    this._leaving = true
    api.post('/games/' + this.data.gameID + '/leave', {}, { silent: true }).then(res => {
      this._leaving = false
      this.stopPolling()
      this.showToast((res && res.message) || '已退出牌台')
      // 不要在页面栈里 back —— 扫码/分享进来的页面栈只有一层
      setTimeout(function() { wx.reLaunch({ url: '/pages/index/index' }) }, 1200)
    }).catch(err => {
      this._leaving = false
      this.showToast((err && err.message) || '退台失败，请重试')
    })
  },

  // ── 移出牌台（台主在换位弹窗里点「移出」）──
  // 换位弹窗对台主多出这个入口：长按他人座位现在一律是「申请换位」，
  // 移出属于离座动作，所以要受「有流水账单必须结束散台结算」限制。
  kickFromSwapModal() {
    if (!this.data.isOwner) return
    var seat = this.data.swapTargetSeat
    var name = this.data.swapTargetName
    if (!seat || !name) return
    this.setData({ showSwapModal: false })
    if (this.hasLedger()) {
      this.showToast('已有流水账单，要用「结束散台」结算')
      return
    }
    this.confirmKick(seat, name)
  },

  // ── 移出牌台确认弹窗 ──
  confirmKick(seat, nickname) {
    var self = this
    wx.showModal({
      title: '移出牌台',
      content: '确定把「' + nickname + '」请出这张牌台吗？座位会腾出来给其他雀友。',
      confirmText: '移出',
      confirmColor: '#c0392b',
      success: function(res) {
        if (res.confirm) self.doKick(seat)
      }
    })
  },

  doKick(seat) {
    if (this._kicking) return
    this._kicking = true
    api.post('/games/' + this.data.gameID + '/kick', { target_seat: seat }, { silent: true }).then(res => {
      this._kicking = false
      this.showToast((res && res.message) || '已移出牌台')
      this.loadGame()
    }).catch(err => {
      this._kicking = false
      this.showToast((err && err.message) || '移出失败，请重试')
    })
  },

  // 轮询发现自己已不在台上（被台主移出）→ 提示并回首页。
  // 只执行一次，避免每次轮询都弹提示。
  handleRemoved() {
    if (this._removedHandled) return
    this._removedHandled = true
    this.stopPolling()
    this.showToast('你已经被移出牌台')
    setTimeout(function() { wx.reLaunch({ url: '/pages/index/index' }) }, 1500)
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
    // 换座是手上真实发生的事，动画起手先给一次轻震（开关在「我的」→ 牌局偏好设置）
    prefs.vibrateAnim()
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
            // 取消后牌台已不存在，直接回首页（分享/扫码直接进本页时页面栈只有一层，navigateBack 会失效）
            setTimeout(function() { wx.reLaunch({ url: '/pages/index/index' }) }, 1000)
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

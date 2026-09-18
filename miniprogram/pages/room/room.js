// pages/room/room.js — 台间页 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')
const tts = require('../../utils/tts')
const wsClient = require('../../utils/ws')
const prefs = require('../../utils/prefs')
const fxsound = require('../../utils/fxsound')
const scoreTags = require('../../utils/score-tags')

// 安全加载 lottie（npm 构建失败时不会阻断页面）
let lottie = null
try {
  lottie = require('lottie-miniprogram')
} catch (e) {
  console.warn('lottie-miniprogram not available, swap animation disabled')
}

const POLL_INTERVAL = 4000
const SLOW_POLL_INTERVAL = 30000 // WS 在线时的兜底刷新间隔（防推送丢失）
const LEDGER_PAGE_SIZE = 7 // 流水账单每页条数（上滑翻页）
const WINDS = ['東', '南', '西', '北']
// 座位 → 桌面方位：与 WINDS 同序（東 南 西 北）→ 左 上 右 下
// 即 南在上、東在左、西在右、北在下（沿用设计稿的方位，不要按通用罗盘翻成「北在上」）
const SEAT_POS = ['left', 'top', 'right', 'bottom']

// 给分标签的「全不选」初始态（打开抽屉 / 关闭 / 提交后都要回到这个状态）
function blankTagOptions() {
  return scoreTags.SCORE_TAGS.map(function(t) { return { code: t.code, label: t.label, on: false } })
}

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
    showFooter: false,
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
    // 已「上膛」的预设分值：点一下填入，再点同一下直接转出（0 = 没上膛）
    presetArmed: 0,
    // 给分标签（多选）：scoreTags 是已选 code（提交用），tagOptions 带 on 标志给 WXML 渲染
    // （WXML 表达式不能调 indexOf，所以选中态必须落到数据里）
    scoreTags: [],
    tagOptions: blankTagOptions(),
    // 给分抽屉：打开即聚焦输入框（省掉「先点输入框」这一步）
    scoreFocus: false,
    // 键盘高度(px)：抽屉整体上移这么多，确认按钮始终露在键盘上方
    kbHeight: 0,
    swapTargetText: '',
    showToast: false,
    toastMsg: '',
    showSwapAnim: false,
    // 席位互动道具（动画编排移植自 docs/design/room-donghua/code.html，只做动画不改桌面样式）
    showPropModal: false,
    propTarget: '',
    propTargetName: '',
    fx: {
      quake: false,   // 全桌地震
      target: '',     // 当前目标方位（top/bottom/left/right）
      hit: false,     // 目标座位命中抖动
      kicked: false,  // 目标座位被踢弹飞
      slipper: null,  // 飞拖鞋 { style }（--sx/--sy/--dx/--dy 注入抛物线）
      stars: null,    // 命中星芒 { style, on }
      kick: null,     // 台下猛踢 { style, cx, cy, footX, footY, run, hit }
      flower: null,   // 花儿谢了 { x, y, on, wither, bubble }
      tea: null,      // 斟杯靓茶 { potX, potY, cupX, cupY, streamX, streamY, streamH, tilt, pour }
      tomato: null,   // 丢番茄（phase: fly/splat；服务端 type 仍为 dimsum）
      sauceTarget: '', // 脸上番茄酱层的目标席位
      tomatoHeavy: false, // 番茄重击头像剧震
      banner: null,   // 踢击私密暗号气泡 { title, desc, on }
      gives: [],      // 给分动画（数组：不同席位对可并行，同对去重）{ key, amount, from, to, fromX, fromY, toX, toY }
      giveHit: {}     // 给分到账标记（按席位 pos 的 map：{top: true}）驱动头像 Q 弹与波纹
    }
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
    // 座位邀请：分享链接带的 seat = 发起邀请的空位；入台后自动坐过去（被占则保持默认分配）
    this._pendingSeat = Number(options.seat) || 0
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
    // 已取消/散台离场中：不再发起任何加载
    if (this._leaving) return
    // 未登录/资料不全时（onLoad 已触发弹回），不再发起轮询
    if (!guard.pass()) return
    if (this.data.gameID && !this.data.loading) {
      this.loadGame()
    }
    this._wsDesired = true
    this.connectRoomWS()
    this.startPolling()
  },

  onHide() {
    this._wsDesired = false
    this.closeRoomWS()
    this.stopPolling()
    // 互动道具动画被打断：清掉未触发的定时器并复位状态，避免回台后状态错乱
    this.clearFxTimers(true)
  },

  onUnload() {
    this._wsDesired = false
    this.closeRoomWS()
    this.stopPolling()
    // 卸载中只清定时器（卸载后 setData 会报错）
    this.clearFxTimers(false)
  },

  // ===== 房间长连接：WS 在线时停用轮询，断线自动回落轮询 + 指数重连 =====
  connectRoomWS() {
    if (this._ws || this._wsDesired !== true) return
    if (!app.globalData.token || !this.data.gameID) return
    var that = this
    this._wsRetry = this._wsRetry || 0
    this._ws = wsClient.createRoomSocket({
      baseURL: app.globalData.baseURL,
      token: app.globalData.token,
      gameID: this.data.gameID,
      onOpen: function() {
        that._wsRetry = 0
        that._wsReady = true
        // 长连接接管：停掉 4s 轮询；保留 30s 兜底刷新（防推送丢失）
        that.stopPolling()
        that.startPolling(SLOW_POLL_INTERVAL)
        that.loadGame() // 打开瞬间全量同步一次，弥补断线窗口
      },
      onClose: function() {
        that._wsReady = false
        that._ws = null
        if (that._wsDesired) {
          that.stopPolling()  // 先停 30s 慢轮询
          that.startPolling() // 回落 4s 轮询
          // 指数退避重连
          var delay = Math.min(15000, 1000 * Math.pow(2, that._wsRetry++))
          setTimeout(function() { that.connectRoomWS() }, delay)
        }
      },
      onMessage: function(msg) { that.handleRoomPush(msg) }
    })
  },

  closeRoomWS() {
    this._wsReady = false
    if (this._ws) {
      this._ws.close()
      this._ws = null
    }
    this._wsRetry = 0
  },

  // 服务端推送分发
  handleRoomPush(msg) {
    if (this._leaving) return
    if (msg.type === 'prop') {
      var d = msg.data || {}
      // 自己发的道具本地已即时播放过（useProp 已把水位推到该 id），广播推回来自身时跳过
      if ((Number(d.id) || 0) <= (this._lastPropId || 0)) return
      this.bumpPropWM(d.id)
      this.enqueueProp(d.type, Number(d.from_player_id), Number(d.to_player_id), Number(d.id) || 0, false)
      return
    }
    if (msg.type === 'game') {
      this.loadGame() // applyGame 会顺带刷新流水/道具轮询
      this.checkIncomingSwapGuarded()
      return
    }
    if (msg.type === 'ledger') {
      this.loadLedger()
      return
    }
    if (msg.type === 'give') {
      // 给分动画全台同步：转分直接生效时后端广播 from/to/amount
      var g = msg.data || {}
      var gid = Number(g.id) || 0
      // 发起人本地已即时播放过（submitScore 已推调整 id 水位），广播推回自身时跳过
      if (gid && gid <= (this._lastGiveId || 0)) return
      if (gid > (this._lastGiveId || 0)) this._lastGiveId = gid
      this.playGiveFx(Number(g.from_player_id), Number(g.to_player_id), Number(g.amount))
      return
    }
    if (msg.type === 'swap') {
      // 换位申请/结果即时推送：秒弹确认框 / 秒看结果
      this.checkIncomingSwapGuarded()
    }
  },

  // 换位弹窗拉取（已有弹窗时不打断用户操作）
  checkIncomingSwapGuarded() {
    if (this.data.showIncomingSwap || this.data.showSwapModal) return
    this.checkIncomingSwap()
  },

  startPolling(interval) {
    if (this._poll) return
    this._poll = setInterval(() => this.pollGame(), interval || POLL_INTERVAL)
  },

  stopPolling() {
    if (this._poll) {
      clearInterval(this._poll)
      this._poll = null
    }
  },

  pollGame() {
    if (!this.data.gameID || this._leaving) return
    api.get('/games/' + this.data.gameID).then(res => {
      this.applyGame(res, true)
      // 已有弹窗时不打断用户操作
      if (!this.data.showIncomingSwap && !this.data.showSwapModal) this.checkIncomingSwap()
    }).catch(err => {
      // 轮询中牌台没了：失去读权限（被移出）或被物理删除（取消开台 / 无流水散台 / 超时清理）
      // —— 统一回首页，不再周期性弹 toast 或空转
      if (err && (err.code === 'FORBIDDEN' || err.code === 'NOT_FOUND')) this.leaveToHome()
    })
  },

  loadGame() {
    if (this._leaving) return
    this.setData({ loading: true, loadError: false })
    api.get('/games/' + this.data.gameID).then(res => {
      this.applyGame(res, false)
    }).catch(err => {
      // 非局内玩家（403）或牌台已被删掉（404，无流水的台取消/散台即物理删除）：
      // 停掉轮询/长连接，跳回首页，不在本页反复弹「没有权限」或停在死掉的房间页
      if (err && (err.code === 'FORBIDDEN' || err.code === 'NOT_FOUND')) {
        this.leaveToHome()
        return
      }
      this.setData({ loading: false, loadError: true })
    })
  },

  // 无权限兜底：停止一切加载行为并回首页
  leaveToHome() {
    if (this._leaving) return
    this._leaving = true
    this._wsDesired = false
    this.stopPolling()
    this.closeRoomWS()
    setTimeout(function() {
      wx.switchTab({ url: '/pages/index/index' })
    }, 600)
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

    var hasScores = (res.completed_rounds || 0) > 0 || this.data.ledger.length > 0

    this.setData({
      game: game,
      players: players,
      seats: seats,
      // 已散台（ended/cancelled/expired）：给分 / 道具 / 换位等写操作统一拦下，
      // 席位卡上的「给分」按钮也随之隐藏（wxml 用 !dissolved 判断）
      dissolved: res.status === 'ended' || res.status === 'cancelled' || res.status === 'expired',
      isOwner: Number(res.creator_id) === myID,
      hasScores: hasScores,
      // 底栏只在还有底栏动作时渲染：取消开台（组桌/开局未记分的台主）或睇翻记录（已散台）；
      // 结束散台已上收到头部台码旁，记分中的台主不再渲染空底栏
      showFooter: (isOwner && (res.status === 'forming' || (res.status === 'active' && !hasScores))) || res.status === 'ended',
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
      // 座位邀请：入台后自动坐到发起邀请的空位（一次性）
      this.maybeTakeInvitedSeat()
    })
  },

  // 座位邀请落座：带 seat 参数进入且当前坐的不是那个位 → 目标位仍空就换过去；被占则保持默认分配
  maybeTakeInvitedSeat() {
    if (!this._pendingSeat || this._seatTried) return
    this._seatTried = true // 只试一次，换座失败不打扰（保持后端分配的座位）
    var seat = this._pendingSeat
    this._pendingSeat = 0
    var seats = this.data.seats || []
    var mine = seats.find(function(s) { return s.player && s.player.isSelf })
    if (!mine || mine.seat === seat) return
    var target = seats.find(function(s) { return s.seat === seat })
    if (!target || target.player) return // 目标位被占：坐哪都一样，不折腾
    this.swapToEmptySeat(seat)
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
          // 给分标签（自摸/明杠/…）：code → 中文，仅有值时才渲染（见 wxml 的 ledger-tag）
          tags: scoreTags.labelsOf(a.tags),
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

      // 道具事件轮询：随主流轮询拉新事件并回放动画（kick 仅双方可见）
      this.loadProps()
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

  // 流水卡片手势：上滑翻下一页，下滑收回到第一页（阈值 50px，纵向位移需大于横向才触发）
  onLedgerTouchStart(e) {
    var t = e.touches && e.touches[0]
    if (!t) return
    this._ledgerSwipe = { x: t.clientX, y: t.clientY }
  },

  onLedgerTouchEnd(e) {
    var s = this._ledgerSwipe
    this._ledgerSwipe = null
    if (!s) return
    var t = (e.changedTouches && e.changedTouches[0]) || {}
    var dy = (t.clientY || 0) - s.y
    var dx = (t.clientX || 0) - s.x
    if (Math.abs(dy) < 50 || Math.abs(dy) <= Math.abs(dx)) return
    if (dy < 0) {
      if (this.data.ledgerShown < this.data.ledger.length) this.loadMoreLedger()
    } else if (this.data.ledgerShown > this.data.ledgerPageSize) {
      this.collapseLedger()
    }
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
    fxsound.warmup() // 首次点击链路预热 WebAudio（规避 iOS 非手势触发限制）
    var that = this
    this.setData({
      showScoreModal: true,
      scoreTargetSeat: seat,
      scoreTargetName: name,
      // 座位 → 风向（座位 1-4 依次为 東南西北，与 applyGame 的 seats 构造一致）
      scoreTargetWind: WINDS[Number(seat) - 1] || '',
      scoreTargetId: playerId,
      currentScore: 0,
      scoreText: '0',
      // 打开即聚焦：直接弹数字键盘，输完点键盘「完成」或点下方确认都能提交，省掉「先点输入框」
      scoreFocus: true,
      presetArmed: 0,
      scoreTags: [],
      tagOptions: blankTagOptions(),
      kbHeight: 0
    })
    // 抽屉渲染后量一次高度，用于限制键盘上移量（小屏不把抽屉顶出屏幕）
    wx.nextTick(function() { that.measureScoreSheet() })
  },

  closeScoringModal() {
    this.setData({
      showScoreModal: false, scoreFocus: false, presetArmed: 0,
      scoreTags: [], tagOptions: blankTagOptions(), kbHeight: 0
    })
    // input 设了 hold-keyboard（点页面不收键盘），抽屉关掉后要显式收，别让键盘悬在那儿
    this.hideScoreKeyboard()
  },

  // hold-keyboard 下键盘不会被点击自动收起，凡是「该收了」的时机都显式调一次
  hideScoreKeyboard() {
    if (wx.hideKeyboard) wx.hideKeyboard({ fail: function() {} })
  },

  // 实测给分抽屉高度（px），打开时量一次；拿不到就退回「不限幅」
  measureScoreSheet() {
    var that = this
    wx.createSelectorQuery().in(this).select('.score-sheet').boundingClientRect(function(rect) {
      if (rect && rect.height) that._scoreSheetH = rect.height
    }).exec()
  },

  // 键盘弹起/收起：抽屉整体上移，保证底部「确认转出 X 分」不被键盘挡住。
  // 输入框设了 adjust-position="{{false}}"（关掉框架自带顶起），位移只由这里给，避免两套叠加。
  // 上移量：屏幕放得下就整张露出来（上移 = 键盘高）；放不下时（小屏 + 抽屉加过标签行后更高）
  // 宁可切掉抽屉顶部那点留白，也要保住按钮——按钮距抽屉底 20~68px，所以最低上移到「键盘高 − 20」。
  onKeyboardHeightChange(e) {
    var kb = (e.detail && e.detail.height) || 0
    var lift = kb
    if (kb > 0) {
      var winH = 0
      try { winH = (wx.getWindowInfo && wx.getWindowInfo().windowHeight) || 0 } catch (err) {}
      var sheetH = this._scoreSheetH || 0
      if (winH && sheetH) {
        var cap = winH - sheetH + 20
        lift = Math.max(0, Math.min(kb, Math.max(cap, kb - 20)))
      }
    }
    if (lift !== this.data.kbHeight) this.setData({ kbHeight: lift })
  },

  // 给分标签：点一下加上，再点一下取消（多选、不分先后顺序，落库按 SCORE_TAGS 的固定序）
  toggleScoreTag(e) {
    var code = e.currentTarget.dataset.code
    if (!code) return
    var picked = {}
    var opts = this.data.tagOptions.map(function(t) {
      var on = t.code === code ? !t.on : t.on
      if (on) picked[t.code] = true
      return { code: t.code, label: t.label, on: on }
    })
    // 提交时的顺序跟展示顺序一致，避免同一组标签因点选先后落出不同的串
    var sel = []
    for (var i = 0; i < opts.length; i++) {
      if (opts[i].on) sel.push(opts[i].code)
    }
    this.setData({ tagOptions: opts, scoreTags: sel })
  },

  // 预设点一下填入分值 + 上膛，再点同一分值直接转出（防误点即转分）；
  // 手输 / ± 步进视为改分，上膛作废（见 onScoreInput / adjustScore）
  setScoreValue(e) {
    var val = Number(e.currentTarget.dataset.val) || 0
    if (!val) return
    if (val === this.data.presetArmed && val === this.data.currentScore) {
      this.submitScore()
      return
    }
    this.setData({ currentScore: val, scoreText: String(val), presetArmed: val })
  },

  adjustScore(e) {
    var delta = Number(e.currentTarget.dataset.delta)
    var next = (this.data.currentScore || 0) + delta
    if (next < 0) next = 0
    this.setData({ currentScore: next, scoreText: String(next), presetArmed: 0 })
  },

  // 点输入框聚焦时：默认 0 分自动清空，直接输入即是新分数（未动过预设/步进时才清）
  onScoreFocus() {
    if (!this.data.currentScore) this.setData({ scoreText: '' })
  },

  // 直接手输分数：只留数字，空输入按 0 处理
  onScoreInput(e) {
    var raw = String(e.detail.value || '').replace(/[^0-9]/g, '')
    var num = Number(raw) || 0
    // 手输即视为改分：预设的「再点一次转出」上膛状态作废。
    // 但值正好等于上膛值时保留——点预设会给 input 回填 value，若某基础库把这次回填也走 bindinput，
    // 上膛状态会在第一次点击后当场被清掉，连点两下就永远不生效。
    var armed = this.data.presetArmed
    this.setData({
      scoreText: raw,
      currentScore: num,
      presetArmed: (armed && num === armed) ? armed : 0
    })
  },

  // 键盘上的「完成」键直接提交：输入 → 点键盘确认 → 完成给分，省去收起键盘再点按钮两步
  onScoreConfirm() {
    this.submitScore()
  },

  submitScore() {
    // 防重复提交：键盘确认与底部确认按钮可能连点
    if (this._scoreSubmitting) return
    // 已散台禁止给分（UI 已隐藏按钮，这里兜底拦截）
    if (!this.ensureActive()) return
    var amount = this.data.currentScore
    if (amount === 0) {
      this.showToast('分数不能为0')
      return
    }
    // 转分语义：我出分、对方得分。扣分（对方出分）须由对方在其页面发起，后端不支持反向
    if (amount < 0) {
      this.showToast('转记需为正数，扣分请由对方操作')
      return
    }
    this._scoreSubmitting = true
    var tags = this.data.scoreTags || []
    var tagLabels = scoreTags.labelsOf(tags)
    api.get('/games/' + this.data.gameID + '/rounds/current', { silent: true }).then(round => {
      if (!round || !round.round_id) {
        this._scoreSubmitting = false
        this.showToast('暂无进行中的局')
        return
      }
      return api.post('/games/' + this.data.gameID + '/rounds/' + round.round_id + '/adjustments', {
        to_player_id: this.data.scoreTargetId,
        adjustment_type: 'supplement',
        amount: amount,
        tags: tags,
        auto_accept: true,
        request_id: api.genRequestID()
      }, { silent: true }).then(res2 => {
        this._scoreSubmitting = false
        this.setData({
          showScoreModal: false, scoreFocus: false, presetArmed: 0,
          scoreTags: [], tagOptions: blankTagOptions(), kbHeight: 0
        })
        this.hideScoreKeyboard() // hold-keyboard 下要显式收键盘
        // 台间记分无需对方确认，后端返回"已转记 X 分给 XX"；选过标签就在后面带上，便于确认没漏选
        var baseMsg = res2.message || ('已转记 ' + amount + ' 分给 ' + this.data.scoreTargetName)
        this.showToast(tagLabels.length ? baseMsg + '（' + tagLabels.join('·') + '）' : baseMsg)
        // 给分动画全台可见：后端会 WS 广播 "give"，各台手机各自播放。
        // 发起人本地即时播（不用等广播回环），并推调整 id 水位去重广播回推。
        var adjId = Number(res2.adjustment && res2.adjustment.id) || 0
        if (adjId > (this._lastGiveId || 0)) this._lastGiveId = adjId
        var myPID = 0
        var seats = this.data.seats || []
        for (var i = 0; i < seats.length; i++) {
          if (seats[i].isSelf && seats[i].player) myPID = seats[i].player.player_id
        }
        this.playGiveFx(myPID, this.data.scoreTargetId, amount)
        setTimeout(() => this.loadGame(), 1100)
      })
    }).catch(err => {
      this._scoreSubmitting = false
      // 统一用自定义顶部 toast，避免与 wx.showToast（居中）叠成两个
      this.showToast((err && err.message) || '转分失败，请重试')
    })
  },

  // 本局是否已产生流水账单。口径与后端 CountAdjustments / 房间页流水列表一致
  // （rejected / cancelled 的转分也算，列表里有多少笔就是多少笔）。
  // 有流水 = 牌局已经开打，人不能单独走（账单会挂在半空），只能「结束散台」统一结算。
  // 注意：只拦「离座」（自己退出 / 台主移出），**换位任何时候都放行**。
  hasLedger() {
    return (this.data.ledger || []).length > 0
  },

  // 已散台的台：给分 / 道具 / 换位申请 / 长按换空位等写操作统一拦下，
  // 只弹一条「散咗台啦！」（此前各路径文案不一，且 api 层的居中 toast 与
  // 自定义顶部 toast 会同时出现两个）。
  ensureActive() {
    if (!this.data.dissolved) return true
    this.showToast('散咗台啦！')
    return false
  },

  // 长按座位：空位=即时换座（无需申请）/ 自己=退出牌台 / 他人=申请换位
  // 长按他人座位一律走「申请换位」（台主也一样，见 PRD §8.7）；
  // 台主额外能在换位弹窗里把对方「移出牌台」，那才是受流水账单限制的离座动作。
  onSeatLongPress(e) {
    // 已散台：换空位 / 申请换位 / 离座 / 移出一律拦截
    if (!this.ensureActive()) return
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
        this.showToast('你点自己做咩呢？')
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
    if (!this.ensureActive()) return // 弹窗开着时牌局可能刚好散台
    var seat = this.data.swapTargetSeat
    this.setData({ showSwapModal: false })
    api.post('/games/' + this.data.gameID + '/swap_requests', {
      target_seat: seat
    }, { silent: true }).then(res => {
      var id = res.request && res.request.id
      this._mySwapReqId = id || 0
      this._mySwapStatus = id ? 'pending' : ''
      this.showToast(res.message || '换位申请已发送，等待对方确认')
    }).catch(err => {
      this.showToast((err && err.message) || '换位申请发送失败，请重试')
    })
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
    }, { silent: true }).then(() => {
      this.loadGame()
    }).catch(err => {
      // api 已静音：错误只走自定义顶部 toast，不会与居中 toast 叠加
      this.showToast((err && err.message) || '换座失败，请稍后再试')
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
    var title = '开台差你一个，快啲入来！'
    // 座位邀请：这次分享从哪个空位发起（用完即清，台码/底部按钮的全桌邀请不带 seat）
    var seat = this._inviteSeat || 0
    this._inviteSeat = 0
    var seatParam = seat > 0 ? '&seat=' + seat : ''
    if (this.data.inviteToken) {
      return {
        title: title,
        path: '/pages/room/room?game_id=' + this.data.gameID + '&invite_token=' + this.data.inviteToken + seatParam
      }
    }
    // 台主重进房间后本地没有 invite_token（只在创建时下发过一次）。
    // 分享路径绝不能缺 token —— 受邀者不是局内玩家，直接开房间页会 403。
    // 兜底走 join 页：后端 JoinGame 支持纯数字 invite_token（= game_id），
    // join 幂等（已在局内直接成功），成功后自动跳回房间页。
    return {
      title: title,
      path: '/pages/join/join?invite_token=' + this.data.gameID + (seat > 0 ? '&seat=' + seat : '')
    }
  },

  // 座位上的「呼叫雀友」：记录从哪个空位发起（bindtap 先于 open-type=share 拉起）
  onSeatInviteTap(e) {
    this._inviteSeat = Number(e.currentTarget.dataset.seat) || 0
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

  // 睇翻记录（牌局已 ended 时的底栏按钮）：直接去记录详情（game-detail），
  // 与翻记录列表点进某场的去向保持一致（原独立「圆满散台」结算页已删除）。
  goRecordDetail() {
    wx.navigateTo({ url: '/pages/game-detail/game-detail?game_id=' + this.data.gameID })
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
            // 牌局已被物理删除：立刻断开 WS/停轮询，否则缓冲期内的
            // "game" 推送和轮询会 loadGame 404，弹出「资源不存在」
            this._leaving = true
            this.closeRoomWS()
            this.stopPolling()
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
      content: '确定要散台吗？结束后进入记录详情。',
      success: (res) => {
        if (res.confirm) {
          api.post('/games/' + this.data.gameID + '/end', {
            request_id: api.genRequestID()
          }).then((res2) => {
            this._leaving = true
            this.closeRoomWS()
            this.stopPolling()
            // 一点流水都没有的台：后端直接删掉房间（没有账可结），记录详情已不存在，
            // 回首页而不是跳一个 404 的详情页
            if (res2 && res2.dissolved) {
              wx.showToast({ title: res2.message || '牌桌已散', icon: 'none' })
              setTimeout(function() { wx.reLaunch({ url: '/pages/index/index' }) }, 900)
              return
            }
            wx.showToast({ title: '已散台', icon: 'success' })
            wx.redirectTo({ url: '/pages/game-detail/game-detail?game_id=' + this.data.gameID })
          })
        }
      }
    })
  },

  // ========== 席位互动道具（编排移植自 docs/design/room-donghua/code.html） ==========
  // 动画层说明：地震/头像弹飞作用在真实节点上（Lottie canvas 无法驱动 DOM），
  // 因此整套编排走 WXSS keyframes；fx 元素坐标由 selectorQuery 实测注入。

  // 点席位头像/昵称打开道具盒：目标就是被点的那个席位，不再做目标选择
  openPropModal(e) {
    if (!this.ensureActive()) return // 已散台不给开道具盒
    fxsound.warmup() // 首次点击链路预热 WebAudio（规避 iOS 非手势触发限制）
    var ds = e.currentTarget.dataset
    var pid = Number(ds.pid) || 0
    if (!ds.pos || !pid) return
    if (pid === this.myPlayerID()) {
      this.showToast('道具要送畀其他雀友')
      return
    }
    this.setData({
      showPropModal: true,
      propTarget: ds.pos,
      propTargetName: ds.name
    })
  },

  closePropModal() {
    this.setData({ showPropModal: false })
  },

  useProp(e) {
    // 已散台禁止用道具
    if (!this.ensureActive()) return
    var type = e.currentTarget.dataset.type
    var pos = this.data.propTarget
    if (!pos) {
      this.showToast('先选一个互动目标')
      return
    }
    var myPID = this.myPlayerID()
    if (!myPID) {
      this.showToast('先上座再使用道具')
      return
    }
    // 目标席位的 player_id
    var seats = this.data.seats || []
    var toPID = 0
    for (var i = 0; i < seats.length; i++) {
      var s = seats[i]
      if (s.pos === pos && s.player) toPID = Number(s.player.player_id)
    }
    if (!toPID) {
      this.showToast('没找到目标席位')
      return
    }
    this.closePropModal()
    var that = this
    // 上报后端 → 同步给同桌；本人立即本地播放（不等轮询）
    // api 静音：错误统一走下方自定义 toast，避免与居中 toast 叠加
    api.post('/games/' + this.data.gameID + '/props', { to_player_id: toPID, type: type }, { silent: true }).then(function(res) {
      that.bumpPropWM(res.id)
      that.enqueueProp(type, myPID, toPID, Number(res.id) || 0, true)
    }).catch(function(err) {
      that.showToast((err && err.message) || '道具发送失败，请重试')
    })
  },

  // 我的 player_id（不在座返回 0）
  myPlayerID() {
    var seats = this.data.seats || []
    for (var i = 0; i < seats.length; i++) {
      if (seats[i].isSelf && seats[i].player) return Number(seats[i].player.player_id)
    }
    return 0
  },

  // 各道具动画总时长（含收尾），用于串行队列解锁
  propDurations: { slipper: 1800, tea: 2900, kick: 4400, flower: 3900, dimsum: 3400 },

  // 并发处理：道具事件入队串行播放。
  // ① 事件 ID 去重 ② 队列内「同人+同目标+同类」合并（短时重复投掷只播一次）
  // ③ 队列上限 4 条，超出丢最旧 ④ kick 可见性在入队前过滤（不占队列）
  // isOwn=自己发的：插队最前（本地即时反馈优先）
  enqueueProp(type, fromPID, toPID, evID, isOwn) {
    if (type === 'kick') {
      var me = this.myPlayerID()
      if (Number(fromPID) !== me && Number(toPID) !== me) return
    }
    if (evID) {
      this._playedPropIds = this._playedPropIds || {}
      if (this._playedPropIds[evID]) return
      this._playedPropIds[evID] = true
      var keys = Object.keys(this._playedPropIds)
      if (keys.length > 20) delete this._playedPropIds[keys[0]] // 只留最近 20 条防膨胀
    }
    var q = this._fxQueue = this._fxQueue || []
    for (var i = 0; i < q.length; i++) {
      if (q[i].type === type && Number(q[i].from) === Number(fromPID) && Number(q[i].to) === Number(toPID)) return
    }
    var item = { type: type, from: Number(fromPID), to: Number(toPID) }
    if (isOwn) q.unshift(item)
    else {
      if (q.length >= 4) q.shift()
      q.push(item)
    }
    this.playNextProp()
  },

  playNextProp() {
    if (this._fxPlaying) return
    var q = this._fxQueue || []
    if (!q.length) return
    var item = q.shift()
    this._fxPlaying = true
    var that = this
    var dur = this.propDurations[item.type] || 3000
    this.playProp(item.type, item.from, item.to)
    this.fxTimeout(function() {
      that._fxPlaying = false
      that.playNextProp()
    }, dur + 150)
  },

  // 回放一次道具动画（仅被队列调用，不做并发假设）
  // 可见性：kick 仅发送者与目标两人看到；其余道具全桌可见
  playProp(type, fromPID, toPID) {
    if (type === 'kick') {
      var me = this.myPlayerID()
      if (Number(fromPID) !== me && Number(toPID) !== me) return
    }
    var that = this
    var seats = this.data.seats || []
    var fromSeat = null
    var toSeat = null
    for (var i = 0; i < seats.length; i++) {
      var s = seats[i]
      if (!s.player) continue
      if (Number(s.player.player_id) === Number(fromPID)) fromSeat = s
      if (Number(s.player.player_id) === Number(toPID)) toSeat = s
    }
    if (!toSeat || !toSeat.player) return
    var meID = this.myPlayerID()
    var targetName = Number(toPID) === meID ? '你' : toSeat.player.nickname
    var fromName = Number(fromPID) === meID ? '我' : (fromSeat && fromSeat.player ? fromSeat.player.nickname : '雀友')

    this.getSeatCenter(fromSeat ? fromSeat.pos : 'east', function(start) {
      if (!start) start = { x: 340, y: 157 }
      that.getSeatCenter(toSeat.pos, function(center) {
        if (!center) return
        var ctx = { start: start, center: center, pos: toSeat.pos, targetName: targetName, fromName: fromName }
        that.setData({ 'fx.target': toSeat.pos })
        if (type === 'slipper') that.fxSlipper(ctx)
        else if (type === 'tea') that.fxTea(ctx)
        else if (type === 'kick') that.fxKick(ctx)
        else if (type === 'flower') that.fxFlower(ctx)
        else if (type === 'dimsum') that.fxTomato(ctx)
      })
    })
  },

  // ===== 道具事件水位：持久化到 storage（按牌局存），跨页面实例/重进房间都不回放历史 =====
  propWMKey() {
    return 'prop_wm_' + this.data.gameID
  },

  bumpPropWM(id) {
    var v = Number(id) || 0
    if (v > (this._lastPropId || 0)) {
      this._lastPropId = v
      try { wx.setStorageSync(this.propWMKey(), v) } catch (e) {}
    }
  },

  loadProps() {
    var that = this
    if (!this.data.gameID || this._leaving || this._propsFetching) return
    this._propsFetching = true
    // 水位来源优先级：本页实例 → storage（跨次进房延续）
    var hasWM = this._lastPropId !== undefined
    if (!hasWM) this._lastPropId = Number(wx.getStorageSync(this.propWMKey())) || 0
    // 该桌从未记录过水位：先向服务器要当前最大事件 id 建水位（绝不回放历史）。
    // ⚠️ 不能用 since_id=0 的列表尾部建水位——后端 LIMIT 50 截断会让水位停在半路，
    //    之后每轮轮询把剩余历史分批当新事件回放（每次几十条动画并发炸屏）
    if (!hasWM && this._lastPropId === 0) {
      api.get('/games/' + this.data.gameID + '/props?latest=1').then(function(r) {
        that._propsFetching = false
        that.bumpPropWM(r.max_id || 0)
      }).catch(function() {
        that._propsFetching = false
      })
      return
    }
    api.get('/games/' + this.data.gameID + '/props?since_id=' + this._lastPropId).then(function(res) {
      that._propsFetching = false
      var evs = res.props || []
      if (!evs.length) return
      for (var i = 0; i < evs.length; i++) {
        var ev = evs[i]
        if ((Number(ev.id) || 0) <= (that._lastPropId || 0)) continue
        that.bumpPropWM(ev.id)
        // 保险：只播 2 分钟内的「活」事件——storage 水位落后时（换设备/清缓存）旧账静默吞掉
        var fresh = ev.created_at && (Date.now() - new Date(ev.created_at).getTime() < 120000)
        if (fresh) that.enqueueProp(ev.type, Number(ev.from_player_id), Number(ev.to_player_id), Number(ev.id) || 0, false)
      }
    }).catch(function() {
      that._propsFetching = false
    })
  },

  // 统一登记 fx 定时器：onHide/onUnload 一次清干净，防状态残留
  fxTimeout(fn, ms) {
    var t = setTimeout(fn, ms)
    this._fxTimers = this._fxTimers || []
    this._fxTimers.push(t)
    return t
  },

  clearFxTimers(reset) {
    if (this._fxTimers) {
      for (var i = 0; i < this._fxTimers.length; i++) clearTimeout(this._fxTimers[i])
      this._fxTimers = []
    }
    // 动画队列一并清空解锁：隐藏/退出后回来不残留半截状态
    this._fxPlaying = false
    this._fxQueue = []
    if (reset) this.setData({ fx: this.initialFx() })
  },

  initialFx() {
    return { quake: false, target: '', hit: false, kicked: false, slipper: null, stars: null, kick: null, flower: null, tea: null, tomato: null, sauceTarget: '', tomatoHeavy: false, banner: null, gives: [], giveHit: {} }
  },

  vibrate(long) {
    try {
      if (long) wx.vibrateLong()
      else wx.vibrateShort({ type: 'medium' })
    } catch (e) {}
  },

  // 席位中心坐标（px，相对 .table-stage 左上角）
  getSeatCenter(pos, cb) {
    var q = wx.createSelectorQuery().in(this)
    q.select('.table-stage').boundingClientRect()
    q.select('.side-' + pos).boundingClientRect()
    q.exec(function(res) {
      var stage = res && res[0]
      var seat = res && res[1]
      if (!stage || !seat) return cb(null)
      cb({ x: seat.left + seat.width / 2 - stage.left, y: seat.top + seat.height / 2 - stage.top })
    })
  },

  // 发射起点：我的席位中心；我不在座则取桌面右侧中部（东位方向）
  mySeatCenter(cb) {
    var seats = this.data.seats || []
    for (var i = 0; i < seats.length; i++) {
      if (seats[i].isSelf && seats[i].player) return this.getSeatCenter(seats[i].pos, cb)
    }
    cb({ x: 340, y: 157 })
  },

  // 1. 扔飞拖鞋 🩴：抛物线飞抵目标 → 命中抖动 + 星芒
  fxSlipper(ctx) {
    var that = this
    var name = ctx.targetName
    var start = ctx.start
    var center = ctx.center
    fxsound.slipperWhoosh() // 出手呼啸
    this.setData({
      'fx.slipper': {
        style: '--sx:' + start.x + 'px;--sy:' + start.y + 'px;--dx:' + center.x + 'px;--dy:' + center.y + 'px;'
      }
    })
    this.fxTimeout(function() {
      fxsound.slipperHit() // 命中啪击脆响（与 CSS 飞行 650ms 同步）
      that.setData({
        'fx.slipper': null,
        'fx.hit': true,
        'fx.stars': { on: true, style: 'left:' + center.x + 'px;top:' + center.y + 'px;' }
      })
      that.vibrate(false)
      that.fxTimeout(function() {
        that.setData({ 'fx.hit': false, 'fx.stars': null })
      }, 900)
    }, 660)
  },

  // 2. 台下猛踢 🦶：大脚破屏 → 300ms 命中 → 三重冲击波 + 全桌地震 + 头像弹飞 + 暗号气泡
  fxKick(ctx) {
    var that = this
    var name = ctx.targetName
    var center = ctx.center
    fxsound.kick() // 低频轰鸣 + 木桌受击
    this.setData({
      'fx.kick': {
        style: 'left:' + center.x + 'px;top:' + center.y + 'px;',
        cx: center.x, cy: center.y,
        footX: center.x - 36, footY: center.y - 45,
        run: true, hit: false
      }
    })
    this.vibrate(false)
    this.fxTimeout(function() {
      that.setData({
        'fx.kicked': true,
        'fx.quake': true,
        'fx.kick.hit': true,
        'fx.banner': {
          on: true,
          title: '大力踢！哎呀！踢咗【' + name + '】一脚！',
          desc: '识做啦，猛咁踢脚喇！'
        }
      })
      that.vibrate(true)
    }, 300)
    this.fxTimeout(function() {
      that.setData({ 'fx.kick': null, 'fx.quake': false })
    }, 1200)
    this.fxTimeout(function() {
      that.setData({ 'fx.banner': null, 'fx.kicked': false })
    }, 1300) // 大力踢提示显示约 1s
  },

  // 3. 花儿谢了 🥀：鲜花送到 → 0.7s 后枯萎凋零 + 愁云雨丝 + 粤语气泡
  fxFlower(ctx) {
    var that = this
    var center = ctx.center
    // 花束统一悬在目标席位上方（花朵本体在锚点下方展开，锚点要比席位中心高约 100px）
    // top 席位（南位）距 fx 层上缘最近：钳制最小 y=6，避免整束被上缘裁掉
    var y = Math.max(6, center.y - 100)
    this.setData({ 'fx.flower': { x: center.x, y: y, on: true, wither: false, bubble: false } })
    this.fxTimeout(function() {
      that.setData({ 'fx.flower.wither': true, 'fx.flower.bubble': true })
      that.vibrate(false)
    }, 700)
    this.fxTimeout(function() {
      that.setData({ 'fx.flower': null })
    }, 3800)
  },

  // 4. 斟杯靓茶 🍵：紫砂壶飞入倾斜 → 茶汤沿壶嘴抛物弧线注入 → 水位涟漪白雾
  fxTea(ctx) {
    var that = this
    var center = ctx.center
    var pos = ctx.pos
    fxsound.teaPour() // 潺潺流水 + 水泡咕嘟（1.4s，铺满斟茶全程）
    var name = ctx.targetName
    var R = 0.5
    try { R = wx.getSystemInfoSync().windowWidth / 750 } catch (e) {}
    // 茶杯悬在席位上方（上方位席位放到席位下方，防止被 fx 层上缘裁掉）
    var cupX = center.x - 44
    var cupY = pos === 'top' ? center.y + 24 : center.y - 110
    // 茶壶悬在杯口右上方；素材壶 210x170rpx，壶嘴尖在 viewBox (10,33)
    var potW = 210 * R
    var potH = 170 * R
    var potX = cupX + 30
    var potY = cupY - 96
    // 壶嘴尖绕壶中心旋转 -42° 后的真实落点（CSS rotate 负角 = 逆时针）
    var pcx = potX + potW / 2
    var pcy = potY + potH / 2
    var ux = potX + (10 / 120) * potW
    var uy = potY + (33 / 100) * potH
    var a = -42 * Math.PI / 180
    var ca = Math.cos(a)
    var sa = Math.sin(a)
    var dx0 = ux - pcx
    var dy0 = uy - pcy
    var spoutX = pcx + dx0 * ca - dy0 * sa
    var spoutY = pcy + dx0 * sa + dy0 * ca
    // 杯口中心；水流素材竖弦 108（壶嘴端 44,6 → 落点 44,114），按弦长缩放并旋转对准杯口
    var rimX = cupX + 44 * R
    var rimY = cupY + 3
    var cdx = spoutX - rimX
    var cdy = rimY - spoutY
    var dist = Math.sqrt(cdx * cdx + cdy * cdy)
    var rot = Math.atan2(cdx, cdy) * 180 / Math.PI
    var s = dist / 108
    this.setData({
      'fx.tea': {
        potX: potX, potY: potY, cupX: cupX, cupY: cupY,
        streamX: spoutX - 44 * s, streamY: spoutY - 6 * s,
        streamW: 60 * s, streamH: 120 * s, rot: rot,
        tilt: false, pour: false
      }
    })
    this.fxTimeout(function() {
      that.setData({ 'fx.tea.tilt': true })
    }, 150)
    this.fxTimeout(function() {
      that.setData({ 'fx.tea.pour': true })
    }, 600)
    this.fxTimeout(function() {
      that.setData({ 'fx.tea.pour': false, 'fx.tea.tilt': false })
    }, 2150)
    this.fxTimeout(function() {
      that.setData({ 'fx.tea': null })
    }, 2800)
  },

  // 5. 丢番茄 🍅（编排移植自 docs/design/fanquedonghua/code.html）：
  // 抛物线飞行 540ms → 冲击闪光 + 6 碎块爆散 + 头像剧震 + 震屏 → 大爆浆糊脸 + 脸上酱层 3.2s → 淡出
  fxTomato(ctx) {
    var that = this
    var pos = ctx.pos
    var fallback = ctx.center
    fxsound.tomatoWhoosh() // 破空抛掷声
    // 精确定位头像中心（席位中心包含昵称/分数，会偏）
    this.getAvatarCenter(pos, function(av) {
      var c = av || fallback
      if (!c) return
      var start = ctx.start
      // 阶段一：飞行（CSS 变量注入起止坐标，keyframes 抛物线 + 680° 旋转）
      that.setData({
        'fx.tomato': { phase: 'fly', style: '--sx:' + start.x + 'px;--sy:' + start.y + 'px;--dx:' + c.x + 'px;--dy:' + c.y + 'px;' },
        'fx.sauceTarget': '',
        'fx.tomatoHeavy': false
      })
      that.vibrate(false)
      // 阶段二：命中——番茄碎裂消失，冲击闪光 + 碎块爆散 + 剧震 + 震屏 + 大爆浆
      that.fxTimeout(function() {
        fxsound.tomatoSplat() // 湿润爆汁破裂声（与 540ms 命中同步）
        that.setData({
          'fx.tomato': { phase: 'splat', x: c.x, y: c.y, pos: pos, fade: false },
          'fx.sauceTarget': pos,
          'fx.tomatoHeavy': true,
          'fx.quake': true
        })
        that.vibrate(true)
      }, 540)
      that.fxTimeout(function() {
        that.setData({ 'fx.quake': false })
      }, 540 + 650)
      that.fxTimeout(function() {
        that.setData({ 'fx.tomatoHeavy': false })
      }, 540 + 1100)
      // 阶段三：整体淡出收尾
      that.fxTimeout(function() {
        that.setData({ 'fx.tomato.fade': true })
      }, 540 + 2700)
      that.fxTimeout(function() {
        that.setData({ 'fx.tomato': null, 'fx.sauceTarget': '' })
      }, 540 + 3300)
    })
  },

  // 7. 给分 🀄：-N 徽章下沉 + 4 枚筹码抛物线飞抵 → 目标头像 Q 弹 + 翡翠波纹 + 浮升 +N
  // （docs/design/geifendonghua：筹码 4 枚错峰 90ms、单枚 650ms，末枚 920ms 到账触发吸收反馈）
  // 全台可见：后端转分生效时 WS 广播 "give"，每台手机各自播放；发起人本地即时播，
  // 广播推回自身时凭调整 id 水位去重（submitScore 已推 _lastGiveId）。
  // 并发：fx.gives 是数组，不同席位对（A→C 与 B→C）并行各播各的；
  // 仅同一对席位重复给分时去重（同航线两批筹码会叠在一起看不清）。
  playGiveFx(fromPlayerId, toPlayerId, amount) {
    var that = this
    fromPlayerId = Number(fromPlayerId)
    toPlayerId = Number(toPlayerId)
    amount = Number(amount) || 0
    if (!fromPlayerId || !toPlayerId || fromPlayerId === toPlayerId || !amount) return
    // 席位定位：筹码从 from 的头像中心飞向 to 的头像中心
    var seats = this.data.seats || []
    var fromPos = ''
    var toPos = ''
    for (var i = 0; i < seats.length; i++) {
      var p = seats[i].player
      if (!p) continue
      if (Number(p.player_id) === fromPlayerId) fromPos = seats[i].pos
      if (Number(p.player_id) === toPlayerId) toPos = seats[i].pos
    }
    if (!fromPos || !toPos) return
    var list = this.data.fx.gives || []
    for (var k = 0; k < list.length; k++) {
      if (list[k].from === fromPos && list[k].to === toPos) return // 同航线在飞：去重
    }
    this.getAvatarCenter(fromPos, function(start) {
      if (!start) return
      that.getAvatarCenter(toPos, function(end) {
        if (!end) return
        // setData 前重读最新列表：两次给分在异步测量窗口内先后进入时，
        // 若仍用进入时的旧数组拼接，后一次会把前一次的动画项顶掉
        var cur = that.data.fx.gives || []
        for (var m = 0; m < cur.length; m++) {
          if (cur[m].from === fromPos && cur[m].to === toPos) return
        }
        var key = 'g' + (++that._giveSeq)
        var item = {
          key: key, amount: amount,
          from: fromPos, to: toPos,
          fromX: start.x, fromY: start.y,
          toX: end.x, toY: end.y
        }
        that.setData({ 'fx.gives': cur.concat([item]) })
        // 筹码发射音随错峰节奏逐枚叮当（音高逐枚升高），与 CSS 动画 delay 对齐
        for (var ci = 0; ci < 4; ci++) {
          (function(idx) {
            that.fxTimeout(function() {
              fxsound.coinClink(idx)
            }, idx * 90)
          })(ci)
        }
        // 末枚筹码到账（270 + 650 = 920ms）：头像 Q 弹 + 翡翠波纹
        that.fxTimeout(function() {
          fxsound.coinArrival() // 金币到账共鸣和弦
          var patch = {}
          patch['fx.giveHit.' + toPos] = true
          that.setData(patch)
          that.vibrate(false)
        }, 920)
        // 徽章 / 波纹播完收尾（+N 徽章 920 + 1750ms）：只摘掉自己这一组
        that.fxTimeout(function() {
          var patch2 = {}
          patch2['fx.giveHit.' + toPos] = false
          that.setData(Object.assign({ 'fx.gives': (that.data.fx.gives || []).filter(function(g) { return g.key !== key }) }, patch2))
        }, 2800)
      })
    })
  },

  // 头像中心坐标（px，相对 .table-stage 左上角）
  getAvatarCenter(pos, cb) {
    var q = wx.createSelectorQuery().in(this)
    q.select('.table-stage').boundingClientRect()
    q.select('.side-' + pos + ' .seat-avatar').boundingClientRect()
    q.exec(function(res) {
      var stage = res && res[0]
      var av = res && res[1]
      if (!stage || !av) return cb(null)
      cb({ x: av.left + av.width / 2 - stage.left, y: av.top + av.height / 2 - stage.top })
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

// pages/room/room.js — 台间页 v6 Stitch 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

const POLL_INTERVAL = 4000
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
    isOwner: false,
    showQrModal: false,
    showQrBanner: true,
    showScoreModal: false,
    showSwapModal: false,
    scoreTargetSeat: '',
    scoreTargetName: '',
    currentScore: 0,
    swapTargetText: '',
    showToast: false,
    toastMsg: ''
  },

  onLoad(options) {
    this.setData({
      gameID: Number(options.game_id) || 0,
      inviteToken: options.invite_token || ''
    })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
    this.loadGame()
  },

  onShow() {
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
    }).catch(function() {})
  },

  loadGame() {
    this.setData({ loading: true })
    api.get('/games/' + this.data.gameID).then(res => {
      this.applyGame(res, false)
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  applyGame(res, poll) {
    var players = (res.players || []).map(function(p) {
      return {
        ...p,
        isOwner: p.role === 'owner',
        avatarColor: util.avatarColor(p.nickname)
      }
    })

    // 构建 2×2 座位
    var seats = []
    for (var i = 0; i < 4; i++) {
      var player = players[i] || null
      var isOwner = player && player.isOwner
      var score = player ? (player.total_score || 0) : 0
      seats.push({
        seat: i + 1,
        wind: WINDS[i],
        windClass: WIND_CLASSES[i],
        player: player,
        isOwner: isOwner,
        score: score
      })
    }

    // 构建流水账单
    var ledger = (res.rounds || []).slice(-5).map(function(r, idx) {
      var windIdx = (r.dealer_seat - 1) % 4 || 0
      return {
        id: r.round_id || idx,
        wind: WINDS[windIdx],
        windClass: WIND_CLASSES[windIdx],
        desc: r.description || ('第' + (idx + 1) + '局'),
        sub: r.winner ? r.winner + ' 赢 ' + r.score + ' 分' : '',
        score: r.score || 0,
        timeText: util.formatTime(r.created_at)
      }
    })

    var game = {
      ...res,
      statusText: util.statusText(res.status)
    }

    var canInvite = (game.status === 'forming' || game.status === 'active') &&
      !game.members_locked && players.length < 4

    this.setData({
      game: game,
      players: players,
      seats: seats,
      ledger: ledger,
      isOwner: res.creator_id === app.globalData.userID,
      loading: false,
      showQrBanner: canInvite
    })

    if (canInvite && !this.data.qrPath && !this.data.qrLoading) {
      this.loadQRCode()
    }
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
    this.setData({ showQrModal: !this.data.showQrModal })
  },

  openScoringModal(e) {
    var seat = e.currentTarget.dataset.seat
    var name = e.currentTarget.dataset.name
    this.setData({
      showScoreModal: true,
      scoreTargetSeat: seat,
      scoreTargetName: name,
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
    if (this.data.currentScore === 0) {
      wx.showToast({ title: '分数不能为0', icon: 'none' })
      return
    }
    this.setData({ showScoreModal: false })
    this.showToast('已记入 ' + this.data.scoreTargetName + ' ' + (this.data.currentScore > 0 ? '+' : '') + this.data.currentScore + ' 分')
    setTimeout(() => this.loadGame(), 500)
  },

  onSeatLongPress(e) {
    var seat = e.currentTarget.dataset.seat
    var name = e.currentTarget.dataset.name
    this.setData({
      showSwapModal: true,
      swapTargetText: '与【' + seat + '位 · ' + name + '】互换座位'
    })
  },

  closeSwapModal() {
    this.setData({ showSwapModal: false })
  },

  sendSwapRequest() {
    this.setData({ showSwapModal: false })
    this.showToast('换位申请已发送，等待对方确认')
  },

  shareInvite() {
    this.showToast('已生成邀请链接')
  },

  goBack() {
    wx.navigateBack()
  },

  goScore() {
    wx.navigateTo({ url: '/pages/score/score?game_id=' + this.data.gameID })
  },

  goSettlement() {
    wx.navigateTo({ url: '/pages/settlement/settlement?game_id=' + this.data.gameID })
  },

  doCancel() {
    wx.showModal({
      title: '取消牌台',
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

  stopPropagation() {},

  onShareAppMessage() {
    return {
      title: '「' + (this.data.game ? this.data.game.name : '得闲开台') + '」等紧你入台！',
      path: '/pages/join/join?invite_token=' + (this.data.inviteToken || this.data.gameID)
    }
  },

  onShareTimeline() {
    return {
      title: '「' + (this.data.game ? this.data.game.name : '得闲开台') + '」等紧你入台！'
    }
  }
})

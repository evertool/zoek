// pages/score/score.js — 记分页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

Page({
  data: {
    gameID: 0,
    round: null,
    game: null,
    myScore: '',
    mySubmitted: false,
    myPlayerID: 0,
    submissions: [],
    submittedCount: 0,
    memberCount: 0,
    scoreSum: 0,
    completedRounds: 0,
    currentRoundNumber: 0,
    roundStatus: '',
    pendingAdjustments: [],
    isOwner: false,
    loading: true,
    submitting: false,
    navPadding: 0
  },

  onLoad(options) {
    // 登录/资料完善守卫：未通过弹回首页，完成后回来继续
    if (!guard.ensure(true)) return
    this.setData({ gameID: Number(options.game_id) || 0, navPadding: util.navPadding() })
    if (!this.data.gameID) {
      wx.showToast({ title: '无效牌局', icon: 'none' })
      return
    }
  },

  onShow() {
    if (!guard.pass()) return
    if (this.data.gameID) {
      this.loadData()
    }
  },

  loadData() {
    this.setData({ loading: true })

    // 并行加载牌局信息和当前局
    Promise.all([
      api.get(`/games/${this.data.gameID}`),
      api.get(`/games/${this.data.gameID}/rounds/current`),
      api.get(`/games/${this.data.gameID}/adjustments`)
    ]).then(([gameRes, roundRes, adjRes]) => {
      const isOwner = gameRes.creator_id === app.globalData.userID
      const myPlayer = (gameRes.players || []).find(p => p.user_id === app.globalData.userID)

      const submissions = (roundRes.submissions || []).map(s => {
        return {
          ...s,
          scoreText: util.formatScore(s.score),
          isMe: s.user_id === app.globalData.userID,
          scoreClass: s.score > 0 ? 'text-positive' : (s.score < 0 ? 'text-negative' : '')
        }
      })

      // 处理 pending 调整
      const pendingAdj = (adjRes.adjustments || []).filter(a => a.status === 'pending')

      // 找到我的提交
      const mySub = submissions.find(s => s.isMe)

      this.setData({
        game: {
          ...gameRes,
          statusText: util.statusText(gameRes.status),
          statusClass: util.statusClass(gameRes.status)
        },
        round: roundRes,
        submissions,
        myScore: mySub ? String(mySub.score) : '',
        mySubmitted: mySub ? mySub.submitted : false,
        myPlayerID: myPlayer ? myPlayer.player_id : 0,
        submittedCount: roundRes.submitted_count || 0,
        memberCount: roundRes.member_count || 0,
        scoreSum: roundRes.score_sum || 0,
        completedRounds: roundRes.completed_rounds || 0,
        currentRoundNumber: roundRes.current_round_number || 0,
        roundStatus: roundRes.status || '',
        pendingAdjustments: pendingAdj,
        isOwner,
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  onScoreInput(e) {
    this.setData({ myScore: e.detail.value })
  },

  doSubmit() {
    if (this.data.submitting) return
    const score = parseInt(this.data.myScore, 10)
    if (isNaN(score)) {
      wx.showToast({ title: '请输入有效分数', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    const roundID = this.data.round.round_id

    api.put(`/games/${this.data.gameID}/rounds/${roundID}/submission`, {
      score,
      request_id: api.genRequestID()
    }).then(res => {
      wx.showToast({ title: res.message || '已入分', icon: 'none' })
      this.setData({ submitting: false })
      this.loadData()
    }).catch(() => {
      this.setData({ submitting: false })
    })
  },

  doLock() {
    if (this.data.scoreSum !== 0) {
      wx.showToast({ title: `今局总和系 ${this.data.scoreSum > 0 ? '+' : ''}${this.data.scoreSum}，请检查同修改`, icon: 'none' })
      return
    }

    wx.showModal({
      title: '确认锁定',
      content: `第${this.data.currentRoundNumber}局核对完成，确认锁定吗？锁定后不能修改。`,
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${this.data.gameID}/rounds/${this.data.round.round_id}/lock`, {
            request_id: api.genRequestID()
          }).then(res => {
            wx.showToast({ title: res.message || '已锁定', icon: 'success' })
            this.loadData()
          })
        }
      }
    })
  },

  doNextRound() {
    api.post(`/games/${this.data.gameID}/rounds`, {
      request_id: api.genRequestID()
    }).then(res => {
      wx.showToast({ title: res.message || '下一局', icon: 'success' })
      this.loadData()
    })
  },

  goAdjustment() {
    wx.navigateTo({
      url: `/pages/adjustment/adjustment?game_id=${this.data.gameID}&round_id=${this.data.round.round_id}`
    })
  },

  goAcceptAdjustment(e) {
    const adjID = e.currentTarget.dataset.id
    api.post(`/games/${this.data.gameID}/adjustments/${adjID}/accept`, {
      request_id: api.genRequestID()
    }).then(res => {
      wx.showToast({ title: res.message || '已确认', icon: 'success' })
      this.loadData()
    })
  },

  goRejectAdjustment(e) {
    const adjID = e.currentTarget.dataset.id
    wx.showModal({
      title: '拒绝调整',
      content: '确定要拒绝呢个改分请求吗？',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${this.data.gameID}/adjustments/${adjID}/reject`, {
            request_id: api.genRequestID()
          }).then(res => {
            wx.showToast({ title: res.message || '已拒绝', icon: 'none' })
            this.loadData()
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
          api.post(`/games/${this.data.gameID}/end`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.redirectTo({
              url: `/pages/settlement/settlement?game_id=${this.data.gameID}`
            })
          })
        }
      }
    })
  }
})

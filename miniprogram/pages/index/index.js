// pages/index/index.js — 首页
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')

const WINDS = ['東', '南', '西', '北']
const WIND_CLASSES = ['east', 'south', 'west', 'north']

// 安全加载 lottie（npm 构建失败时不会阻断页面）
let lottie = null
try {
  lottie = require('lottie-miniprogram')
} catch (e) {
  console.warn('lottie-miniprogram not available, using CSS fallback')
}

Page({
  data: {
    games: [],
    loading: true,
    isLoggedIn: false,
    needProfile: false,
    tempAvatar: '',
    tempNickname: '',
    lottieError: false
  },

  onShow() {
    const isLoggedIn = !!app.globalData.token
    const needProfile = isLoggedIn && app.checkProfileNeeded()

    this.setData({ isLoggedIn, needProfile })

    // 未登录时初始化 Lottie 动画
    if (!isLoggedIn && lottie && !this._lottieLoaded) {
      this._lottieLoaded = true
      setTimeout(() => this.initLottie(), 100)
    }

    if (isLoggedIn && !needProfile) {
      this.loadGames()
    } else {
      this.setData({ loading: false })
    }
  },

  /** 加载 Lottie 麻将牌动画 */
  initLottie() {
    if (!lottie) {
      this.setData({ lottieError: true })
      return
    }
    // lottie-miniprogram 的 path 只支持 http 协议
    // 本地文件需要用 animationData 直接传 JSON 对象
    let animationData = null
    try {
      animationData = require('../../assets/animations/login-tiles.js')
    } catch (e) {
      console.error('Lottie JSON load failed:', e)
      this.setData({ lottieError: true })
      return
    }
    const query = wx.createSelectorQuery()
    query.select('#lottie-login').fields({ node: true, size: true }).exec((res) => {
      if (!res || !res[0] || !res[0].node) {
        this.setData({ lottieError: true })
        return
      }
      const canvas = res[0].node
      const ctx = canvas.getContext('2d')
      const dpr = wx.getSystemInfoSync().pixelRatio
      canvas.width = res[0].width * dpr
      canvas.height = res[0].height * dpr
      ctx.scale(dpr, dpr)
      try {
        lottie.loadAnimation({
          loop: true,
          autoplay: true,
          animationData: animationData,
          rendererSettings: {
            context: ctx,
            dpr: dpr
          }
        })
      } catch (err) {
        console.error('Lottie load failed:', err)
        this.setData({ lottieError: true })
      }
    })
  },

  onPullDownRefresh() {
    if (app.globalData.token && !app.checkProfileNeeded()) {
      this.loadGames().then(() => {
        wx.stopPullDownRefresh()
      })
    } else {
      wx.stopPullDownRefresh()
    }
  },

  loadGames() {
    return api.get('/games/active').then(res => {
    const games = (res.games || []).map(g => {
      const players = (g.players || []).map((p, i) => ({
        ...p,
        wind: p.wind || WINDS[i] || '',
        windClass: p.windClass || WIND_CLASSES[i] || 'east',
        scoreClass: (p.total_score || p.score || 0) >= 0 ? 'positive' : 'negative',
        scoreText: ((p.total_score || p.score || 0) >= 0 ? '+' : '') + (p.total_score || p.score || 0)
      }))
      return {
        ...g,
        players,
        statusText: util.statusText(g.status),
        statusClass: util.statusClass(g.status),
        roundInfo: g.current_round_number
          ? `第${g.current_round_number}局 · 已完成${g.completed_rounds}局`
          : `已完成${g.completed_rounds}局`
      }
    })
      this.setData({ games, loading: false })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  // ===== 登录流程 =====
  doLogin() {
    wx.showLoading({ title: '登录中...' })
    app.login().then(() => {
      wx.hideLoading()
      const needProfile = app.checkProfileNeeded()
      this.setData({
        isLoggedIn: true,
        needProfile
      })
      if (!needProfile) {
        this.loadGames()
      }
    }).catch(() => {
      wx.hideLoading()
    })
  },

  // ===== 头像昵称授权 =====
  onChooseAvatar(e) {
    this.setData({ tempAvatar: e.detail.avatarUrl })
  },

  onNicknameInput(e) {
    this.setData({ tempNickname: e.detail.value })
  },

  doSaveProfile() {
    const nickname = this.data.tempNickname.trim()
    const avatarURL = this.data.tempAvatar

    if (!nickname) {
      wx.showToast({ title: '请输入昵称', icon: 'none' })
      return
    }
    if (!avatarURL) {
      wx.showToast({ title: '请选择头像', icon: 'none' })
      return
    }

    wx.showLoading({ title: '保存中...' })
    // 头像转 base64 持久保存，避免微信临时路径重启后失效
    util.avatarToDataUrl(avatarURL).then(dataUrl => {
      return app.saveProfile(nickname, dataUrl)
    }).then(() => {
      wx.hideLoading()
      wx.showToast({ title: '已完善', icon: 'success' })
      this.setData({ needProfile: false, tempAvatar: '', tempNickname: '' })
      this.loadGames()
    }).catch(() => {
      wx.hideLoading()
    })
  },

  // ===== 列表操作 =====
  // PRD v1.0 §4.2-A: 开台零摩擦——点按钮直接创建牌桌并进入房间，不填台名
  goCreate() {
    if (!app.globalData.token) {
      this.doLogin()
      return
    }
    if (app.checkProfileNeeded()) {
      this.setData({ needProfile: true })
      return
    }
    if (this._creating) return
    this._creating = true
    wx.showLoading({ title: '开台中...' })
    api.post('/games', {
      name: '',
      request_id: api.genRequestID()
    }).then(res => {
      wx.hideLoading()
      this._creating = false
      wx.navigateTo({
        url: `/pages/room/room?game_id=${res.game_id}&invite_token=${res.invite_token}`
      })
    }).catch(() => {
      wx.hideLoading()
      this._creating = false
    })
  },

  goRoom(e) {
    const gameID = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/room/room?game_id=${gameID}` })
  },

  doDelete(e) {
    const gameID = e.currentTarget.dataset.id
    wx.showModal({
      title: '删除房间',
      content: '确定要删除这个房间吗？',
      confirmColor: '#B33A3A',
      success: (res) => {
        if (res.confirm) {
          api.post(`/games/${gameID}/cancel`, {
            request_id: api.genRequestID()
          }).then(() => {
            wx.showToast({ title: '已删除', icon: 'success' })
            this.loadGames()
          })
        }
      }
    })
  }
})

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
    lottieError: false,
    navPadding: 0
  },

  onLoad() {
    // 顶部无导航条，内容需让出状态栏 + 胶囊按钮高度
    this.setData({ navPadding: util.navPadding() })
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
        scoreText: ((p.total_score || p.score || 0) >= 0 ? '+' : '') + (p.total_score || p.score || 0),
        avatar_url: util.resolveAvatarURL(p.avatar_url || '')
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
      // 资料完整：有被守卫拦下的目标页（分享入台/牌台）就回去，否则留在牌局页
      if (!needProfile && !this.goPendingRoute()) {
        this.loadGames()
      }
    }).catch(() => {
      wx.hideLoading()
    })
  },

  /** 登录/完善资料完成后回到进入前的页面；返回 false 表示没有待跳页 */
  goPendingRoute() {
    const target = app.globalData.pendingRoute
    app.globalData.pendingRoute = ''
    if (target) {
      wx.reLaunch({ url: target })
      return true
    }
    return false
  },

  // ===== 头像昵称授权 =====
  onChooseAvatar(e) {
    this.setData({ tempAvatar: e.detail.avatarUrl })
  },

  onNicknameInput(e) {
    this.setData({ tempNickname: e.detail.value })
  },

  doSaveProfile() {
    var nickname = this.data.tempNickname.trim()
    var avatarPath = this.data.tempAvatar

    // 必填校验（按钮已 disabled，此处为安全冗余）
    if (!nickname) {
      wx.showToast({ title: '请输入昵称', icon: 'none' })
      return
    }
    if (!avatarPath) {
      wx.showToast({ title: '请选择头像', icon: 'none' })
      return
    }
    if (this._saving) return
    this._saving = true

    wx.showLoading({ title: '上传头像...' })
    // 上传头像到服务器（内部自动压缩）
    util.uploadAvatar(avatarPath).then(function (relPath) {
      // 拿到相对路径后再调 saveProfile 保存昵称+路径
      return app.saveProfile(nickname, relPath)
    }).then(function (res) {
      wx.hideLoading()
      this._saving = false
      if (res && res.need_profile === false) {
        wx.showToast({ title: '资料已保存', icon: 'success' })
        this.setData({ needProfile: false, tempAvatar: '', tempNickname: '' })
        if (!this.goPendingRoute()) {
          this.loadGames()
        }
      } else {
        wx.showModal({
          title: '保存失败',
          content: '头像或昵称未能通过校验，请重新选择',
          showCancel: false
        })
      }
    }.bind(this)).catch(function (err) {
      wx.hideLoading()
      this._saving = false
      var msg = '保存失败，请重试'
      if (err && err.message === 'FILE_TOO_LARGE') {
        msg = '头像文件超过5MB，请重新选择'
      }
      wx.showToast({ title: msg, icon: 'none' })
    }.bind(this))
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

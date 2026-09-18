// app.js — 得闲开台小程序入口
const api = require('./utils/api')
const util = require('./utils/util')

// ===== 后端地址 =====
// ⚠️ 绝不能再按 envVersion 选环境。微信官方文档对 envVersion=develop 的定义是：
//   「开发版，提交代码审核时默认使用开发版进行审核」
// 即**审核员打开小程序时 envVersion 就是 'develop'**（社区里大量开发者踩过这个坑）。
// 曾经此处 develop → 局域网地址 http://192.168.1.15:8080，于是审核端所有请求必然失败
// （真机强制 https + 域名白名单，且 project.config.json 里 urlCheck:false 只在开发者工具生效，
// 本地完全测不出来）→ 首页整页「网络开小差」→ 审核以「可用性/完整性」被拒。
// 唯一可靠的判据是「跑在开发者工具还是真机」：真机（开发版/审核版、体验版、正式版）一律正式域名。
const PROD_BASE_URL = 'https://zoek.246891.xyz/api/v1'
const DEV_BASE_URL = 'http://192.168.1.15:8080/api/v1'

const envVersion = (() => {
  try {
    return wx.getAccountInfoSync().miniProgram.envVersion || ''
  } catch (e) {
    return ''
  }
})()

// 是否跑在微信开发者工具里（真机上 platform 为 ios / android / mac / windows）
function inDevtools() {
  try {
    return (wx.getSystemInfoSync() || {}).platform === 'devtools'
  } catch (e) {
    return false
  }
}

// baseURL 选择：
//   · 真机（预览/真机调试/体验版/正式版/审核版）→ 永远正式域名。
//     这是铁律，不提供任何开关 —— 审核环境也在这里面，一旦能跑偏就会重演本次拒审。
//   · 开发者工具 → 默认连本地局域网后端（保持原有开发习惯，避免误操作线上数据）；
//     想在工具里连线上：控制台 wx.setStorageSync('use_prod_api', 1) 后重新编译。
function wantLocalAPI() {
  if (!inDevtools()) return false
  try {
    return wx.getStorageSync('use_prod_api') !== 1
  } catch (e) {
    return true // 开关读不到时按本地处理（真机已在上一行返回 false，不影响线上）
  }
}

const baseURL = wantLocalAPI() ? DEV_BASE_URL : PROD_BASE_URL

App({
  globalData: {
    token: '',
    userID: 0,
    nickname: '',
    avatarURL: '',
    baseURL,
    // 当前运行版本：develop（开发者工具 / 审核版）/ trial（体验版）/ release（正式版）。
    // 仅供台码生成决定「扫码后打开哪个版本的小程序」使用。
    // ⚠️ 不要拿它选 baseURL —— 审核环境下这里也是 develop（见文件顶部说明）。
    envVersion,
    needProfile: false,
    // 守卫拦下的目标页（如分享入台/牌台），登录+完善资料后自动回去
    pendingRoute: ''
  },

  onLaunch() {
    const token = wx.getStorageSync('token')
    if (token) {
      this.globalData.token = token
      this.globalData.userID = wx.getStorageSync('user_id') || 0
      this.globalData.nickname = wx.getStorageSync('nickname') || ''
      this.globalData.avatarURL = wx.getStorageSync('avatar_url') || ''
      // 从 storage 恢复后，以本地数据推导 needProfile（服务端返回前兜底）
      this.globalData.needProfile = !this.globalData.nickname || !this.globalData.avatarURL
      // 异步校验 token 有效性并刷新资料
      this._readyPromise = api.get('/user/profile').then(res => {
        this.globalData.nickname = res.nickname || ''
        this.globalData.avatarURL = util.resolveAvatarURL(res.avatar_url || '')
        this.globalData.needProfile = !!res.need_profile
        wx.setStorageSync('nickname', this.globalData.nickname)
        wx.setStorageSync('avatar_url', this.globalData.avatarURL)
      }).catch(err => {
        // 仅 401（token 过期）才登出；网络波动等不登出
        if (err && err.code === 'UNAUTHORIZED') {
          this.logout()
          // token 过期 ≠ 网络问题：wx.login 静默换新 token，用户无感知。
          // 不重登的话，7 天 token 到期后首页必然出现「网络开小差，部分内容未加载」
          // （ready() resolve 时 token 已空，onShow 只能展示失败重试条）。
          return this.login(true).catch(() => {})
        }
      })
    } else {
      // 新用户（无本地 token）：直接静默登录（wx.login 无需用户授权弹窗），
      // 失败不阻塞启动，首页仍可浏览，进入房间时会再次兜底
      this._readyPromise = this.login(true).catch(() => {})
    }
  },

  /** 等待 onLaunch 中的异步 profile 校验完成 */
  ready() {
    return this._readyPromise || Promise.resolve()
  },

  /** 确保已登录，返回 Promise<string token> */
  ensureLogin() {
    if (this.globalData.token) {
      return Promise.resolve(this.globalData.token)
    }
    return this.login()
  },

  /** 微信登录流程（仅获取 code → 后端换 token）
   * @param {boolean} silent — 静默登录（启动时自动调用），失败不弹 toast */
  login(silent) {
    return new Promise((resolve, reject) => {
      wx.login({
        success: (res) => {
          if (!res.code) {
            if (!silent) wx.showToast({ title: '登录失败', icon: 'none' })
            reject(new Error('no code'))
            return
          }
          api.post('/auth/login', {
            code: res.code
          }).then(data => {
            this.globalData.token = data.token
            this.globalData.userID = data.user_id
            this.globalData.nickname = data.nickname
            // 后端返回相对路径，拼接完整 URL
            this.globalData.avatarURL = util.resolveAvatarURL(data.avatar_url || '')
            wx.setStorageSync('token', data.token)
            wx.setStorageSync('user_id', data.user_id)
            wx.setStorageSync('nickname', data.nickname)
            wx.setStorageSync('avatar_url', this.globalData.avatarURL)
            // 以服务端返回为准；后端未返回时按本地资料推导
            this.globalData.needProfile = data.need_profile !== undefined
              ? !!data.need_profile
              : (!this.globalData.nickname || !this.globalData.avatarURL)
            resolve(data.token)
          }).catch(err => {
            if (!silent) wx.showToast({ title: '登录失败', icon: 'none' })
            reject(err)
          })
        },
        fail: () => {
          if (!silent) wx.showToast({ title: '登录失败', icon: 'none' })
          reject(new Error('wx.login failed'))
        }
      })
    })
  },

  /** 保存头像昵称到后端
   * avatarURL 应为服务器返回的相对路径（如 /uploads/avatars/xxx.jpg）
   * 内部自动拼接完整 URL 存储 */
  saveProfile(nickname, avatarURL) {
    return api.put('/user/profile', {
      nickname: nickname,
      avatar_url: avatarURL
    }).then(res => {
      this.globalData.nickname = res.nickname
      // 拼接完整 URL 用于前端显示
      this.globalData.avatarURL = util.resolveAvatarURL(res.avatar_url || '')
      // 以服务端判定为准
      this.globalData.needProfile = !!res.need_profile
      wx.setStorageSync('nickname', res.nickname)
      wx.setStorageSync('avatar_url', this.globalData.avatarURL)
      return res
    })
  },

  /** 检查是否需要授权头像昵称（后端 need_profile 为准，本地推导兜底） */
  checkProfileNeeded() {
    return this.globalData.needProfile ||
      !this.globalData.nickname ||
      !this.globalData.avatarURL
  },

  /** 已登录且资料完整 */
  isReady() {
    return !!this.globalData.token && !this.checkProfileNeeded()
  },

  /** 退出登录 */
  logout() {
    this.globalData.token = ''
    this.globalData.userID = 0
    this.globalData.nickname = ''
    this.globalData.avatarURL = ''
    this.globalData.needProfile = false
    wx.removeStorageSync('token')
    wx.removeStorageSync('user_id')
    wx.removeStorageSync('nickname')
    wx.removeStorageSync('avatar_url')
  }
})

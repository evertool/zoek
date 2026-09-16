// pages/profile/profile.js — 我的页 v7 · docs/design/me 100% 还原
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')
const tts = require('../../utils/tts')
const prefs = require('../../utils/prefs')

// 徽章静态元数据：key 与后端 /user/badges 返回的 code 一一对应（PRD §3.6.6 v1.3 共 7 枚）
// icon —— 图标资源（本地 SVG，展架与弹窗共用同一份，保证「图标 ↔ 徽章」一一对应）
// tint —— 勋章底盘色（点亮时用作光晕色）
const BADGE_META = {
  mahjong_god:     { icon: '/assets/icons/badge-mahjong-god.svg',     tint: '#fffbeb' },
  streak_fire:     { icon: '/assets/icons/badge-streak-fire.svg',     tint: '#fff7ed' },
  big_comeback:    { icon: '/assets/icons/badge-big-comeback.svg',    tint: '#ecfdf5' },
  lucky_king:      { icon: '/assets/icons/badge-lucky-king.svg',      tint: '#fff1f2' },
  stable_mountain: { icon: '/assets/icons/badge-stable-mountain.svg', tint: '#f0f9ff' },
  regular:         { icon: '/assets/icons/badge-regular-player.svg',  tint: '#ecfdf5' },
  iron_leg:        { icon: '/assets/icons/badge-iron-foot.svg',       tint: '#f1f5f9' }
}
const BADGE_FALLBACK_TINT = '#f1f5f9'

// 展架网格最多展示 4 枚（设计稿：4 列灰模 + 最接近的一枚高亮）
const SHOWCASE_COUNT = 4
// 升星提示用序数词（每胜 1 场点亮 1 星）
const STAR_ORDINALS = ['首星', '第二星', '第三星', '第四星', '第五星', '第六星', '第七星', '第八星']

Page({
  data: {
    isLoggedIn: false,
    nickname: '',
    avatarURL: '',
    avatarColor: '',
    userId: '',
    motto: '',
    ageText: '',
    editing: false,
    tempNickname: '',
    tempAvatar: '',
    avatarChanged: false,
    stats: null,
    rankTier: '',
    rankIcon: '',
    star: null, // 升星进度：{ label, hint, pct }
    badges: [],
    badges4: [], // 展架网格（最多 4 枚）
    nextBadge: null,
    badgeTotal: 0,
    badgeLoading: false,
    badgeError: false,
    unlockedCount: 0,
    showBadgePanel: false,
    badgeRules: false,
    bpFocusCode: '',
    bpScrollInto: '',
    vibrateEnabled: true, // 震动提醒（默认开，storage 可关）
    voiceEnabled: false, // 得分语音播报（默认关，storage 可开）
    voiceTone: 'female_yue', // 配音音色（默认女声粤语）
    // 三选一的文案；key 与 utils/prefs.js 的 TONES、后端 handler/tts.go 的 ttsVoices 对齐
    voiceTones: [
      { key: 'female_yue', label: '女声粤语' },
      { key: 'female_mandarin', label: '女声普语' },
      { key: 'male_mandarin', label: '男声普语' }
    ],
    showPrefsPanel: false,
    showSafetyPanel: false,
    // 安全与隐私守则：纯静态文案，点「公平计分与规则公示」展开查看
    safetySections: [
      {
        title: '一、关于本工具',
        items: [
          '「得闲开台」由佛山数匠科技开发，是一款供雀友自行记分、查看战绩的休闲娱乐工具。所有积分只是屏幕上的数字，不具备任何货币价值，亦不可兑换现金、实物或其他利益。'
        ]
      },
      {
        title: '二、我们收集的信息',
        items: [
          '1. 微信登录凭证（OpenID）：用于识别你的账号；',
          '2. 头像与昵称：仅在你主动授权后获取，用于牌台展示与排名；',
          '3. 牌局数据：开台、入台、座位、转分记录、结算结果与时间；',
          '4. 偏好设置：语音播报、音色、震动等，只保存在你本机，不会上传。'
        ]
      },
      {
        title: '三、信息的使用与存储',
        items: [
          '信息只用于实现记分、结算、排名、徽章与历史战绩等产品功能。我们不会向任何第三方出售或共享你的个人信息，不会用于广告投放或用户画像。',
          '牌局数据存储于境内服务器，传输全程使用 HTTPS 加密，访问遵循最小权限原则。'
        ]
      },
      {
        title: '四、信息展示范围',
        items: [
          '头像、昵称、段位与战绩：仅对与你同过台的雀友可见；',
          '雀友号：仅自己可见，可自行复制分享；',
          '牌局明细：仅本台成员可见。',
          '你可以在「雀友黑名单与隐藏」里管理不想同台或不想被看到战绩的雀友。'
        ]
      },
      {
        title: '五、权限说明',
        items: [
          '头像昵称：在你选择微信头像 / 昵称时使用；',
          '音频播放：得分语音播报时使用（默认关闭）；',
          '震动：仅本机轻触反馈（默认开启）。',
          '拒绝授权不会影响记分等核心功能。'
        ]
      },
      {
        title: '六、行为守则',
        items: [
          '1. 本工具纯属休闲娱乐，严禁以任何形式进行赌博、押注或金钱结算；',
          '2. 请勿在牌台内发布违法、暴力、色情、辱骂或其他不良信息；',
          '3. 尊重同台雀友，友善交流，请勿恶意刷分或干扰他人；',
          '4. 发现违规行为，可通过黑名单屏蔽，并向我们举报。'
        ]
      },
      {
        title: '七、未成年人保护',
        items: [
          '本工具面向成年雀友。未成年人请在监护人陪同与同意下使用；我们不会主动向未成年人推送任何内容。'
        ]
      },
      {
        title: '八、你的权利',
        items: [
          '你可以随时在本页修改头像与昵称；',
          '可以退出登录并清除本机登录状态；',
          '可以联系我们申请删除账号及相关数据。删除后，牌局与积分记录将无法恢复。'
        ]
      },
      {
        title: '九、免责声明',
        items: [
          '积分数据由同台雀友自行录入，仅作娱乐参考，不具备任何法律效力，亦不可作为债权债务凭证。',
          '请合理安排娱乐时间，切勿沉迷。得闲饮茶，开心最紧要。'
        ]
      },
      {
        title: '十、联系我们',
        items: [
          '如对本守则有任何疑问、建议或投诉，请通过微信小程序「投诉与反馈」入口，或联系运营方佛山数匠科技。'
        ]
      }
    ],
    showToast: false,
    toastMsg: '',
    navPadding: 0
  },

  onLoad() {
    // 顶部无导航条，内容需让出状态栏 + 胶囊按钮高度
    this.setData({ navPadding: util.navPadding() })
  },

  onShow() {
    // 等待 app onLaunch 异步校验完成
    guard.ensureAsync().then(ok => {
      if (!ok) return
      var isLoggedIn = !!app.globalData.token
      this.setData({
        isLoggedIn: isLoggedIn,
        nickname: app.globalData.nickname || '',
        avatarURL: app.globalData.avatarURL || '',
        avatarColor: util.avatarColor(app.globalData.nickname || ''),
        userId: app.globalData.userID ? ('ZK-' + String(app.globalData.userID).padStart(6, '0')) : '',
        // 个性雀风签名：暂无自定义入口时展示品牌默认文案
        motto: '牌品好，手气自然好 · 得闲多开台',
        // 每次进页面都从 storage 重读，避免「改了但显示的是旧值」
        voiceEnabled: prefs.getVoice(),
        vibrateEnabled: prefs.getVibrate()
      })
      if (isLoggedIn) {
        this.loadStats()
        this.loadBadges()
        this.loadRank()
      }
    })
  },

  // 段位胶囊 + 升星进度条（数据全部来自 /user/profile 的 rank 字段，前端不伪造）
  loadRank() {
    api.get('/user/profile').then(res => {
      var r = res.rank
      if (!r) return
      var star = null
      if (r.is_peak) {
        // 已晋「至尊·最强雀圣」：进度条点满
        star = { label: '至尊·最强雀圣', hint: '已晋雀圣 · 战绩为证', pct: 100 }
      } else if (r.stars_needed === 0) {
        // 至尊段未晋圣：展示晋圣累计进度
        var total = r.total_stars || 0
        star = {
          label: '晋圣进度：' + total + '/50★',
          hint: '再攒 ' + (r.stars_to_peak || 0) + ' 星晋为雀圣',
          pct: Math.min(100, Math.round(total / 50 * 100))
        }
      } else {
        // 常规段：当前段内星 → 下一颗星
        var inTier = r.stars_in_tier || 0
        var next = Math.min(inTier + 1, r.stars_needed)
        var ord = STAR_ORDINALS[next - 1] || ('第' + next + '星')
        var hint
        if (inTier >= r.stars_needed) {
          hint = '再赢 1 场可晋升下一段'
        } else {
          hint = '再赢 1 场可点亮' + ord
        }
        star = {
          label: r.tier_short + ' ' + inTier + '★ → ' + next + '★',
          hint: hint,
          pct: Math.min(100, Math.round(inTier / r.stars_needed * 100))
        }
      }
      this.setData({
        rankTier: r.tier_short,
        rankIcon: util.tierIcon(r.tier_index, r.is_peak),
        tierFull: r.tier_name,
        star: star,
        // 雀龄：由注册时间推算，不足 1 年显示 <1年
        ageText: this.ageFromCreated(res.created_at)
      })
    }).catch(function() {})
  },

  ageFromCreated(created) {
    if (!created) return ''
    var t = new Date(created).getTime()
    if (isNaN(t)) return ''
    var years = Math.floor((Date.now() - t) / 86400000 / 365.25)
    return years >= 1 ? ('雀龄 ' + years + '年') : '雀龄 <1年'
  },

  goRank() {
    wx.navigateTo({ url: '/pages/rank/rank' })
  },

  loadStats() {
    api.get('/user/stats').then(res => {
      var total = res.total_score || 0
      var avg = res.avg_score || 0
      var fmt = function(n) {
        var v = Math.round(n * 100) / 100
        return (v > 0 ? '+' : '') + v
      }
      this.setData({
        stats: {
          games: res.games || 0,
          month_games: res.month_games || 0,
          scoreText: fmt(total),
          avgText: fmt(avg),
          win_rate: res.win_rate || 0,
          recent_wins: res.recent_wins || 0
        }
      })
    }).catch(function() {})
  },

  loadBadges() {
    // 成就徽章：点亮状态一律取自后端 /user/badges（PRD §3.6.6 v1.3 规则 + 真实对局数据），
    // 前端只做「图标 / 底盘 / 排序」映射，绝不自行判定或伪造点亮状态
    this.setData({ badgeLoading: true, badgeError: false })
    api.get('/user/badges', {}, { silent: true }).then(res => {
      const raw = (res && res.badges) || []
      const list = raw.map(b => {
        const meta = BADGE_META[b.code] || {}
        const unlocked = !!b.unlocked
        const target = b.target || 0
        const current = b.current || 0
        const tint = meta.tint || BADGE_FALLBACK_TINT
        return {
          code: b.code,
          name: b.name || '未命名徽章',
          desc: b.desc || '',
          locked: !unlocked,
          iconURL: meta.icon || '',
          icon: '🏅', // 兜底：新增徽章未配图标时才走到
          tint: tint,
          // 已点亮：保留勋章原色底盘；未点亮：由 wxss .pf-badge-ico-dim 置灰
          progress: target > 0
            ? Math.min(100, Math.round(current / target * 100))
            : (unlocked ? 100 : 0),
          current: current,
          target: target
        }
      })
      // 排布：已点亮的勋章优先置顶，未点亮按完成度从高到低
      const lit = list.filter(b => !b.locked)
      const unlit = list.filter(b => b.locked).sort((a, b) => b.progress - a.progress)
      const badges = lit.concat(unlit)
      // 展架空态用：最接近点亮的那枚（只取有计数进度的，布尔型徽章没有中间态）
      const nextBadge = unlit.filter(b => b.target > 1 && b.current > 0)[0] || null
      // 展架网格：最多 4 枚；最接近的一枚带进度数字 + 虚线高亮框
      const focusCode = nextBadge ? nextBadge.code : ''
      const badges4 = badges.slice(0, SHOWCASE_COUNT).map(b => {
        var isNext = b.locked && b.code === focusCode
        return {
          code: b.code,
          iconURL: b.iconURL,
          icon: b.icon,
          tint: b.tint,
          locked: b.locked,
          isNext: isNext,
          label: isNext ? (b.name + ' (' + b.current + '/' + b.target + ')') : b.name
        }
      })
      this.setData({
        badges: badges,
        badges4: badges4,
        nextBadge: nextBadge,
        badgeTotal: (res && res.total) || badges.length,
        unlockedCount: typeof res.unlocked_count === 'number' ? res.unlocked_count : lit.length,
        badgeLoading: false,
        badgeError: false
      })
    }).catch(() => {
      // 读不到战绩时明确报错，而不是静默展示成「一枚都没点亮」
      this.setData({ badgeLoading: false, badgeError: true, badges: [], badges4: [], nextBadge: null, unlockedCount: 0, badgeTotal: 0 })
    })
  },

  /** 打开徽章总览面板（全部徽章排布 + 点亮状态 + 进度 + 规则说明）
   *  从展架网格点入时携带 code：面板自动滚到并高亮该枚徽章 */
  openBadgePanel(e) {
    var code = (e && e.currentTarget && e.currentTarget.dataset) ? (e.currentTarget.dataset.code || '') : ''
    this.setData({
      showBadgePanel: true,
      bpFocusCode: code,
      bpScrollInto: code ? ('bp-' + code) : ''
    })
    if (this._bpFocusTimer) clearTimeout(this._bpFocusTimer)
    if (code) {
      // 高亮只作定位提示，稍后自动淡出；滚动位置保留
      this._bpFocusTimer = setTimeout(() => this.setData({ bpFocusCode: '' }), 1600)
    }
  },

  closeBadgePanel() {
    if (this._bpFocusTimer) clearTimeout(this._bpFocusTimer)
    this.setData({ showBadgePanel: false, badgeRules: false, bpFocusCode: '', bpScrollInto: '' })
  },

  /** 面板内 ⓘ 说明符号：展开/收起规则说明 */
  toggleBadgeRules() {
    this.setData({ badgeRules: !this.data.badgeRules })
  },

  stopPropagation() {},

  doLogin() {
    wx.showLoading({ title: '登录中...' })
    app.login().then(() => {
      wx.hideLoading()
      this.setData({
        isLoggedIn: true,
        nickname: app.globalData.nickname,
        avatarURL: app.globalData.avatarURL,
        avatarColor: util.avatarColor(app.globalData.nickname),
        userId: app.globalData.userID ? ('ZK-' + String(app.globalData.userID).padStart(6, '0')) : ''
      })
      this.loadStats()
      this.loadBadges()
      this.loadRank()
    }).catch(() => {
      wx.hideLoading()
    })
  },

  doLogout() {
    wx.showModal({
      title: '退出登录',
      content: '确定要退出吗？',
      confirmColor: '#B33A3A',
      success: (res) => {
        if (res.confirm) {
          app.logout()
          // 未登录统一回到首页登录页
          wx.reLaunch({ url: '/pages/index/index' })
        }
      }
    })
  },

  goEditProfile() {
    this.setData({
      editing: true,
      tempNickname: this.data.nickname,
      tempAvatar: this.data.avatarURL,
      avatarChanged: false
    })
  },

  onChooseAvatar(e) {
    this.setData({ tempAvatar: e.detail.avatarUrl, avatarChanged: true })
  },

  onNicknameInput(e) {
    this.setData({ tempNickname: e.detail.value })
  },

  saveProfile() {
    var nickname = this.data.tempNickname.trim()
    if (!nickname) {
      wx.showToast({ title: '请输入昵称', icon: 'none' })
      return
    }
    if (this._saving) return
    this._saving = true

    var op
    if (this.data.avatarChanged) {
      // 头像有变更：上传到服务器获取相对路径，再保存
      wx.showLoading({ title: '上传头像...' })
      op = util.uploadAvatar(this.data.tempAvatar).then(function (relPath) {
        wx.showLoading({ title: '保存中...' })
        return app.saveProfile(nickname, relPath)
      })
    } else {
      // 头像未变更：只保存昵称，不传 avatar_url（后端不覆盖）
      wx.showLoading({ title: '保存中...' })
      op = app.saveProfile(nickname, '')
    }
    op.then(function () {
      wx.hideLoading()
      this._saving = false
      wx.showToast({ title: '已保存', icon: 'success' })
      this.setData({
        nickname: app.globalData.nickname,
        avatarURL: app.globalData.avatarURL,
        avatarColor: util.avatarColor(app.globalData.nickname),
        editing: false
      })
    }.bind(this)).catch(function (err) {
      wx.hideLoading()
      this._saving = false
      var msg = '保存失败'
      if (err && err.message === 'FILE_TOO_LARGE') {
        msg = '头像文件超过5MB'
      }
      wx.showToast({ title: msg, icon: 'none' })
    }.bind(this))
  },

  cancelEdit() {
    this.setData({ editing: false })
  },

  copyUserId() {
    wx.setClipboardData({
      data: this.data.userId,
      success: () => {
        this.showToast('雀友号已复制: ' + this.data.userId)
      }
    })
  },

  // ── 牌局偏好设置面板 ──
  openPrefsPanel() {
    this.setData({
      showPrefsPanel: true,
      // 打开时同步一次 storage，多端/多页面改过也能显示对
      voiceEnabled: prefs.getVoice(),
      vibrateEnabled: prefs.getVibrate(),
      voiceTone: prefs.getVoiceTone()
    })
  },

  closePrefsPanel() {
    this.setData({ showPrefsPanel: false })
  },

  // 配音音色：女声粤语（默认）/ 女声普通话 / 男声普通话。
  // 选完立刻用同一句试听——不听见就等于没选。
  // 注意：只有「女声粤语」是真粤语（腾讯云 TextToVoice 的粤语音色只有智彤一个），
  // 另外两个是普通话精品音色；说明文案在面板里写清楚了。
  selectVoiceTone(e) {
    var tone = e.currentTarget.dataset.key
    if (!tone || tone === this.data.voiceTone) return
    this.setData({ voiceTone: tone })
    prefs.setVoiceTone(tone)
    var label = ''
    for (var i = 0; i < this.data.voiceTones.length; i++) {
      if (this.data.voiceTones[i].key === tone) label = this.data.voiceTones[i].label
    }
    this.showToast('配音音色：' + label)
    if (this.data.voiceEnabled) tts.speak('收到10分')
  },

  // 震动提醒（默认开）。开启时立刻震一下当反馈——用户马上知到生效
  toggleVibrate() {
    var next = !this.data.vibrateEnabled
    this.setData({ vibrateEnabled: next })
    prefs.setVibrate(next)
    if (next) prefs.buzz('medium')
    this.showToast(next ? '震动提醒已开启' : '震动提醒已关闭')
  },

  // 得分语音播报（默认关）。开启时用一句粤语试听当反馈
  toggleVoice() {
    var next = !this.data.voiceEnabled
    this.setData({ voiceEnabled: next })
    prefs.setVoice(next)
    this.showToast(next ? '得分语音播报已开启' : '得分语音播报已关闭')
    if (next) tts.speak('得分语音播报已开启，有人转分我会话你知')
  },

  showBlacklist() {
    this.showToast('已载入 0 位屏蔽雀友')
  },

  // ── 公平计分与规则公示（安全与隐私守则）面板 ──
  openSafety() {
    this.setData({ showSafetyPanel: true })
  },

  closeSafety() {
    this.setData({ showSafetyPanel: false })
  },

  showAbout() {
    this.showToast('得闲开台 v1.2.4 (Build 2026)')
  },

  showToast(msg) {
    this.setData({ showToast: true, toastMsg: msg })
    if (this._toastTimer) clearTimeout(this._toastTimer)
    this._toastTimer = setTimeout(() => {
      this.setData({ showToast: false })
    }, 2200)
  }
})

// pages/rank/rank.js — 排位段位页（我的段位 + 升星机制 + 六段雀位）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

// 段位品阶表（六段）：图标为设计资产段位 SVG 图标包（rank-1~6 + rank-peak-sheng），
// 底座色对齐各图标自带底色渐变（iconBg 取渐变起点色）。
const TIER_ROWS = [
  { idx: 1, icon: '/assets/icons/rank-1-que.svg',  iconBg: '#eaf0f3', name: '新手雀仔', req: '星需 3 星 · 新晋开台', grade: '青铜级', range: 'I~III段' },
  { idx: 2, icon: '/assets/icons/rank-2-you.svg',  iconBg: '#e7f5ee', name: '街坊雀友', req: '星需 3 星 · 街坊熟客', grade: '白银级', range: 'I~III段' },
  { idx: 3, icon: '/assets/icons/rank-3-xia.svg',  iconBg: '#fff3d8', name: '叹茶雀侠', req: '星需 4 星 · 叹茶开台', grade: '黄金级', range: 'I~IV段' },
  { idx: 4, icon: '/assets/icons/rank-4-shi.svg',  iconBg: '#fce8e0', name: '老练雀师', req: '星需 4 星 · 牌路纯熟', grade: '铂金级', range: 'I~IV段' },
  { idx: 5, icon: '/assets/icons/rank-5-zong.svg', iconBg: '#f0e8fa', name: '岭南雀宗', req: '星需 6 星 · 手牌自成一派', grade: '皇牌级', range: 'I~V段' },
  { idx: 6, icon: '/assets/icons/rank-6-shen.svg', iconBg: '#fff4c9', name: '无双雀神', req: '累计 50 星晋「至尊·最强雀圣」', grade: '大满贯', range: '全段锁' }
]

// 晋圣后的品阶行文案（至尊行随称号变化）
const PEAK_ROW_NAME = '至尊·最强雀圣'
const PEAK_ROW_REQ = '已晋圣 · 累计星无上限'

Page({
  data: {
    capsuleTop: 0,
    capsuleHeight: 32,
    rank: null,
    starRow: [],
    tierRows: [],
    loading: true,
    navPadding: 0
  },

  onLoad() {
    var cap = util.capsuleBox()
    this.setData({ navPadding: util.navPadding(), capsuleTop: cap.top, capsuleHeight: cap.height })
  },

  onShow() {
    if (!guard.ensure()) return
    this.loadRank()
  },

  loadRank() {
    this.setData({ loading: true })
    api.get('/rank/me').then(res => {
      var tier = res.tier || {}
      var starRow = []
      if (tier.stars_needed > 0) {
        for (var i = 0; i < tier.stars_needed; i++) {
          starRow.push({ filled: i < tier.stars_in_tier })
        }
      }
      this.setData({
        rank: {
          ...res,
          avatar_url: util.resolveAvatarURL(res.avatar_url || ''),
          avatarColor: util.avatarColor(res.nickname || ''),
          winRateText: (res.win_rate || 0) + '%',
          pointsText: (res.points || 0) > 0 ? '+' + res.points : '' + (res.points || 0),
          totalStarsText: tier.stars_needed === 0 ? (res.stars || 0) + '' : ''
        },
        starRow: starRow,
        tierRows: this.buildTierRows(tier),
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
    })
  },

  /** 品阶行：标记「当前所在」，至尊晋圣后该行同步改称号 */
  buildTierRows(tier) {
    var peak = !!tier.is_peak
    return TIER_ROWS.map(function (r) {
      return Object.assign({}, r, {
        is_current: r.idx === tier.tier_index,
        name: (r.idx === 6 && peak) ? (tier.tier_name || PEAK_ROW_NAME) : r.name,
        req: (r.idx === 6 && peak) ? PEAK_ROW_REQ : r.req
      })
    })
  },

  goBack() {
    var pages = getCurrentPages()
    if (pages.length > 1) wx.navigateBack()
    else wx.reLaunch({ url: '/pages/profile/profile' })
  },

  onShareAppMessage() {
    var r = this.data.rank
    return {
      title: '我的排位段位：' + (r ? r.tier.tier_name : '新手雀仔') + (r && r.streak > 1 ? ' · ' + r.streak + '连胜进行中' : ''),
      path: '/pages/rank/rank'
    }
  }
})

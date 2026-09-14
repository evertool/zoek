// pages/rank/rank.js — 排位段位页（我的段位 + 升星机制 + 六品雀位）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

// 六品雀位品阶表：图标取设计资产同名资源（docs/design/icons/rank-*.svg → miniprogram/assets/icons/），
// 底座色对齐设计稿（stone-50 / amber-50、emerald-50、red-50 等）。
const TIER_ROWS = [
  { idx: 1, icon: '/assets/icons/rank-9-novice.svg',    iconBg: '#f8fafc', name: '九品·初入雀境', req: '星需 3 星 · 新晋保底不扣星', grade: '青铜级', range: 'I~III段' },
  { idx: 2, icon: '/assets/icons/rank-8-street.svg',    iconBg: 'rgba(255, 251, 235, 0.75)', name: '八品·市井雀手', req: '星需 3 星 · 零基础连胜稳进', grade: '白银级', range: 'I~III段' },
  { idx: 3, icon: '/assets/icons/rank-6-artisan.svg',   iconBg: 'rgba(245, 245, 244, 0.8)',  name: '六品·茶铺雀侠', req: '星需 4 星 · 橙火高段切磋', grade: '黄金级', range: 'I~IV段' },
  { idx: 4, icon: '/assets/icons/rank-4-facai.svg',     iconBg: '#ecfdf5', name: '四品·省城雀师', req: '星需 4 星 · 需稳踏着齐物徽', grade: '铂金级', range: 'I~IV段' },
  { idx: 5, icon: '/assets/icons/rank-2-hongzhong.svg', iconBg: '#fef2f2', name: '二品·岭南雀宗', req: '星需 6 星 · 扣星加雀考虑心', grade: '皇牌级', range: 'I~V段' },
  { idx: 6, icon: '/assets/icons/rank-1-god-crown.svg', iconBg: '#fffbeb', name: '至尊·无双雀神', req: '累计 50 星晋「至尊·最强雀圣」', grade: '大满贯', range: '全段锁' }
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
      title: '我的排位段位：' + (r ? r.tier.tier_name : '九品·初入雀境') + (r && r.streak > 1 ? ' · ' + r.streak + '连胜进行中' : ''),
      path: '/pages/rank/rank'
    }
  }
})

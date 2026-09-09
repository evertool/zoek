// pages/rank/rank.js — 排位段位页（我的段位 + 升星机制 + 六品雀位）
const app = getApp()
const api = require('../../utils/api')
const util = require('../../utils/util')
const guard = require('../../utils/guard')

Page({
  data: {
    rank: null,
    starRow: [],
    loading: true,
    navPadding: 0
  },

  onLoad() {
    this.setData({ navPadding: util.navPadding() })
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
        loading: false
      })
    }).catch(() => {
      this.setData({ loading: false })
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

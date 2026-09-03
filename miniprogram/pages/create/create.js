// pages/create/create.js — 开台页
const app = getApp()
const api = require('../../utils/api')

Page({
  data: {
    name: '',
    submitting: false
  },

  onInput(e) {
    this.setData({ name: e.detail.value })
  },

  doCreate() {
    if (this.data.submitting) return
    this.setData({ submitting: true })

    const name = this.data.name.trim()
    api.post('/games', {
      name: name || undefined,
      request_id: api.genRequestID()
    }).then(res => {
      // 跳转到台间页
      wx.redirectTo({
        url: `/pages/room/room?game_id=${res.game_id}&invite_token=${res.invite_token}`
      })
    }).catch(() => {
      this.setData({ submitting: false })
    })
  }
})

// components/profile-sheet/profile-sheet.js — 完善头像昵称半屏弹窗
// 仅在用户主动触发「开台/入台」且资料未完善时弹出；点遮罩或「暂不」可随时关闭，
// 绝不自动弹出、不重复骚扰（审核合规）。保存成功后触发 saved 事件继续原动作。
const app = getApp()
const util = require('../../utils/util')

Component({
  properties: {
    show: { type: Boolean, value: false }
  },

  data: {
    tempAvatar: '',
    tempNickname: ''
  },

  methods: {
    onChooseAvatar(e) {
      this.setData({ tempAvatar: e.detail.avatarUrl })
    },

    onNicknameInput(e) {
      var v = e.detail.value || ''
      // 与微信昵称限制一致：总长 ≤32 个字符位（汉字/全角算 2，英文数字算 1，即最多 16 个汉字）
      var out = ''
      var len = 0
      for (var i = 0; i < v.length; i++) {
        var code = v.charCodeAt(i)
        var w = code > 255 ? 2 : 1
        if (len + w > 32) break
        len += w
        out += v[i]
      }
      if (out !== v) {
        wx.showToast({ title: '昵称最多 16 个汉字或 32 个字符', icon: 'none' })
      }
      this.setData({ tempNickname: out })
    },

    onClose() {
      this.triggerEvent('close')
    },

    onMaskTap() {
      this.triggerEvent('close')
    },

    noop() {},

    doSave() {
      var that = this
      var nickname = this.data.tempNickname.trim()
      var avatarPath = this.data.tempAvatar

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
      util.uploadAvatar(avatarPath).then(function (relPath) {
        return app.saveProfile(nickname, relPath)
      }).then(function (res) {
        wx.hideLoading()
        that._saving = false
        if (res && res.need_profile === false) {
          wx.showToast({ title: '资料已保存', icon: 'success' })
          that.setData({ tempAvatar: '', tempNickname: '' })
          that.triggerEvent('saved')
        } else {
          wx.showModal({
            title: '保存失败',
            content: '头像或昵称未能通过校验，请重新选择',
            showCancel: false
          })
        }
      }).catch(function (err) {
        wx.hideLoading()
        that._saving = false
        var msg = '保存失败，请重试'
        if (err && err.message === 'FILE_TOO_LARGE') {
          msg = '头像文件超过5MB，请重新选择'
        }
        wx.showToast({ title: msg, icon: 'none' })
      })
    }
  }
})

// utils/prefs.js — 牌局偏好设置（「我的」→ 牌局偏好设置 面板里的两个开关）
//
// 默认值：
//   得分语音播报 OFF —— 会出声、还走网络请求，默认不打扰
//   震动提醒    ON  —— 只有触感，默认给反馈
//
// 两个开关都存在本机 storage，同步读取（wx.getStorageSync），
// 首页/房间页每次用到时直接问，不做内存缓存，免得「我的」里改了别处不生效。

var VOICE_KEY = 'score_voice_enabled'
var VIBRATE_KEY = 'score_vibrate_enabled'

function readBool(key, def) {
  var v = wx.getStorageSync(key)
  // 未设置过（'' / undefined / null）→ 用默认值；显式存过 false 要认
  if (v === '' || v === undefined || v === null) return def
  return !!v
}

function writeBool(key, on) {
  try {
    wx.setStorageSync(key, !!on)
  } catch (e) {
    // storage 写失败不影响功能，下次读到的还是旧值
  }
}

function getVoice() {
  return readBool(VOICE_KEY, false)
}

function setVoice(on) {
  writeBool(VOICE_KEY, on)
}

function getVibrate() {
  return readBool(VIBRATE_KEY, true)
}

function setVibrate(on) {
  writeBool(VIBRATE_KEY, on)
}

// 震动：开关关掉、或设备不支持时静默跳过，绝不打断业务流程。
// type: 'heavy' | 'medium' | 'light'（默认 medium）
function buzz(type) {
  if (!getVibrate()) return
  if (!wx.vibrateShort) return
  try {
    wx.vibrateShort({
      type: type || 'medium',
      fail: function() {}
    })
  } catch (e) {}
}

// 得分（有人转分给我）——重要事件，给足一点触感
function vibrateScore() {
  buzz('medium')
}

// 动画效果（换座 Lottie 等）——点缀性质，轻一点，别烦人
function vibrateAnim() {
  buzz('light')
}

// ── 配音音色（「我的」→ 牌局偏好设置 → 配音音色，默认女声粤语）──
// 只影响云端通道（后端 /tts 的 voice 参数）；
// 云端不可用回落微信同声传译插件时固定是普通话，音色选不了。
var TONE_KEY = 'tts_voice_tone'

// 与后端 handler/tts.go 的 ttsVoices 键保持一致
var TONES = ['female_yue', 'female_mandarin', 'male_mandarin']
var TONE_DEFAULT = 'female_yue'

function getVoiceTone() {
  var v = wx.getStorageSync(TONE_KEY)
  if (TONES.indexOf(v) !== -1) return v
  return TONE_DEFAULT
}

function setVoiceTone(tone) {
  try {
    wx.setStorageSync(TONE_KEY, TONES.indexOf(tone) !== -1 ? tone : TONE_DEFAULT)
  } catch (e) {
    // storage 写失败不影响功能，下次读到的还是旧值
  }
}

module.exports = {
  getVoice: getVoice,
  setVoice: setVoice,
  getVibrate: getVibrate,
  setVibrate: setVibrate,
  getVoiceTone: getVoiceTone,
  setVoiceTone: setVoiceTone,
  buzz: buzz,
  vibrateScore: vibrateScore,
  vibrateAnim: vibrateAnim
}

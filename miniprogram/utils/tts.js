// utils/tts.js — 得分语音播报
// 主通道：后端 /tts（腾讯云 TextToVoice，粤语音色 101019，需在 backend/config.yaml 配置 tts 密钥）
// 降级：微信同声传译插件（普通话，zh_CN）；两者都不可用则静默
var api = require('./api')
var plugin = null
try {
  plugin = requirePlugin('WechatSI')
} catch (e) {
  plugin = null
}

var audio = null
var queue = []      // 待播报的音频地址（云端临时文件 / 插件返回 URL）
var playing = false
var seq = 0

// 播报开关：storage 未设置时默认开启
function isEnabled() {
  var v = wx.getStorageSync('score_voice_enabled')
  return v === '' || v === undefined || v === null ? true : !!v
}

function setEnabled(on) {
  wx.setStorageSync('score_voice_enabled', !!on)
}

function ensureAudio() {
  if (!audio) {
    audio = wx.createInnerAudioContext()
    audio.onEnded(function() {
      playing = false
      playNext()
    })
    audio.onError(function() {
      playing = false
      playNext()
    })
  }
  return audio
}

function playNext() {
  if (playing || !queue.length) return
  playing = true
  var item = queue.shift()
  var a = ensureAudio()
  a.src = item.src
  a.play()
  // 云端临时文件播完即删，防堆积（插件 URL 无需清理）
  if (item.temp) {
    setTimeout(function() {
      try { wx.getFileSystemManager().unlinkSync(item.src) } catch (e) {}
    }, 5000)
  }
}

// 云端粤语：GET /tts?text= → { audio: base64 } → 写临时文件播放
function cloudSpeak(text, cb) {
  api.get('/tts?text=' + encodeURIComponent(text)).then(function(res) {
    if (!res || !res.audio) return cb(false)
    var fs = wx.getFileSystemManager()
    var path = wx.env.USER_DATA_PATH + '/tts_' + Date.now() + '_' + (seq++) + '.mp3'
    fs.writeFile({
      filePath: path,
      data: res.audio,
      encoding: 'base64',
      success: function() {
        queue.push({ src: path, temp: true })
        playNext()
        cb(true)
      },
      fail: function() { cb(false) }
    })
  }).catch(function() { cb(false) })
}

// 降级：微信同声传译插件（普通话）
function pluginSpeak(text) {
  if (!plugin) return
  plugin.textToSpeech({
    lang: 'zh_CN',
    tts: 1,
    content: String(text),
    success: function(res) {
      if (res && res.filename) {
        queue.push({ src: res.filename, temp: false })
        playNext()
      }
    },
    fail: function() {}
  })
}

// speak(text)：排队播报。失败静默，不打断业务流程
function speak(text) {
  if (!text) return
  if (!isEnabled()) return
  if (queue.length > 3) queue = queue.slice(-2) // 防积压，只留最新
  cloudSpeak(text, function(ok) {
    if (!ok) pluginSpeak(text)
  })
}

module.exports = {
  speak: speak,
  isEnabled: isEnabled,
  setEnabled: setEnabled
}

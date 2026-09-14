// utils/tts.js — 得分语音播报
// 主通道：后端 /tts（腾讯云 TextToVoice；音色按偏好设置三选一：女声粤语（默认）/ 女声普通话 / 男声普通话，
//         需在 backend/config.yaml 配置 tts 密钥）
// 降级：微信同声传译插件（普通话，zh_CN）；两者都不可用则静默
var api = require('./api')
var prefs = require('./prefs')
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

// 播报开关统一由 utils/prefs 管（「我的」→ 牌局偏好设置 → 得分语音播报，默认关）
function isEnabled() {
  return prefs.getVoice()
}

function setEnabled(on) {
  prefs.setVoice(on)
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

// 云端粤语：GET /tts?text=&voice= → { audio: base64 } → 写临时文件播放
function cloudSpeak(text, voice, cb) {
  var path = '/tts?text=' + encodeURIComponent(text)
  if (voice) path += '&voice=' + encodeURIComponent(voice)
  api.get(path).then(function(res) {
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
// 音色取偏好设置（女声粤语默认 / 女声普通话 / 男声普通话），后端按它选 VoiceType
function speak(text) {
  if (!text) return
  if (!isEnabled()) return
  if (queue.length > 3) queue = queue.slice(-2) // 防积压，只留最新
  cloudSpeak(text, prefs.getVoiceTone(), function(ok) {
    if (!ok) pluginSpeak(text)
  })
}

module.exports = {
  speak: speak,
  isEnabled: isEnabled,
  setEnabled: setEnabled
}

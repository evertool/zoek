// utils/fxsound.js — 桌面动画音效合成器
// 1:1 移植 docs/design/donghuashengyin 的 Web Audio 合成方案：振荡器 + 噪声缓冲 + 滤波包络，
// 不依赖任何音频素材文件。小程序没有浏览器 AudioContext，用 wx.createWebAudioContext()
// （基础库 2.19.0+，项目 libVersion 3.5.3 满足）；不支持时静默，绝不影响动画本身。
//
// 发射/命中拆成独立函数（slipper/tomato 设计稿是合成一体的 setTimeout 编排），
// 是因为房间页各动画的飞行时长与设计稿 demo 不同，命中音由 room.js 按自己的节奏触发。

var ctx = null
var enabled = true

function getCtx() {
  if (!enabled) return null
  if (!ctx) {
    try {
      if (wx.createWebAudioContext) ctx = wx.createWebAudioContext()
    } catch (e) {
      ctx = null
    }
  }
  return ctx
}

// 预热：首次在用户点击链路里调用，规避 iOS 对非手势触发的音频限制
function warmup() {
  getCtx()
}

function setEnabled(on) {
  enabled = !!on
}

function isEnabled() {
  return enabled
}

function noiseBuffer(c, seconds) {
  var len = Math.max(1, Math.floor(c.sampleRate * seconds))
  var buf = c.createBuffer(1, len, c.sampleRate)
  var d = buf.getChannelData(0)
  for (var i = 0; i < len; i++) d[i] = Math.random() * 2 - 1
  return buf
}

// 1. 金币飞掠叮当：每枚筹码发射一枚，音高随序号递增（987/1318/2637/3136 Hz × pitch）
function coinClink(index) {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var pitch = 1 + (index || 0) * 0.08
    var freqs = [987 * pitch, 1318 * pitch, 2637 * pitch, 3136 * pitch]
    for (var i = 0; i < freqs.length; i++) {
      var osc = c.createOscillator()
      var gain = c.createGain()
      osc.type = i === 2 ? 'triangle' : 'sine'
      osc.frequency.setValueAtTime(freqs[i], now)
      osc.frequency.exponentialRampToValueAtTime(freqs[i] * 0.985, now + 0.16)
      var base = i === 0 ? 0.28 : (i === 1 ? 0.22 : 0.14)
      gain.gain.setValueAtTime(base, now)
      gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.28)
      osc.connect(gain)
      gain.connect(c.destination)
      osc.start(now)
      osc.stop(now + 0.3)
    }
  } catch (e) {}
}

// 2. 金币到账共鸣和弦：C 大调琶音（1046.5/1318.5/1568/2093 Hz），逐个延迟 20ms
function coinArrival() {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var chord = [1046.5, 1318.5, 1568.0, 2093.0]
    for (var i = 0; i < chord.length; i++) {
      var osc = c.createOscillator()
      var gain = c.createGain()
      osc.type = 'sine'
      osc.frequency.setValueAtTime(chord[i], now + i * 0.02)
      gain.gain.setValueAtTime(0.24, now + i * 0.02)
      gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.65)
      osc.connect(gain)
      gain.connect(c.destination)
      osc.start(now + i * 0.02)
      osc.stop(now + 0.7)
    }
  } catch (e) {}
}

// 3. 台下猛踢：低频主冲击（150→45Hz 三角波）+ 木桌受击低通白噪
function kick() {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var osc = c.createOscillator()
    var oscGain = c.createGain()
    osc.type = 'triangle'
    osc.frequency.setValueAtTime(150, now)
    osc.frequency.exponentialRampToValueAtTime(45, now + 0.35)
    oscGain.gain.setValueAtTime(0.9, now)
    oscGain.gain.exponentialRampToValueAtTime(0.001, now + 0.38)
    osc.connect(oscGain)
    oscGain.connect(c.destination)
    osc.start(now)
    osc.stop(now + 0.4)

    var buf = noiseBuffer(c, 0.25)
    var noise = c.createBufferSource()
    noise.buffer = buf
    var filter = c.createBiquadFilter()
    filter.type = 'lowpass'
    filter.frequency.setValueAtTime(220, now)
    var noiseGain = c.createGain()
    noiseGain.gain.setValueAtTime(0.45, now)
    noiseGain.gain.exponentialRampToValueAtTime(0.001, now + 0.25)
    noise.connect(filter)
    filter.connect(noiseGain)
    noiseGain.connect(c.destination)
    noise.start(now)
    noise.stop(now + 0.26)
  } catch (e) {}
}

// 4a. 飞拖鞋出手：带通白噪呼啸（800→1400Hz）
function slipperWhoosh() {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var whoosh = c.createBufferSource()
    whoosh.buffer = noiseBuffer(c, 0.15)
    var wFilter = c.createBiquadFilter()
    wFilter.type = 'bandpass'
    wFilter.frequency.setValueAtTime(800, now)
    wFilter.frequency.linearRampToValueAtTime(1400, now + 0.12)
    var wGain = c.createGain()
    wGain.gain.setValueAtTime(0.18, now)
    wGain.gain.exponentialRampToValueAtTime(0.001, now + 0.15)
    whoosh.connect(wFilter)
    wFilter.connect(wGain)
    wGain.connect(c.destination)
    whoosh.start(now)
  } catch (e) {}
}

// 4b. 飞拖鞋命中：脆响瞬态（950→260Hz）+ 高通白噪拍击爆裂
function slipperHit() {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var hitOsc = c.createOscillator()
    var hitGain = c.createGain()
    hitOsc.type = 'sine'
    hitOsc.frequency.setValueAtTime(950, now)
    hitOsc.frequency.exponentialRampToValueAtTime(260, now + 0.12)
    hitGain.gain.setValueAtTime(0.85, now)
    hitGain.gain.exponentialRampToValueAtTime(0.001, now + 0.14)
    hitOsc.connect(hitGain)
    hitGain.connect(c.destination)
    hitOsc.start(now)
    hitOsc.stop(now + 0.15)

    var len = Math.floor(c.sampleRate * 0.18)
    var buf = c.createBuffer(1, len, c.sampleRate)
    var d = buf.getChannelData(0)
    for (var i = 0; i < len; i++) d[i] = (Math.random() * 2 - 1) * Math.exp(-i / (c.sampleRate * 0.03))
    var slap = c.createBufferSource()
    slap.buffer = buf
    var slapFilter = c.createBiquadFilter()
    slapFilter.type = 'highpass'
    slapFilter.frequency.setValueAtTime(1200, now)
    var slapGain = c.createGain()
    slapGain.gain.setValueAtTime(0.7, now)
    slapGain.gain.exponentialRampToValueAtTime(0.001, now + 0.16)
    slap.connect(slapFilter)
    slapFilter.connect(slapGain)
    slapGain.connect(c.destination)
    slap.start(now)
  } catch (e) {}
}

// 5a. 扔番茄出手：破空上扬（320→680Hz 正弦）
function tomatoWhoosh() {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var whooshOsc = c.createOscillator()
    var whooshGain = c.createGain()
    whooshOsc.type = 'sine'
    whooshOsc.frequency.setValueAtTime(320, now)
    whooshOsc.frequency.linearRampToValueAtTime(680, now + 0.3)
    whooshGain.gain.setValueAtTime(0.16, now)
    whooshGain.gain.exponentialRampToValueAtTime(0.001, now + 0.32)
    whooshOsc.connect(whooshGain)
    whooshGain.connect(c.destination)
    whooshOsc.start(now)
    whooshOsc.stop(now + 0.33)
  } catch (e) {}
}

// 5b. 扔番茄爆汁：低音爆破（240→70Hz）+ 带通湿润爆溅白噪
function tomatoSplat() {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var popOsc = c.createOscillator()
    var popGain = c.createGain()
    popOsc.type = 'triangle'
    popOsc.frequency.setValueAtTime(240, now)
    popOsc.frequency.exponentialRampToValueAtTime(70, now + 0.22)
    popGain.gain.setValueAtTime(0.65, now)
    popGain.gain.exponentialRampToValueAtTime(0.001, now + 0.24)
    popOsc.connect(popGain)
    popGain.connect(c.destination)
    popOsc.start(now)
    popOsc.stop(now + 0.25)

    var splatLen = Math.floor(c.sampleRate * 0.35)
    var splatBuf = c.createBuffer(1, splatLen, c.sampleRate)
    var data = splatBuf.getChannelData(0)
    for (var i = 0; i < splatLen; i++) {
      data[i] = (Math.random() * 2 - 1) * Math.exp(-i / (c.sampleRate * 0.08))
    }
    var noise = c.createBufferSource()
    noise.buffer = splatBuf
    var filter = c.createBiquadFilter()
    filter.type = 'bandpass'
    filter.frequency.setValueAtTime(900, now)
    filter.Q.setValueAtTime(2.5, now)
    var sGain = c.createGain()
    sGain.gain.setValueAtTime(0.75, now)
    sGain.gain.exponentialRampToValueAtTime(0.001, now + 0.34)
    noise.connect(filter)
    filter.connect(sGain)
    sGain.connect(c.destination)
    noise.start(now)
  } catch (e) {}
}

// 6. 斟靓茶：14 颗上升水泡（400→860Hz）+ 带通潺潺流水白噪（1.4s）
function teaPour() {
  var c = getCtx()
  if (!c) return
  try {
    var now = c.currentTime
    var duration = 1.4
    var bubbleCount = 14
    for (var b = 0; b < bubbleCount; b++) {
      var bubbleTime = now + (b * (duration / bubbleCount)) + (Math.random() * 0.04)
      var osc = c.createOscillator()
      var gain = c.createGain()
      var baseFreq = 400 + (b * 32) + (Math.random() * 60)
      osc.type = 'sine'
      osc.frequency.setValueAtTime(baseFreq, bubbleTime)
      osc.frequency.exponentialRampToValueAtTime(baseFreq * 1.35, bubbleTime + 0.07)
      gain.gain.setValueAtTime(0.18, bubbleTime)
      gain.gain.exponentialRampToValueAtTime(0.001, bubbleTime + 0.08)
      osc.connect(gain)
      gain.connect(c.destination)
      osc.start(bubbleTime)
      osc.stop(bubbleTime + 0.09)
    }

    var flowLen = Math.floor(c.sampleRate * duration)
    var flowBuf = c.createBuffer(1, flowLen, c.sampleRate)
    var fd = flowBuf.getChannelData(0)
    for (var j = 0; j < flowLen; j++) fd[j] = Math.random() * 2 - 1
    var flowSource = c.createBufferSource()
    flowSource.buffer = flowBuf
    var flowFilter = c.createBiquadFilter()
    flowFilter.type = 'bandpass'
    flowFilter.frequency.setValueAtTime(1400, now)
    flowFilter.frequency.linearRampToValueAtTime(2100, now + duration)
    flowFilter.Q.setValueAtTime(3.5, now)
    var flowGain = c.createGain()
    flowGain.gain.setValueAtTime(0.05, now)
    flowGain.gain.linearRampToValueAtTime(0.2, now + 0.25)
    flowGain.gain.linearRampToValueAtTime(0.15, now + duration - 0.3)
    flowGain.gain.exponentialRampToValueAtTime(0.001, now + duration)
    flowSource.connect(flowFilter)
    flowFilter.connect(flowGain)
    flowGain.connect(c.destination)
    flowSource.start(now)
    flowSource.stop(now + duration + 0.1)
  } catch (e) {}
}

module.exports = {
  warmup: warmup,
  setEnabled: setEnabled,
  isEnabled: isEnabled,
  coinClink: coinClink,
  coinArrival: coinArrival,
  kick: kick,
  slipperWhoosh: slipperWhoosh,
  slipperHit: slipperHit,
  tomatoWhoosh: tomatoWhoosh,
  tomatoSplat: tomatoSplat,
  teaPour: teaPour
}

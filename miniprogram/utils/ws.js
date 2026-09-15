// utils/ws.js — 房间长连接（wx.connectSocket）
// 服务端推送协议：
//   {"type":"prop","data":{...}}   道具事件（即时回放动画）
//   {"type":"game"}                牌局状态变化（重拉 /games/:id）
//   {"type":"ledger"}              流水变化（重拉 /games/:id/adjustments）
//   {"type":"ping"}                服务端心跳（客户端需回 pong 保活）
// 断线由调用方处理重连；连接期间调用方应停用轮询。

// 创建房间 socket。opts: { baseURL, token, gameID, onMessage, onOpen, onClose }
// 返回 { close(), send(obj), isClosed() }
function createRoomSocket(opts) {
  var url = opts.baseURL.replace(/^http/, 'ws') +
    '/ws?token=' + encodeURIComponent(opts.token) +
    '&game_id=' + opts.gameID

  var task = wx.connectSocket({ url: url, fail: function() {} })
  var closedByUs = false
  var fired = false // onError/onClose 只回调一次
  var pingTimer = null

  function fireClose() {
    if (fired) return
    fired = true
    if (pingTimer) clearInterval(pingTimer)
    if (!closedByUs && opts.onClose) opts.onClose()
  }

  task.onOpen(function() {
    if (opts.onOpen) opts.onOpen()
    // 客户端保活：25s 一跳，重置服务端读超时
    pingTimer = setInterval(function() {
      send({ type: 'pong' })
    }, 25000)
  })

  task.onMessage(function(res) {
    var msg
    try { msg = JSON.parse(res.data) } catch (e) { return }
    if (msg.type === 'ping') { send({ type: 'pong' }); return }
    if (opts.onMessage) opts.onMessage(msg)
  })

  task.onError(function() { fireClose() })
  task.onClose(function() { fireClose() })

  function send(obj) {
    try { task.send({ data: JSON.stringify(obj) }) } catch (e) {}
  }

  return {
    close: function() {
      closedByUs = true
      if (pingTimer) clearInterval(pingTimer)
      try { task.close({ code: 1000 }) } catch (e) {}
    },
    send: send,
    isClosed: function() { return fired }
  }
}

module.exports = { createRoomSocket: createRoomSocket }

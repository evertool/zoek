#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""房间页横向溢出测量页：验证「整页能不能被左右拖动」。

把 room.wxml 的文档流部分（顶栏 / 全出血桌面 / 账单卡）按 1rpx = 0.5px 还原，
加载后检查 documentElement.scrollWidth 是否等于视口宽，并把所有越过视口右边界的元素列进 #measure。
配合 Chrome --dump-dom 读结果；?guard=1 可对比加 page{overflow-x:hidden} 前后的差异。
"""
import os
import sys

GUARD = sys.argv[1] if len(sys.argv) > 1 else '0'
EXTRA = sys.argv[2] if len(sys.argv) > 2 else '0'
OUT = "/Users/lkahung/workspace/zoek/docs/preview/room-overflow-measure.html"

HTML = """<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>房间页横向溢出测量</title>
<style>
  html, body { margin: 0; padding: 0; width: 375px; }
  /* page { overflow-x: hidden } 的等价物（?guard=1 时生效） */
  html.guard, body.guard { overflow-x: hidden; }
  body { width: 375px; background: #f8f9ff; color: #0b1c30;
    font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Helvetica Neue", sans-serif; }
  .page { min-height: 100vh; background: #f8f9ff; }
  /* 顶栏：与 room.wxss 一致（rpx÷2） */
  .top-bar { display: flex; align-items: center; gap: 8px; padding: 0 20px 4px; }
  .nav-back-btn { width: 32px; height: 32px; border-radius: 50%; background: #fff; box-shadow: 0 2px 8px -2px rgba(11,28,48,.06); display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
  .status-pill { display: flex; align-items: center; gap: 6px; padding: 4px 12px; border-radius: 999px; background: #a6f3c9; color: #002113; font-size: 12px; font-weight: 500; flex-shrink: 0; }
  .qr-icon-btn { width: 32px; height: 32px; border-radius: 50%; background: #fff; flex-shrink: 0; }
  /* 主内容：左右内边距 20px，桌面全出血 */
  .main-content { padding: 4px 20px 100px; display: flex; flex-direction: column; gap: 16px; }
  .table-stage { position: relative; width: calc(375px + __EXTRA__px); margin-left: -20px; height: 315px; flex-shrink: 0; }
  .table-center { position: absolute; left: 187.5px; top: 157.5px; width: 95px; height: 95px; margin: -47.5px 0 0 -47.5px; border-radius: 50%; background: rgba(255,255,255,.72); border: 1px solid rgba(27,107,74,.16); z-index: 3; }
  .wind-dot { position: absolute; width: 26px; height: 26px; border-radius: 50%; z-index: 4; }
  .wd-top { left: 174.5px; top: 97px; background: #dda43c; }
  .wd-left { left: 127px; top: 144.5px; background: #4a7fd4; }
  .wd-right { left: 222px; top: 144.5px; background: #1b6b4a; }
  .wd-bottom { left: 174.5px; top: 192px; background: #9b7fb0; }
  .table-side { position: absolute; background: rgba(27,107,74,.18); z-index: 1; }
  .side-top    { left: 108.5px; top: 5px;   width: 158px; height: 105px; }
  .side-bottom { left: 108.5px; top: 205px; width: 158px; height: 105px; }
  .side-left   { left: 35px;     top: 78.5px; width: 105px; height: 158px; }
  .side-right  { left: 235px;    top: 78.5px; width: 105px; height: 158px; }
  /* 压力测试：超长昵称 / 超长状态文案 / 超长账单描述，验证省略号与 min-width:0 是否兜得住 */
  .seat-name { font-size: 12px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ledger-card { background: #fff; border-radius: 16px; padding: 16px; box-shadow: 0 2px 8px -2px rgba(11,28,48,.06); }
  .ledger-item { display: flex; align-items: center; gap: 8px; padding: 7px 0; }
  .ledger-info { flex: 1; min-width: 0; }
  .ledger-item-title { font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ledger-item-score { font-size: 19px; font-weight: 700; flex-shrink: 0; }
  #measure { position: fixed; right: 6px; top: 6px; z-index: 99; font: 11px/1.5 monospace; background: #fff; border: 1px solid #ccc; padding: 8px; white-space: pre; }
</style>
</head>
<body class="__GUARD__">
  <div class="page">
    <div class="top-bar">
      <div class="nav-back-btn">‹</div>
      <div class="status-pill">进行中 · 4/4 入座</div>
      <div class="qr-icon-btn"></div>
    </div>
    <div class="main-content">
      <div class="table-stage">
        <div class="table-center"></div>
        <div class="wind-dot wd-top"></div><div class="wind-dot wd-left"></div>
        <div class="wind-dot wd-right"></div><div class="wind-dot wd-bottom"></div>
        <div class="table-side side-top"></div>
        <div class="table-side side-bottom"></div>
        <div class="table-side side-left"></div>
        <div class="table-side side-right"></div>
      </div>
      <div class="ledger-card">
        <div class="ledger-item">
          <div class="ledger-info">
            <div class="ledger-item-title">这是一条特别特别长的账单描述，用来压测不换行文本会不会把整页撑宽导致可以左右拖动整个页面</div>
          </div>
          <div class="ledger-item-score">+42</div>
        </div>
      </div>
    </div>
  </div>
<pre id="measure">measuring…</pre>
<script>
  const vw = 375;                                  // 设计视口（750rpx ÷ 2）
  const sw = document.body.scrollWidth;            // 内容实际需要的宽度（> vw 即可横向拖动）
  const offenders = [];
  document.querySelectorAll('.page *').forEach(el => {
    const r = el.getBoundingClientRect();
    if (r.right > vw + 0.5 || r.left < -0.5) {
      offenders.push(el.className + '  left=' + r.left.toFixed(1) + ' right=' + r.right.toFixed(1));
    }
  });
  const lines = [
    'guard(overflow-x:hidden) = ' + (document.body.classList.contains('guard') ? 'ON' : 'OFF'),
    '视口宽 = ' + vw + '   scrollWidth = ' + sw,
    '横向可拖动 = ' + (sw > vw ? (document.body.classList.contains('guard') ? '否（overflow-x:hidden 已禁止拖动，仅存在溢出区域）' : '是（差 ' + (sw - vw).toFixed(1) + 'px）') : '否'),
    '越界元素数 = ' + offenders.length
  ].concat(offenders.slice(0, 8));
  document.getElementById('measure').textContent = lines.join('\\n');
</script>
</body>
</html>
"""

os.makedirs(os.path.dirname(OUT), exist_ok=True)
with open(OUT, "w", encoding="utf-8") as fh:
    fh.write(HTML.replace("__GUARD__", "guard" if GUARD == '1' else "").replace("__EXTRA__", EXTRA))
print("written:", OUT, "guard=", GUARD)

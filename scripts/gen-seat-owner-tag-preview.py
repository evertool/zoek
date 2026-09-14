#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""北位（下）座位「台主」标签与北风位圆点：改前 / 改后对照（3 倍放大）。

改前：.side-body gap 6rpx、.seat-meta gap 2rpx、.seat-name lh 1.5、.seat-score lh 1.1、内容居中
改后：gap 4rpx、meta gap 0、name lh 1.25、score lh 1，内容再朝远离中心方向让开 12rpx
1rpx = 0.5px（与真机 750rpx 设计宽度一致），整块放大 3 倍便于看细节。
"""
import os

OUT = "/Users/lkahung/workspace/zoek/docs/preview/room-owner-tag-before-after.html"
ZOOM = 2.5

HTML = """<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>北位台主标签 · 改前改后</title>
<style>
  * { box-sizing: border-box; }
  body { margin: 0; padding: 20px; background: #eef2f8; color: #0b1c30; font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Helvetica Neue", sans-serif; }
  h1 { font-size: 16px; margin: 0 0 4px; }
  .note { font-size: 12px; color: #5a6573; margin-bottom: 16px; line-height: 1.7; }
  .note b { color: #005235; }
  .row { display: flex; gap: 26px; align-items: flex-start; }
  .panel-title { font-size: 13px; font-weight: 700; margin-bottom: 8px; }
  .panel-title.bad { color: #b02d29; }
  .panel-title.good { color: #005235; }
  .crop { width: 440px; height: 300px; overflow: hidden; background: #f8f9ff; border-radius: 14px; box-shadow: 0 10px 26px rgba(11,28,48,.12); position: relative; }
  /* 只裁「下位座位 + 中心 + 北圆点」那一块，放大 3 倍 */
  .zoom { position: absolute; left: -187.5px; top: -437.5px; transform: scale(__ZOOM__); transform-origin: 0 0; }
  .table-stage { position: relative; width: 375px; height: 315px; }
  .table-center { position: absolute; left: 187.5px; top: 157.5px; width: 95px; height: 95px; margin: -47.5px 0 0 -47.5px; border-radius: 50%; background: rgba(255,255,255,.72); border: 1px solid rgba(27,107,74,.16); z-index: 3; }
  .wind-dot-bottom { position: absolute; left: 174.5px; top: 192px; width: 26px; height: 26px; border-radius: 50%; background: #9b7fb0; color: #fff; font-size: 12px; font-weight: 700; display: flex; align-items: center; justify-content: center; box-shadow: 0 2px 5px -2px rgba(11,28,48,.3); z-index: 4; }
  .table-side { position: absolute; left: 108.5px; top: 205px; width: 158px; height: 105px; background: rgba(27,107,74,.18); z-index: 1; }
  .side-fill { position: absolute; top: 1.5px; left: 1.5px; right: 1.5px; bottom: 1.5px; background: #fff; display: flex; align-items: center; justify-content: center; }
  .side-body { width: 100%; max-width: 85px; display: flex; flex-direction: column; align-items: center; }
  .seat-head { display: flex; align-items: center; gap: 4px; width: 100%; min-width: 0; }
  .seat-avatar { width: 28px; height: 28px; border-radius: 50%; background: #dce9ff; flex-shrink: 0; display: flex; align-items: center; justify-content: center; font-size: 13px; font-weight: 600; }
  .seat-meta { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .seat-owner-tag { align-self: flex-start; padding: 1px 5px; border-radius: 4px; background: #f6eedc; color: #8a6a1f; font-size: 9px; font-weight: 700; }
  .seat-name { font-size: 12px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .seat-score { font-size: 21px; font-weight: 700; color: #005235; }
  .give-btn { width: 100%; height: 28px; border-radius: 999px; border: 1px solid rgba(27,107,74,.35); color: #005235; display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 600; }
  /* 改前（原样式） */
  .before .side-body { gap: 3px; }
  .before .seat-meta { gap: 1px; }
  .before .seat-name { line-height: 1.5; }
  .before .seat-score { line-height: 1.1; }
  /* 改后（当前 room.wxss） */
  .after .side-body { gap: 2px; transform: translateY(6px); }
  .after .seat-meta { gap: 0; }
  .after .seat-name { line-height: 1.25; }
  .after .seat-score { line-height: 1; }
</style>
</head>
<body>
  <h1>北位（台主）· 「台主」标签与北风位圆点</h1>
  <div class="note">
    改前：标签被北圆点竖直盖住 <b>11.1rpx</b>（水平几乎整块重叠）。<br/>
    改后：先把座位内 <code>.side-body</code> 行距收紧腾出空间，再把内容整体下移 12rpx →
    标签与圆点之间留出 <b>8rpx</b> 间隙，底部距梯形内边仍有 <b>7rpx</b>。
  </div>
  <div class="row">
    <div>
      <div class="panel-title bad">改前（原样）</div>
      <div class="crop"><div class="zoom"><div class="table-stage before">
        <div class="table-center"></div>
        <div class="wind-dot-bottom">北</div>
        <div class="table-side" id="sideA">
          <div class="side-fill" id="fillA">
            <div class="side-body">
              <div class="seat-head"><div class="seat-avatar">细</div>
                <div class="seat-meta"><div class="seat-owner-tag">台主</div><div class="seat-name">细辉</div></div>
              </div>
              <div class="seat-score">+42</div>
              <div class="give-btn">＋ 给分</div>
            </div>
          </div>
        </div>
      </div></div></div>
    </div>
    <div>
      <div class="panel-title good">改后（当前 room.wxss）</div>
      <div class="crop"><div class="zoom"><div class="table-stage after">
        <div class="table-center"></div>
        <div class="wind-dot-bottom">北</div>
        <div class="table-side" id="sideB">
          <div class="side-fill" id="fillB">
            <div class="side-body">
              <div class="seat-head"><div class="seat-avatar">细</div>
                <div class="seat-meta"><div class="seat-owner-tag">台主</div><div class="seat-name">细辉</div></div>
              </div>
              <div class="seat-score">+42</div>
              <div class="give-btn">＋ 给分</div>
            </div>
          </div>
        </div>
      </div></div></div>
    </div>
  </div>
<script>
  const POLY = 'polygon(18.6% 6.8%, 19.4% 4.1%, 20.8% 1.9%, 22.6% 0.5%, 24.6% 0.0%, 75.4% 0.0%, 77.4% 0.5%, 79.2% 1.9%, 80.6% 4.1%, 81.4% 6.8%, 97.6% 87.7%, 97.7% 92.1%, 96.6% 96.2%, 94.3% 99.0%, 91.5% 100.0%, 8.5% 100.0%, 5.7% 99.0%, 3.4% 96.2%, 2.3% 92.1%, 2.4% 87.7%)';
  for (const id of ['sideA', 'sideB']) document.getElementById(id).style.clipPath = POLY;
  for (const id of ['fillA', 'fillB']) document.getElementById(id).style.clipPath = POLY;
</script>
</body>
</html>
"""

os.makedirs(os.path.dirname(OUT), exist_ok=True)
with open(OUT, "w", encoding="utf-8") as fh:
    fh.write(HTML.replace("__ZOOM__", str(ZOOM)))
print("written:", OUT)

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""房间页互动道具动画的视觉预览：直接复用 room.wxss 的 fx 样式（rpx ÷ 2 = px，数值零漂移），
用「负 animation-delay + paused」把每套动画定格在关键帧上截图核对。"""
import os
import re

ROOT = "/Users/lkahung/workspace/zoek"
WXSS = os.path.join(ROOT, "miniprogram/pages/room/room.wxss")
OUT = os.path.join(ROOT, "docs/preview/room-prop-fx-preview.html")

src = open(WXSS, encoding="utf-8").read()
start = src.index("/* ========== 席位互动道具 fx 层")
fx_css = src[start:]
# 去掉道具抽屉样式（预览用不到）
fx_css = fx_css[: fx_css.index("/* 道具抽屉")]
# rpx → px（÷2），负号也要处理
fx_css = re.sub(r"(-?\d+(?:\.\d+)?)rpx", lambda m: f"{float(m.group(1)) / 2:g}px", fx_css)

HTML = """<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>席位互动道具动画 · 关键帧预览</title>
<style>
  * { box-sizing: border-box; }
  body { margin: 0; padding: 26px 20px 50px; background: #eef2f8; color: #0b1c30;
    font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Helvetica Neue", sans-serif; }
  h1 { font-size: 17px; margin: 0 auto 6px; max-width: 860px; }
  .note { font-size: 12px; color: #5a6573; max-width: 860px; margin: 0 auto 18px; line-height: 1.7; }
  .note code { background: #fff; padding: 1px 5px; border-radius: 5px; font-size: 11px; }
  .grid { display: flex; flex-wrap: wrap; gap: 22px; justify-content: center; max-width: 860px; margin: 0 auto; }
  .panel-title { font-size: 12px; font-weight: 700; color: #33465c; margin-bottom: 6px; }
  .panel { width: 402px; }
  /* 舞台 750×630 rpx → 375×315px，加浅色描边便于观察 */
  .table-stage { position: relative; width: 375px; height: 315px; background: #eff4ff; border-radius: 12px; box-shadow: 0 10px 26px rgba(11,28,48,.10); }
  .fx-stage { pointer-events: none; }
  /* 席位占位（真实页面里是圆角梯形座位） */
  .side { position: absolute; background: #fff; border: 1px solid rgba(27,107,74,.15); box-shadow: 0 2px 8px -2px rgba(11,28,48,.06); display: flex; align-items: center; justify-content: center; }
  .side-top    { left: 108.5px; top: 5px;   width: 158px; height: 105px; border-radius: 14px 14px 8px 8px; }
  .side-bottom { left: 108.5px; top: 205px; width: 158px; height: 105px; border-radius: 8px 8px 14px 14px; }
  .side-left   { left: 35px;     top: 78.5px; width: 105px; height: 158px; border-radius: 14px 8px 8px 14px; }
  .side-right  { left: 235px;    top: 78.5px; width: 105px; height: 158px; border-radius: 8px 14px 14px 8px; }
  .avatar { width: 28px; height: 28px; border-radius: 50%; background: #dce9ff; color: #0b1c30; font-size: 13px; font-weight: 600; display: flex; align-items: center; justify-content: center; }
  .table-center { position: absolute; left: 187.5px; top: 157.5px; width: 95px; height: 95px; margin: -47.5px 0 0 -47.5px; border-radius: 50%; background: rgba(255,255,255,.72); border: 1px solid rgba(27,107,74,.16); display: flex; align-items: center; justify-content: center; font-size: 30px; font-weight: 700; color: #005235; z-index: 3; }
  .fx-caption { position: absolute; left: 8px; bottom: 6px; font-size: 10px; color: #6f7a72; z-index: 40; }
/* ===== room.wxss fx 样式（rpx 已换算 px） ===== */
__FX_CSS__
/* ===== 各面板的定格状态（负 delay + paused = 冻结在关键帧） ===== */
.p-kick .table-stage { animation-play-state: paused; }
.p-kick .fx-foot { animation-play-state: paused; animation-delay: -0.30s; opacity: 1; }
.p-kick .fx-run-1 { animation-play-state: paused; animation-delay: -0.10s; }
.p-kick .fx-run-2 { animation-play-state: paused; animation-delay: 0s; }
.p-kick .fx-run-3 { animation-play-state: paused; animation-delay: 0s; }
.p-kick .fx-avatar-kick { animation-play-state: paused; animation-delay: -0.17s; }
.p-kick .fx-sparks.fx-on, .p-kick .fx-banner.fx-on { opacity: 1; transform: none; }
.p-kick .fx-banner { transform: none; }
.p-slipper .fx-slipper, .p-slipper .fx-slipper-inner { animation-play-state: paused; animation-delay: -0.325s; }
.p-slipper .fx-stars, .p-slipper .side-fill { opacity: 0.999; }
.p-slipper .fx-star { animation-play-state: paused; animation-delay: -0.2s; }
.p-slipper .side-fill { animation-play-state: paused; animation-delay: -0.2s; }
.p-flower .fx-flower { opacity: 1; }
.p-flower .fx-run-droop, .p-flower .fx-run, .p-flower .fx-rain view,
.p-flower .fx-wind view, .p-flower .fx-flower-bubble { animation-play-state: paused; }
.p-flower .fx-run-droop { animation-delay: -1.6s; }
.p-flower .p1 { animation-delay: -0.9s; }
.p-flower .p2 { animation-delay: -0.75s; }
.p-flower .p3 { animation-delay: -0.6s; }
.p-flower .fx-rain view, .p-flower .fx-wind view { animation-delay: -0.4s; }
.p-flower .fx-flower-bubble { opacity: 1; transform: none; }
.p-tea .fx-tea-teapot { transform: rotate(-42deg); }
.p-tea .fx-tea-stream, .p-tea .cup-water { opacity: 1; }
.p-tea .fx-steam view, .p-tea .fx-dimsum-steam view, .p-tea .cup-water.fx-on { animation-play-state: paused; animation-delay: -0.6s; opacity: 0.75; }
.p-tea .fx-tea-stream { opacity: 1; }
.p-tea .cup-water.fx-on { transform: scale(1.05); opacity: 1; }
.p-tea .fx-tea-teapot { transform: rotate(-42deg); }
.p-tea .fx-tea-teapot { transition: none; }
.p-dimsum .fx-dimsum-lid.fx-run, .p-dimsum .fx-dimsum-steam view { animation-play-state: paused; }
.p-dimsum .fx-dimsum-lid.fx-run { animation-delay: -0.9s; }
.p-dimsum .fx-dimsum-steam view { animation-delay: -0.6s; opacity: 0.8; }
</style>
</head>
<body>
  <h1>席位互动道具动画 · 关键帧预览</h1>
  <div class="note">
    样式直接取自 <code>miniprogram/pages/room/room.wxss</code> 的 fx 段（rpx ÷ 2 = px，数值零漂移），
    用「负 <code>animation-delay</code> + <code>paused</code>」定格关键帧。
    完整时序：拖鞋 0.65s 飞行 → 命中；猛踢 0.3s 命中 → 地震 0.75s + 气泡 4.2s；花儿 3.8s；斟茶 2.8s；点心 2.7s。
  </div>
  <div class="grid">
    <div class="panel">
      <div class="panel-title">台下猛踢 🦶（命中瞬间：地震 + 冲击波 + 头像弹飞 + 暗号气泡）</div>
      <div class="table-stage p-kick" style="animation: none;">
        <div class="fx-stage">
          <div class="fx-shock-center" style="left: 187px; top: 257px;">
            <div class="fx-shock fx-shock-1 fx-run-1"></div>
            <div class="fx-shock fx-shock-2 fx-run-2"></div>
            <div class="fx-shock fx-shock-3 fx-run-3"></div>
          </div>
          <div class="fx-foot fx-run" style="left: 143px; top: 212px;">🦶</div>
          <div class="fx-sparks fx-on" style="left: 187px; top: 257px;">
            <text>💥</text><text class="fx-spark-b">💢</text><text class="fx-spark-c">⚡</text><text class="fx-spark-tag">CRITICAL HIT!</text>
          </div>
          <div class="side side-bottom" style="animation: none;"><div class="side-fill fx-avatar-kick"><div class="avatar">强</div></div></div>
          <div class="fx-banner fx-on">
            <text class="banner-icon">🦶</text>
            <div style="flex:1;min-width:0;">
              <text class="banner-title">大力踢！哎呀！踢咗【阿强】一脚！</text>
              <div class="banner-desc"><text>台底踢咁大啖，脚趾尾都抽筋！全桌得我知你踢我！</text></div>
            </div>
          </div>
          <div class="fx-caption">下位 · 阿强</div>
        </div>
      </div>
    </div>
    <div class="panel">
      <div class="panel-title">扔飞拖鞋 🩴（抛物线中段 + 命中星芒 + 目标抖动）</div>
      <div class="table-stage p-slipper">
        <div class="fx-stage">
          <div class="fx-slipper" style="--sx: 300px; --sy: 157px; --dx: 179px; --dy: 257px;">
            <div class="fx-slipper-inner"><image src="__ICON__/fx-slipper.svg" style="width:52px;height:52px;display:block;" /></div>
          </div>
          <div class="fx-stars fx-on" style="left: 179px; top: 257px;">
            <text class="fx-star">⭐</text><text class="fx-star-b">💥</text><text class="fx-star-c">✨</text>
          </div>
          <div class="side side-right"><div class="avatar">我</div></div>
          <div class="side side-bottom"><div class="side-fill fx-hit-shake" style="animation-play-state: paused; animation-delay: -0.2s;"><div class="avatar">强</div></div></div>
          <div class="fx-caption">右位(我) → 下位 · 阿强</div>
        </div>
      </div>
    </div>
    <div class="panel">
      <div class="panel-title">花儿谢了 🥀（枯萎垂头 + 花瓣飘落 + 愁云雨丝寒风 + 气泡）</div>
      <div class="table-stage p-flower">
        <div class="fx-stage">
          <div class="fx-flower fx-on" style="left: 187px; top: 122px; transform: translateX(-50%);">
            <div class="fx-flower-cloud"><text>🌧️</text><text>☁️</text></div>
            <div class="fx-rain"><view class="r1"></view><view class="r2"></view><view class="r3"></view></div>
            <div class="fx-wind"><view></view><view class="w2"></view></div>
            <div class="fx-flower-stem fx-run-droop">
              <text class="fx-flower-head">🥀</text>
              <div class="fx-stem"></div>
            </div>
            <div class="fx-petals">
              <text class="p1 fx-run">🥀</text><text class="p2 fx-run">🍂</text><text class="p3 fx-run">🍁</text>
            </div>
            <div class="fx-flower-bubble fx-on"><text>等得我花儿都谢了... 🥀</text></div>
          </div>
          <div class="side side-top"><div class="avatar">萍</div></div>
          <div class="fx-caption">上位 · 阿萍</div>
        </div>
      </div>
    </div>
    <div class="panel">
      <div class="panel-title">斟杯靓茶 🍵 + 送件点心 🥟（倾斜注水 / 掀盖白雾）</div>
      <div class="table-stage p-tea">
        <div class="fx-stage">
          <div class="fx-tea-teapot fx-tilt" style="left: 168px; top: 47px;"><image src="__ICON__/fx-teapot.svg" style="width:105px;height:85px;display:block;" /></div>
          <div class="fx-tea-stream fx-on" style="left: 203px; top: 72px; height: 52px;"></div>
          <div class="fx-tea-cup" style="left: 143px; top: 127px;">
            <div class="fx-steam"><view class="s1"></view><view class="s2"></view><view class="s3"></view></div>
            <div class="cup-body"><div class="cup-water fx-on"><text style="font-size:9px;color:#451a03;font-weight:700;">茶香</text></div></div>
            <div class="cup-label"><text style="font-size:9px;color:#fde68a;">敬上一杯高山靓乌龙</text></div>
          </div>
          <div class="fx-caption">西位 · 卡窿二</div>
        </div>
      </div>
    </div>
    <div class="panel">
      <div class="panel-title">送件点心 🥟（蒸笼掀盖 + 白雾 + 水晶虾饺）</div>
      <div class="table-stage p-dimsum">
        <div class="fx-stage">
          <div class="fx-dimsum" style="left: 187px; top: 107px; transform: translateX(-50%);">
            <div class="fx-dimsum-steam"><view class="s1"></view><view class="s2"></view></view>
            <div class="fx-dimsum-lid fx-run"><image src="__ICON__/fx-steamer-lid.svg" style="width:78px;height:34px;display:block;" /></div>
            <div class="fx-dimsum-base"><text style="font-size:24px;">🥟</text><text style="font-size:24px;">🥟</text></div>
            <div class="fx-dimsum-label"><text style="font-size:10px;color:#fde68a;">热腾腾广式虾饺 · 催牌啦！</text></div>
          </div>
          <div class="side side-right"><div class="avatar">我</div></div>
          <div class="fx-caption">右位 · 勿忘我</div>
        </div>
      </div>
    </div>
  </div>
</body>
</html>
"""

html = HTML.replace("__FX_CSS__", fx_css).replace("__ICON__", "/Users/lkahung/workspace/zoek/miniprogram/assets/icons")
os.makedirs(os.path.dirname(OUT), exist_ok=True)
with open(OUT, "w", encoding="utf-8") as fh:
    fh.write(html)
print("written:", OUT, len(html), "bytes")

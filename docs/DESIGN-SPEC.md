# 得闲开台 — 小程序 UI 重构设计规范（v2）

> 依据 docs/design/1.png、2.png 设计图整理。设计图为私有效果参考，实现时用 CSS 复刻视觉，禁止把 docs/design 下的大 PNG 打进小程序包（单张 2~4MB，超出主包限制）。

## 设计语言

- **风格**：新中式麻将馆 —— 米白宣纸底、牌桌深绿、红橙印章色、金色点缀、3D 麻将牌插画母题。
- **背景**：`#F7F3E8`（米白宣纸），卡片 `#FFFFFF` / `#FCFAF3`，边框 `#EDE7D8`。
- **主色（品牌绿）**：`#1E6B47`，深 `#145036`，浅底 `#E4F0E9`。用于次级强调、tab 选中、深绿主视觉卡。
- **CTA 红**：`#E8442E`（渐变 `#F0603F → #E8442E`），浅底 `#FDE8E4`。用于最重要的主按钮（开台、提交、确认锁定）。
- **金色点缀**：`#C9A254`，浅底 `#F6EEDC`。用于称号、冠军、徽章。
- **文字**：主 `#23352B`，次 `#7A8480`，弱 `#B8C0BB`。
- **圆角**：卡片 28rpx，按钮胶囊 48rpx，标签 20rpx。
- **阴影**：卡片 `0 6rpx 28rpx rgba(35,53,43,.06)`；绿卡 `0 12rpx 32rpx rgba(30,107,71,.3)`。

## 可复用全局类（app.wxss）

- 容器：`.page`、`.card`、`.card-warm`、`.card-green`（深绿渐变实底卡，白字）
- 按钮：`.btn-primary`（红橙渐变 CTA）、`.btn-green`（深绿）、`.btn-secondary`、`.btn-danger`
- 麻将牌母题：`.mj-tile`（米白立体牌面 + 底部厚度阴影，红字），`.mj-tile-green` / `.mj-tile-gold` 变体。需要设置 width/height/font-size。
- 标签：`.tag` + `.tag-forming/.tag-active/.tag-ended/.tag-cancelled/.tag-pending/.tag-accepted`
- 文字：`.text-secondary/.text-positive/.text-negative/.text-zero/.text-success/.text-warning/.text-error`
- 固定底栏：`.footer-bar`

## 各页面视觉要点（来自设计图）

1. **index 首页**：顶部品牌区（大号 .mj-tile 「中」+「得闲开台」标题）；开台入口为深绿大卡（.card-green，含麻将牌图形 + 红橙箭头按钮）；进行中牌局为白卡列表。未登录态：居中 3D 牌堆视觉 + 品牌名 + 副标语 + 红 CTA「微信登入」。
2. **create 开台**：白卡表单；台名输入框（暖白底圆角）；人数选择用 4 个麻将牌样式选择块；底部红橙大 CTA「开台」。
3. **room 房间**：深绿主视觉头部卡（台名、状态、成员 2/4、局号）；成员列表用带头像圆形 + 「你」标签 + 台主金标；QR 邀请卡（白底 + 绿边框虚线内框）；操作按钮区。
4. **join 入台**：牌桌预览卡（深绿）+ 确认入台红 CTA + 昵称确认。
5. **score 记分**：顶部深绿状态条（第 N 局 · 已完成 M 局 · 已入 x/y）；分数输入区为大号数字键盘风格输入；review 态列出每人分数牌（.mj-tile 展示 ±分）；确认锁定为红 CTA + 二次确认。
6. **adjustment 补分/退分**：类型选择（两个对称大块：补分红 / 退分绿）；对方选择列表；金额步进器；原因输入。
7. **settlement 结算**：冠军 podium 视觉（第一名金 + 麻将牌王冠/牌堆图形，二三位并列卡）；名次列表带奖牌色（金 #C9A254 / 银 #B9C0C8 / 铜 #C98A5B）；称号用金色标签。
8. **history 对局记录**：月份分组列表，白卡行（台名 + 局数 + 我的得分正负着色）。
9. **detail 单场详情**：逐局时间线（局号用小 .mj-tile）；调整记录作为追加卡片（金色左边条）。
10. **profile 我的**：顶部深绿个人卡（头像圆框金边 + 昵称）；设置列表白卡分组；「关于得闲开台」区。

## 通用规则

- 文案保持粤语口语规范（docs/BRAND.md 第五章）。
- 品牌名一律「得闲开台」，禁止出现「雀友记」「雀记」。
- 只改 wxml/wxss/json（navigationBarTitleText 按页面语义设置），**不改 js 逻辑与数据绑定**；wxml 中所有 bindtap/catchtap、数据字段必须与现有 js 完全一致。
- rpx 单位，750 设计稿。

#!/usr/bin/env node
/**
 * scripts/check-mp-client.js — 小程序端「审核致命项」回归
 *
 * 背景：2026-09-18 提审被拒（《常见拒绝情形》3.3 可用性与完整性），
 * 原因是 app.js 用 envVersion 选 baseURL，而微信官方文档写明
 * 「develop：开发版，**提交代码审核时默认使用开发版进行审核**」，
 * 于是审核端拿到局域网地址 http://192.168.1.15:8080，所有请求必然失败，
 * 首页整页「网络开小差」。
 *
 * 本脚本用 stub 跑真实源码，锁住两条铁律：
 *   1. 真机（含审核环境）的 baseURL 必须是 https 正式域名 —— 任何 envVersion 都不能改变它；
 *   2. 静默登录失败时首页先自动重试，不立刻渲染报错兜底条。
 *
 * 用法：node scripts/check-mp-client.js
 */
'use strict'

const path = require('path')
const ROOT = path.resolve(__dirname, '..')
const APP = path.join(ROOT, 'miniprogram/app.js')
const INDEX = path.join(ROOT, 'miniprogram/pages/index/index.js')
const API = path.join(ROOT, 'miniprogram/utils/api.js')
const UTIL = path.join(ROOT, 'miniprogram/utils/util.js')

const PROD = 'https://zoek.246891.xyz/api/v1'
const DEV = 'http://192.168.1.15:8080/api/v1'
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

let fail = 0
function check(name, cond, extra) {
  console.log(`${cond ? '  PASS' : '  FAIL'}  ${name}${cond ? '' : '  <- ' + JSON.stringify(extra)}`)
  if (!cond) fail++
}

// ---------------------------------------------------------------------------
// 1) baseURL 环境判定
// ---------------------------------------------------------------------------
function loadApp(opts) {
  const storage = Object.assign({}, opts.storage || {})
  delete require.cache[require.resolve(APP)]
  let appDef = null
  global.App = (o) => { appDef = o }
  global.wx = {
    getAccountInfoSync: () => {
      if (opts.accountInfoThrows) throw new Error('boom')
      return { miniProgram: { envVersion: opts.envVersion } }
    },
    getSystemInfoSync: () => {
      if (opts.systemInfoThrows) throw new Error('boom')
      return { platform: opts.platform }
    },
    getStorageSync: (k) => storage[k],
    setStorageSync: () => {},
    removeStorageSync: () => {},
    getFileSystemManager: () => ({}),
  }
  require(APP)
  return appDef
}

function testBaseURL() {
  console.log('\n[1/2] baseURL 环境判定（真机必须走正式域名）')
  const cases = [
    // 审核端：真机 + envVersion=develop —— 本次拒审的场景，必须走正式域名
    { name: '审核端 android + envVersion=develop', platform: 'android', envVersion: 'develop', want: PROD },
    { name: '审核端 ios + envVersion=develop', platform: 'ios', envVersion: 'develop', want: PROD },
    { name: '审核端 harmony + envVersion=develop', platform: 'harmony', envVersion: 'develop', want: PROD },
    { name: '体验版 ios + trial', platform: 'ios', envVersion: 'trial', want: PROD },
    { name: '正式版 android + release', platform: 'android', envVersion: 'release', want: PROD },
    // 开发者工具：默认连本地（保持开发习惯）
    { name: '开发者工具 默认 → 本地', platform: 'devtools', envVersion: 'develop', want: DEV },
    { name: '开发者工具 + use_prod_api=1 → 正式', platform: 'devtools', envVersion: 'develop', storage: { use_prod_api: 1 }, want: PROD },
    // 异常兜底：取不到环境信息一律按正式域名
    { name: 'getAccountInfoSync 抛异常', platform: 'ios', accountInfoThrows: true, want: PROD },
    { name: 'getSystemInfoSync 抛异常', platform: '', systemInfoThrows: true, want: PROD },
    { name: 'platform 为空', platform: '', envVersion: 'release', want: PROD },
  ]
  for (const c of cases) {
    const got = loadApp(c).globalData.baseURL
    check(c.name, got === c.want, { got, want: c.want })
  }
  // envVersion 仍须保留：台码生成要用它决定扫码打开哪个版本
  const env = loadApp({ platform: 'android', envVersion: 'develop' }).globalData.envVersion
  check('envVersion 仍透传给台码逻辑', env === 'develop', { env })
}

// ---------------------------------------------------------------------------
// 2) 首页静默登录失败 → 自动重试
// ---------------------------------------------------------------------------
function setupIndex(opts) {
  let page = null
  let loginCalls = 0
  let gamesCalls = 0

  const app = {
    globalData: { token: opts.token || '', baseURL: PROD, pendingRoute: '' },
    ready: () => Promise.resolve(),
    login: () => {
      loginCalls++
      if (opts.loginFails) return Promise.reject(new Error('network down'))
      app.globalData.token = 'tok-fresh'
      return Promise.resolve('tok-fresh')
    },
    logout: () => { app.globalData.token = '' },
    checkProfileNeeded: () => false,
  }

  global.getApp = () => app
  global.Page = (o) => { page = o }
  global.wx = {
    getStorageSync: () => '',
    setStorageSync: () => {},
    removeStorageSync: () => {},
    showToast: () => {},
    showLoading: () => {},
    hideLoading: () => {},
    stopPullDownRefresh: () => {},
    getAccountInfoSync: () => ({ miniProgram: { envVersion: 'release' } }),
    getSystemInfoSync: () => ({ platform: 'ios', statusBarHeight: 20, screenWidth: 375, screenHeight: 812, safeArea: {} }),
    request: (o) => {
      if (String(o.url).indexOf('/games/active') >= 0) gamesCalls++
      o.success({ statusCode: 200, data: { games: [], recent: [] } })
    },
  }

  for (const f of [INDEX, API, UTIL]) delete require.cache[require.resolve(f)]
  require(INDEX)

  page.data = Object.assign({}, page.data)
  page.setData = function (o) { Object.assign(this.data, o) }
  return { page, counters: () => ({ loginCalls, gamesCalls }) }
}

async function testIndexRetry() {
  console.log('\n[2/2] 首页静默登录失败 → 自动重试')

  // 登录持续失败：前两次重试不打扰，第三次才露出兜底条
  {
    const s = setupIndex({ token: '', loginFails: true })
    s.page.onShow()
    await sleep(40)
    check('失败场景：初始不立刻显示报错兜底条', s.page.data.loginFailed === false, s.page.data)
    check('失败场景：等待期间保持 loading（观感=加载中）', s.page.data.loading === true, s.page.data)
    await sleep(700 + 1500 + 600)
    check('失败场景：两次静默重试后露出兜底条', s.page.data.loginFailed === true, s.page.data)
    check('失败场景：确实重试了 2 次登录', s.counters().loginCalls === 2, s.counters())
  }

  // 第一次重试即成功
  {
    const s = setupIndex({ token: '', loginFails: false })
    s.page.onShow()
    await sleep(700 + 300)
    check('成功场景：兜底条不出现', s.page.data.loginFailed === false, s.page.data)
    check('成功场景：拿到 token 后拉起列表', s.counters().gamesCalls > 0, s.counters())
    s.page.onHide()
  }

  // 已有 token：直接加载，不触发重登
  {
    const s = setupIndex({ token: 'tok-cached' })
    s.page.onShow()
    await sleep(50)
    check('已登录场景：直接拉列表', s.counters().gamesCalls > 0, s.counters())
    check('已登录场景：不触发登录', s.counters().loginCalls === 0, s.counters())
    s.page.onHide()
  }

  // onHide 必须清掉重试定时器
  {
    const s = setupIndex({ token: '', loginFails: true })
    s.page.onShow()
    await sleep(40)
    s.page.onHide()
    const afterHide = s.counters().loginCalls
    await sleep(700 + 1500 + 400)
    check('onHide 后不再偷偷重试', s.counters().loginCalls === afterHide, s.counters())
    check('onHide 后不会补出兜底条', s.page.data.loginFailed === false, s.page.data)
  }
}

;(async () => {
  testBaseURL()
  await testIndexRetry()
  console.log(fail === 0 ? '\n小程序端回归：ALL PASS' : `\n小程序端回归：${fail} FAILED`)
  process.exit(fail === 0 ? 0 : 1)
})()

# zoek 部署说明（与 zola 同服务器共存版）

本项目部署物只有 **Go 后端二进制**（小程序端通过微信开发者工具上传，不走服务器）。
命名全部带 `zoek` 前缀，与 zola 的 `xingyue-api` / `/opt/xingyue` / 8080、8089 端口互不冲突。

## 目录与命名约定

| 项目 | zola（数匠） | zoek（雀友记） |
|---|---|---|
| 远程目录 | `/opt/xingyue` | `/opt/zoek` |
| systemd 服务 | `xingyue-api` | `zoek-api` |
| 后端端口 | 8080 / 8089 | **8090** |
| Caddy 站点 | `conf.d/xingyue.caddy` | `conf.d/zoek.caddy` |
| 二进制 | `/opt/xingyue/api` | `/opt/zoek/api` |

## 文件说明

- `release.env.example` — 发版配置模板（SSH、远程路径、端口），复制为 `release.env` 后填写（已 gitignore）
- `zoek-api.service` — systemd 单元（脚本会按 `REMOTE_ROOT` 动态生成，此文件作为参考/手动安装用）
- `remote-check.sh` — 服务器端部署自检脚本（由 `make check-deploy` 上传执行，也可手动拷到服务器跑）
- `Caddyfile` — Caddy 主配置（仅全局项 + `import conf.d/*.caddy`；zola 已装主文件时**不要覆盖**）
- `caddy/conf.d/zoek.caddy` — zoek 站点：`/api/*`、`/uploads/*`、`/health` 反代到 `127.0.0.1:8090`

## ⚠️ 关键：systemd 必须显式传 `-config`

`cmd/server/main.go` 的 `-config` 参数**默认是空字符串**，而空的路径会让 `config.Load("")`
**跳过读取配置文件、直接使用内置默认值**：

| 项 | 不传 `-config` 时的默认值 | 应有的值 |
|---|---|---|
| `server.port` | `8080` | `8090` |
| `database.password` | 空 | 真实密码 |
| `server.mode` | `debug` | `release` |
| `jwt.secret` | `zoek-dev-secret-change-in-production` | 自定义 |
| `wechat.appid/app_secret` | 空 | 真实值 |

所以 `ExecStart` 必须是：

```ini
ExecStart=/opt/zoek/api -config /opt/zoek/config.yaml
```

漏掉 `-config` 时的典型症状（两者会同时出现）：

- `curl http://127.0.0.1:8090/health` 不通（进程实际在听 8080），Caddy 反代 502
- `FATAL 数据库连接失败 {"error": "connect database: Error 1045 (28000): Access denied for user 'root'@'localhost' (using password: NO)"}`
- systemd 进入崩溃循环，`NRestarts` 一路涨

## 服务器首次准备

1. 建目录与配置：
   ```bash
   sudo mkdir -p /opt/zoek/uploads/avatars
   sudo cp backend/config.yaml /opt/zoek/config.yaml   # 改 server.port=8090、database.*、jwt.secret、wechat.*
   sudo chown -R zoek:zoek /opt/zoek/uploads           # 服务用户需可写
   ```
2. 建库 + 建账号（**不要**用 root，root 在 MySQL 8 常见为 auth_socket，TCP + 空密码必然 1045）：
   ```sql
   CREATE DATABASE IF NOT EXISTS zoek DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   CREATE USER IF NOT EXISTS 'zoek'@'localhost' IDENTIFIED BY '换成强密码';
   GRANT ALL PRIVILEGES ON zoek.* TO 'zoek'@'localhost';
   FLUSH PRIVILEGES;
   ```
   然后把该账号写进 `/opt/zoek/config.yaml` 的 `database.user/password`。
3. systemd unit（`ExecStart` 务必带 `-config`，见上一节）：
   ```bash
   sudo cp deploy/zoek-api.service /etc/systemd/system/zoek-api.service
   sudo systemctl daemon-reload && sudo systemctl enable --now zoek-api
   ```
4. Caddy 站点（zola 已装主 Caddyfile 时只拷站点文件）：
   ```bash
   sudo cp deploy/caddy/conf.d/zoek.caddy /etc/caddy/conf.d/zoek.246891.xyz.caddy
   sudo caddy validate --config /etc/caddy/Caddyfile && sudo systemctl reload caddy
   ```
   > 文件名按域名起更好认（服务器上现为 `zoek.246891.xyz.caddy`）；
   > 主 Caddyfile 里用 `import conf.d/*.caddy`，叫什么名字都会被加载。
   > ACME 通知邮箱写在**主 Caddyfile 顶部的全局块**里：`{ email you@example.com }`。
5. 微信公众平台：把该域名加进小程序 request 合法域名（必须是 **https**，不能填 IP/局域网地址）。
6. 小程序端 `miniprogram/app.js` 的 `ENV_CONFIG.trial` / `ENV_CONFIG.release` 指向正式域名。

## 发版

```bash
cp deploy/release.env.example deploy/release.env   # 填 SSH_HOST 等
./scripts/release.sh backend                       # 构建 linux 二进制 → 上传 → 重启 → 健康检查
./scripts/release.sh --build-only backend          # 只构建
./scripts/release.sh --check                       # 只做服务器端自检
```

发版脚本会自动：上传二进制（tmp + mv 原子替换）、确保 systemd unit 存在（无则自动安装，
**已存在时只校验 `-config` 不覆盖**）、重启 `zoek-api`、轮询 `http://127.0.0.1:8090/health`。
健康检查失败时会自动跑一遍 `remote-check.sh` 打印缺失项。

## 线上自检

```bash
make check-deploy        # 等价于 scripts/release.sh --check
```

覆盖：`-config` 参数、`config.yaml` 的端口/密码/JWT/微信密钥、MySQL 连通性与建库、
监听端口、uploads 属主、Caddy 站点与 80/443、`https://<REMOTE_SITE_DOMAIN>/health`。

## TLS 证书：源站用自签（`tls internal`），不碰 ACME

链路是 `客户端 → 阿里云 ESA 边缘 →(HTTPS 回源)→ 源站 Caddy:443`：

- **客户端看到的是 ESA 托管的证书**（当前 `CN=*.246891.xyz`，Let's Encrypt 签发，
  ESA 自动续期，到期 2026-11-26）
- 源站证书只用于 **ESA↔源站**这一段，而实测 **ESA 不校验源站证书**，只要求握手能完成
- 所以源站用 Caddy 内部 CA 自签即可：`tls internal`，自动签发 + 自动轮转，零维护

这台机器在国内，ACME 出网实测结果（都在这台机器上跑过）：

| CA | ACME 目录 | 结果 |
|---|---|---|
| Let's Encrypt | `acme-v02.api.letsencrypt.org`（落在 Cloudflare `172.65.32.248`） | ❌ HTTP 000，12s 超时 |
| ZeroSSL | `acme.zerossl.com/v2/DV90` | ✅ 1.0s，但**强制 EAB** |
| Actalis | `acme-api.actalis.com/acme/directory` | ✅ 可达，`externalAccountRequired=true` |
| Certum | `acme.certum.pl/directory` | ✅ 可达，`externalAccountRequired=true` |
| SSL.com | `acme.ssl.com/sslcom-dv-rsa` | ✅ 不要求 EAB，但注册要绑账单（401） |
| Buypass / Google | — | ❌ 不可达（Cloudflare 整段不通，`www.cloudflare.com` 也超时） |

站点块最终写法：

```caddyfile
zoek.246891.xyz {
	tls internal      # ← 不要写 tls { ca ...; email ... }，见下
	...
}
```

### 踩过的两个坑

1. **Caddy 2.6.2 的站点级 `tls` 不支持 `email`**（报 `unknown subdirective: email`）。
   `email` 只能写在**全局选项块**里（`/etc/caddy/Caddyfile` 顶部的 `{ email ... }`）。
2. **ZeroSSL 现在强制 EAB**，而 Caddy 2.6.2 自动取 EAB 的老接口
   `api.zerossl.com/acme/eab-credentials-email` 已返回 404，于是 Caddy 不带 EAB 去注册，
   被拒：`HTTP 400 externalAccountRequired`。
   如果哪天要用 ZeroSSL，得自己去 zerossl.com 注册拿 API Key，再取 EAB：
   ```bash
   curl "https://api.zerossl.com/acme/eab-credentials?access_key=<你的KEY>"
   # 返回 eab_kid / eab_hmac_key，写进 Caddy：
   #   tls { ca https://acme.zerossl.com/v2/DV90; eab <kid> <hmac> }
   ```

> ⚠️ **前提**：ESA 侧不要开启「源站证书校验」，否则自签证书会被拒，需要改用受信证书。
> 同理，`/etc/caddy/conf.d/246891.xyz.caddy`（主域）那张 Let's Encrypt 证书
> **2026-11-04 到期后无法续签**——但客户端看到的是 ESA 的证书，所以主域本身不受影响；
> 只有「ESA 回源校验源站证书」打开时才会出问题。

### 阿里云 ESA

- 域名解析到 ESA 边缘（响应头 `Server: ESA`），回源协议保持 **HTTPS** 即可
- 源站 Caddy 只要能用该 SNI 完成握手，ESA 就放行；没有证书时报
  **525 Origin SSL Handshake Error**
- 走 ACME 的话要在 ESA 放行 `/.well-known/acme-challenge/*`（不缓存、不改写）；
  用 `tls internal` 则完全不需要

## 故障排查

| 现象 | 原因 | 处理 |
|---|---|---|
| 健康检查 8090 不通 | ExecStart 缺 `-config`，进程在听 8080 | 补 `-config`，`daemon-reload` + `reset-failed` + `restart` |
| `Access denied ... (using password: NO)` | 同上（密码为空），或账号不存在 | 补 `-config`；建 `zoek` 账号并写进 config.yaml |
| `Unknown database 'zoek'` | 没建库 | `CREATE DATABASE zoek ...`（AutoMigrate 会自动建表） |
| Caddy 502 | 后端没起来 / 端口不符 | `ss -lnt` 看实际监听端口 |
| **ESA 回源 525** | 源站拿不出该 SNI 的证书 | 站点块加 `tls internal`（见上一节） |
| ACME 报 `context deadline exceeded` | 到 Let's Encrypt / Cloudflare 出网被挡 | 不要用 LE，改 `tls internal` |
| ACME 报 `externalAccountRequired` | ZeroSSL 强制 EAB，Caddy 没带 | 自取 EAB 或改 `tls internal` |
| ACME 报 `caddy_legacy_user_removed` | ZeroSSL 内置旧账号已停用 | 同上 |
| `unknown subdirective: email` | Caddy 2.6.2 站点级 `tls` 不支持 `email` | `email` 要写在全局块里 |
| 证书签发失败 | 80/443 未放行或域名未解析 | 云安全组 + ESA 回源放行，DNS 记录指向源站 |
| 头像上传 500 | `User=zoek` 对 `/opt/zoek/uploads` 无写权限 | `sudo chown -R zoek:zoek /opt/zoek/uploads` |
| `systemctl restart` 报 `start-limit-hit` | 崩溃循环触发 StartLimitBurst | `systemctl reset-failed zoek-api` 后再 start |

## 数据库迁移

表结构由程序 AutoMigrate 维护（含历史迁移 006 等），新库首启自动建表；无独立迁移脚本需要手动执行。

`backend/migrations/*.sql` 只是**变更留档**，SQL 不会被程序执行。其中纯数据订正类的脚本
（如 `008_game_name_date_only.sql`）需要时在服务器上手动跑一次，文件头有说明。

## 台码（小程序码）打开哪个版本：`env_version`

`GET /api/v1/games/:id/qrcode` 支持可选查询参数 `env_version`，取值 `release` / `trial` / `develop`
（白名单校验，非法值返回 400 `INVALID_INPUT`），直接透传给微信 `getwxacodeunlimit`。

**微信默认是 `release`（正式版）** —— 不显式指定的话，在体验版里扫出来的台码会跳到**正式版**，
那边连的是正式域名/正式库，联调时很容易看半天才发现走错环境。所以小程序端
（`pages/room/room.js: loadQRCode`）会带上自己的
`wx.getAccountInfoSync().miniProgram.envVersion`，后端按它生成对应版本的码：

| 小程序运行环境 | `envVersion` | 台码打开 |
|---|---|---|
| 开发者工具 / 开发版 | `develop` | 开发版 |
| 体验版 | `trial` | 体验版 |
| 正式版 | `release` | 正式版 |

注意：`develop` / `trial` 的码**只有该小程序的开发者、体验成员扫了才生效**，
且需要小程序管理后台里确实存在对应版本（已上传代码 / 已设为体验版）。
后端还固定带了 `check_path: false`，这样 `pages/join/join` 在尚未发布时也能出码。

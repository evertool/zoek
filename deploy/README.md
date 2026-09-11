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
- `Caddyfile` — Caddy 主配置（仅全局项 + `import conf.d/*.caddy`；zola 已装主文件时**不要覆盖**）
- `caddy/conf.d/zoek.caddy` — zoek 站点：`/api/*`、`/uploads/*`、`/health` 反代到 `127.0.0.1:8090`

## 服务器首次准备

1. 建目录与配置：
   ```bash
   sudo mkdir -p /opt/zoek/uploads/avatars
   sudo cp backend/config.yaml /opt/zoek/config.yaml   # 改 server.port=8090、数据库、jwt.secret
   ```
2. 建库：
   ```sql
   CREATE DATABASE zoek DEFAULT CHARACTER SET utf8mb4;
   ```
3. Caddy 站点（zola 已装主 Caddyfile 时只拷站点文件）：
   ```bash
   sudo cp deploy/caddy/conf.d/zoek.caddy /etc/caddy/conf.d/
   sudo caddy validate --config /etc/caddy/Caddyfile && sudo systemctl reload caddy
   ```
4. 微信公众平台：把该域名加进小程序 request 合法域名。

## 发版

```bash
cp deploy/release.env.example deploy/release.env   # 填 SSH_HOST 等
./scripts/release.sh backend                       # 构建 linux 二进制 → 上传 → 重启 → 健康检查
./scripts/release.sh --build-only backend          # 只构建
```

发版脚本会自动：上传二进制（tmp + mv 原子替换）、确保 systemd unit 存在（无则自动安装）、
重启 `zoek-api`、轮询 `http://127.0.0.1:8090/health` 健康检查。

## 数据库迁移

表结构由程序 AutoMigrate 维护（含历史迁移 006 等），新库首启自动建表；无独立迁移脚本需要手动执行。

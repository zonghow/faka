# GPT PLUS 提卡网（Go + React）

- 后端：Go + Gin + GORM + SQLite
- 前端：React + TypeScript + Tailwind CSS
- 部署：Docker Compose + Caddy（自动 HTTPS）

## 目录

- `server/` Go API
- `web/` React 前端
- `deploy/` Caddy 配置
- `data/` SQLite 数据（运行时）
- `storage/` 上传/下载文件（运行时）

## 本地开发

### 后端

```bash
export PATH=/usr/local/go/bin:$PATH
export TIKAWANG_SESSION_SECRET=local-dev-secret-change-me
export TIKAWANG_AUTH_PASSWORD=Wgs0405java
export TIKAWANG_BASE_DIR=/path/to/faka
cd /path/to/faka
go -C server build -o bin/faka-server ./cmd/server
./server/bin/faka-server
```

默认监听：`http://0.0.0.0:18743`

### 前端

```bash
cd web
npm install
npm run build   # 产物由 Go 托管 web/dist
# 或开发模式：
npm run dev     # http://127.0.0.1:5173
```

## Docker Compose（生产）

服务器已有 Nginx + Certbot 时（推荐）：

```bash
cp .env.example .env
# 编辑 .env：强随机密码与 session 密钥
docker compose up -d --build
# 安装站点配置并申请证书
bash deploy/deploy.sh
```

- 应用仅监听 `127.0.0.1:18743`
- 入口：Nginx `80/443` + Certbot 证书

## 环境变量

| 变量 | 说明 | 默认 |
|------|------|------|
| `TIKAWANG_AUTH_PASSWORD` | 后台密码 | 开发默认，生产必改 |
| `TIKAWANG_SESSION_SECRET` | Session 签名密钥 | 开发默认，生产必改 |
| `TIKAWANG_ADDR` | 监听地址 | `0.0.0.0:18743` |
| `TIKAWANG_BASE_DIR` | 数据根目录 | `.` |
| `TIKAWANG_DATABASE_URL` | SQLite DSN | `sqlite:///.../data/tikawang.db` |
| `TIKAWANG_STORAGE_DIR` | 文件存储 | `.../storage` |
| `TIKAWANG_STATIC_DIR` | 前端静态目录 | `.../web/dist` |
| `DOMAIN` | Caddy 域名 | `key.whistlelads.com` |

## 主要接口

- `GET /api/inventory`
- `POST /api/redeem` form: `card_code`, `output_format=cpa|sub`
- `POST /api/auth/login` `{ "password": "..." }`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `GET /api/admin/dashboard`
- `GET/POST/DELETE /api/admin/spaces`
- `GET/POST /api/admin/cards` + status/download/redemptions
- `GET/POST /api/admin/files` + upload/status/download
- `POST /api/admin/clear`

## 测试

```bash
# 单元测试
go -C server test ./...

# 接口冒烟（需服务已启动）
TIKAWANG_AUTH_PASSWORD=... bash scripts/api_smoke_test.sh http://127.0.0.1:18743
```

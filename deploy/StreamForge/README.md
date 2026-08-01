# StreamForge 一键部署包

在 Linux (amd64) 服务器上部署 StreamForge：前端 + user-service (Java) + media-service (Go) + LiveKit。
MySQL、Redis 使用云上服务，不在此部署。

## 目录结构

```
StreamForge/
├── deploy.sh                # 服务器上执行的一键部署脚本
├── build.sh                 # 开发机上执行的构建脚本
├── docker-compose.yml       # 四个容器的编排
├── .env.example             # 环境变量模板（复制为 .env 后填写）
├── user-service/
│   ├── Dockerfile           # 只 COPY jar，几十 MB
│   └── user-service.jar     # (build.sh 生成)
├── media-service/
│   ├── Dockerfile           # 只 COPY 二进制
│   └── media-service        # (build.sh 生成，linux/amd64)
├── frontend/
│   ├── Dockerfile           # nginx + dist
│   ├── nginx.conf           # 反向代理 /api/users → user-service; /api/rooms 和 /ws/rooms → media-service
│   └── dist/                # (build.sh 生成，vite build 产物)
└── livekit/
    └── livekit.yaml         # LiveKit 配置
```

## 使用步骤

### 一、开发机（Windows / Linux / macOS）

在项目根目录（`StreamForge/`）执行：

**Windows PowerShell**：
```powershell
powershell -ExecutionPolicy Bypass -File deploy\StreamForge\build.ps1
```

`build.ps1` 默认把 `VITE_USER_API_BASE_URL` 和 `VITE_MEDIA_API_BASE_URL` 设置为 `/`，
使生产前端通过同源 Nginx 反向代理访问 `/api/users/*` 和 `/api/rooms*`，不会把
`localhost:8081` 或 `localhost:8080` 写入浏览器静态包。需要自定义独立 API 域名时，
可在运行脚本前显式设置这两个环境变量。


**Linux / macOS / Git Bash**：
```bash
bash deploy/StreamForge/build.sh
```

需要本机已装（并在 PATH 中）：
- **JDK 17+**（user-service 使用了 Java 17 `record`，低于 17 会报 `class, interface, or enum expected`）
- Go 1.21+
- Node 22 或 24 + npm

> ⚠️ 在 WSL 里跑 `build.sh` 用的是 WSL 内的 java，不是 Windows 侧的 JDK。若 WSL 没装 JDK 17：
> `sudo apt install openjdk-17-jdk` 后再跑；或者直接改用 Windows 侧 PowerShell 的 `build.ps1`。

产物会被复制到 `deploy/StreamForge/{user-service,media-service,frontend}/` 下。

打包上传：
```bash
cd deploy
tar --exclude='StreamForge/.env' -czf StreamForge.tgz StreamForge
scp StreamForge.tgz user@server:/opt/
```

### 二、服务器（Linux amd64）

需要装：
- Docker Engine 20.10+
- Docker Compose v2（`docker compose ...`）

```bash
cd /opt && tar -xzf StreamForge.tgz && cd StreamForge
cp .env.example .env
vim .env          # 按注释填写 HTTPS 域名、回源端口、云上 MySQL/Redis 连接、LiveKit 密钥
bash deploy.sh
```

默认按 1Panel/OpenResty 部署：

- `meet.beichuang.art` 反向代理到 `http://服务器内网IP:8088`；
- `livekit.beichuang.art` 反向代理到 `http://服务器内网IP:7880`，并启用 WebSocket；
- OpenResty 占用公网 `80/443`，前端容器不再占用这两个端口。

### 三、放通防火墙

| 端口 | 协议 | 用途 |
|---|---|---|
| 80 / 443 | TCP | 1Panel OpenResty：HTTP 验证、HTTPS 与 WSS |
| `FRONTEND_PORT`（默认 8088） | TCP | StreamForge 前端回源端口；不向公网开放 |
| 7880 | TCP | LiveKit 信令回源端口；不向公网开放 |
| 7881 | TCP | LiveKit TURN over TCP |
| 30000-30020 | UDP | LiveKit WebRTC 媒体 |

## 关键设计

- **浏览器 → 1Panel OpenResty（443） → 前端 Nginx（8088） → user/media**：所有 API/WS 走同源的 `/api/users/*`、`/api/rooms*`、`/ws/rooms/*`，避免跨域，也不用把后端端口暴露到公网。
- **LiveKit 用 host 网络**：WebRTC 需要 UDP 端口范围直连，不能走 Docker bridge。
- **APP_DOMAIN / LIVEKIT_DOMAIN**：分别配置应用域名和 LiveKit 信令域名；media-service 返回 `wss://` 地址给前端。
- **MYSQL / REDIS**：使用云服务，直接在 `.env` 里填 host/port/账密。
- **构建与运行分离**：源码在开发机构建成产物，服务器上镜像里只有 jar / 二进制 / 静态资源，构建速度快、镜像小、服务器不需要 JDK/Node/Go。

## 常用运维命令

```bash
docker compose ps                       # 查看运行状态
docker compose logs -f media-service    # 查看某个服务日志
docker compose restart media-service    # 重启某个服务
docker compose down                     # 停止全部
docker compose up -d --build            # 重新构建并启动
```

## 升级

有新版本时，在开发机重跑 `build.sh` 得到新产物，重新打包上传覆盖，然后：

```bash
bash deploy.sh
```

`deploy.sh` 会调用 `docker compose build` 让 Docker 检测到新产物并重建镜像。

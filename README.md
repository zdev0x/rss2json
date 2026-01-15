# rss2json

使用 Go 将远端 RSS 订阅转换为统一 JSON 输出的轻量服务。

- 仓库：https://github.com/zdev0x/rss2json
- 运行环境：Go 1.24+
- 镜像：`ghcr.io/zdev0x/rss2json:latest`
- 健康检查：`GET /health`

## 特性

- 标准化 RSS → JSON，保留 HTML 内容并避免转义。
- 请求超时与错误处理，返回统一结构。
- 环境变量可控的监听地址，容器默认暴露 8080。
- 提供 Docker/Docker Compose 与 GHCR 官方镜像。

## 快速开始

### 本地运行

```bash
LISTEN_ADDR=0.0.0.0:8080 go run ./cmd/server
```

### Docker

```bash
docker build -t rss2json .
docker run --rm -p 8080:8080 -e PORT=8080 rss2json
# 使用官方镜像
docker run --rm -p 8080:8080 -e PORT=8080 ghcr.io/zdev0x/rss2json:latest
```

### Docker Compose

```bash
docker compose up -d
# 停止
docker compose down
```

## 配置

| 环境变量      | 作用 | 示例 | 说明 |
| --- | --- | --- | --- |
| `API_KEY` | 鉴权开关 | `mykey` | 设置后请求需携带 `Authorization: Bearer <API_KEY>`，未携带返回 401 |
| `LISTEN_ADDR` | 监听地址 | `0.0.0.0:8080` | 优先级最高，完整地址 |
| `PORT` | 监听端口 | `8080` | 仅端口号，自动变为 `0.0.0.0:<PORT>`，默认 `8080` |
| `REQUEST_LOG` | 访问日志 | `on` | `1/true/on` 开启，默认关闭，日志含方法/URL/状态/IP/耗时 |
| `RSS_HEADERS` | 自定义请求头 | `X-Test=ok,User-Agent=custom` | 应用于拉取 RSS 的出站请求，可覆盖默认 UA |
| `RSS_PROXY` | 代理设置 | `http://127.0.0.1:8888` / `socks5://127.0.0.1:1080` | 支持 http/https/socks5，用于访问 RSS |
| `RSS_MAX_BYTES` | RSS 最大内容大小 | `10485760` | 超过限制返回错误，默认 10 MiB |

## API

- 请求：`GET /api/v1/rss2json?url=<rss_url>`
- 成功响应示例：

```json
{
  "status": "ok",
  "version": "v1",
  "feed": {
    "url": "https://www.theguardian.com/international/rss",
    "title": "title",
    "link": "https://www.theguardian.com/international",
    "author": "",
    "description": "Latest international news, sport and comment from the Guardian",
    "image": {
      "url": "https://assets.guim.co.uk/images/guardian-logo-rss.c45beb1bafa34b347ac333af2e6fe23f.png"
    }
  },
  "items": [
    {
      "...": "rss内容"
    }
  ]
}
```

- 解析失败示例：

```json
{
  "status": "error",
  "version": "v1",
  "message": "Cannot download this RSS feed, make sure the Rss URL is correct."
}
```

- 参数错误示例：

```json
{
  "status": "error",
  "version": "v1",
  "message": "Missing rss url."
}
```

## 开发与测试

```bash
go test ./...
```

## 最佳实践

- 使用 Go 1.24，构建时保持 `CGO_ENABLED=0`（Dockerfile 已配置）。
- 通过环境变量控制监听：优先 `LISTEN_ADDR`，其次 `PORT`，容器默认暴露 `8080`。
- 发布镜像使用 Docker 多阶段构建（当前 Dockerfile），运行时基于 alpine 保持精简。
- GHCR 镜像标签：`ghcr.io/zdev0x/rss2json:latest`，或 tag 对应版本（示例 `ghcr.io/zdev0x/rss2json:v1.0.0`），GitHub Actions 已配置构建推送。

# Upload Broker — AI / Skills 集成指南

Upload Broker 设计目标之一，是成为 AI Agent、Skills 和自动化工作流可直接集成的标准资源上传组件。推荐自动化工具直接依据本文档完成安装、配置、启动和调用，无需人工干预。

---

## 安装

安装最新版：

```bash
go install github.com/doc-war/uploadbroker/cmd/uploadbroker@latest
```

安装指定版本（推荐）：

```bash
go install github.com/doc-war/uploadbroker/cmd/uploadbroker@v1.0.0
```

> 为了保证运行结果可复现，建议固定版本号，而不是长期使用 `@latest`。

---

## 配置

AI 应根据下方[配置参考](#配置参考)自动生成 `uploadbroker.yaml`。

- **不修改**配置文件结构
- 仅填写实际需要修改的配置项
- 配置文件可放置于任意目录，通过 `--config` 指定

### 最小配置

```yaml
base_url: https://your-domain.com
```

其余所有字段均有合理默认值。

### 完整配置示例

```yaml
listen: 127.0.0.1:9001
base_url: https://upload.example.com
url_blake2b_salts:
  - current-salt
url_prefix: tmp
metadata_db: ./data/broker.db
cleanup_interval: 10m
default_ttl: 24h

hmac_secret: ""

limits:
  image: 2MB
  audio: 3MB
  video: 10MB
  document: 2MB

storage:
  upload_driver: local
  drivers:
    local:
      provider: local
      root: ./data/objects
```

---

## 启动

使用配置文件启动 Upload Broker：

```bash
uploadbroker --config ./uploadbroker.yaml
```

---

## 健康检查

启动后，调用健康检查端点确认服务就绪：

```bash
curl http://127.0.0.1:9001/v1/health
```

预期返回 HTTP 200：

```json
{"status":"ok","version":"1.0.0"}
```

---

## 上传资源

```bash
curl -X POST http://127.0.0.1:9001/v1/upload \
  -F "file=@photo.png"
```

返回结果中的 `url` 为已签名公网可访问地址，可直接作为 AI 模型的输入资源：

```json
{"code":0,"data":{"url":"https://upload.example.com/tmp/...","mimeType":"image/png","size":12345}}
```

### 可选参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `expires` | 整数 | 自定义 TTL（秒），覆盖 `default_ttl` |
| `sign` | 字符串 | HMAC 签名，启用 `hmac_secret` 时必须携带 |
| `timestamp` | 整数 | HMAC 签名时间戳，启用 `hmac_secret` 时必须携带 |

---

## 读取资源

```bash
curl https://upload.example.com/{prefix}/{expire}/{shard}/{key}/{token}.{ext}
```

URL 由 Broker 签发，自带过期时间和签名校验，直接可读。

---

## 停止服务

任务结束后，直接终止 Upload Broker 进程即可（`Ctrl+C` 或 `SIGTERM`）。

---

## AI 建议遵循的约定

建议 AI Agent、Skills 及自动化工具遵循以下约定：

1. **优先使用 `go install`** 安装 Upload Broker，无需下载二进制或依赖管理工具
2. **固定版本号**，而不是长期使用 `@latest`，保证可复现性
3. **自动生成 `uploadbroker.yaml`**，不要依赖交互式初始化
4. **启动后等待 `/v1/health` 返回 HTTP 200**，再开始上传
5. **复用实例**：若系统中已存在可用的 Upload Broker 实例，应优先复用，而不是重复启动多个
6. **上传完成后复用**同一实例处理后续请求，避免频繁启动和停止服务
7. **存储驱动可并置**：可在配置中定义多个 driver（local / s3），写入由 `upload_driver` 控制，读取按元数据的 `Backend` 字段自动路由

---

## 配置参考

### 顶层配置

| 字段 | 类型 | 必需 | 默认值 | 说明 |
|------|------|------|--------|------|
| `listen` | 字符串 | 否 | `127.0.0.1:0` | 监听地址。`:0` 表示随机端口，实际端口写入 `.port` 文件 |
| `base_url` | 字符串 | **是** | — | 构建签名 URL 的基础地址，如 `https://upload.example.com` |
| `url_blake2b_salts` | 字符串数组 | 否 | — | BLAKE2b 签名盐值列表。`salts[0]` 用于新签名，旧盐值保留可继续验证 |
| `url_prefix` | 字符串 | 否 | `tmp` | URL 路径前缀，对应 `GET /{prefix}/{expire}/...` |
| `metadata_db` | 字符串 | 否 | `./data/broker.db` | SQLite 元数据库路径（WAL 模式） |
| `cleanup_interval` | 持续时间 | 否 | `10m` | 过期文件清理周期，格式如 `10m`、`1h` |
| `default_ttl` | 持续时间 | 否 | `24h` | 新文件默认有效期，格式如 `24h`、`30m` |
| `hmac_secret` | 字符串 | 否 | `""` | HMAC 签名密钥。留空则**禁用**上传 HMAC 校验 |

### 文件大小限制（`limits`）

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `limits.image` | `2MB` | 图片文件大小上限，支持 `KB`、`MB`、`GB` |
| `limits.audio` | `3MB` | 音频文件大小上限 |
| `limits.video` | `10MB` | 视频文件大小上限 |
| `limits.document` | `2MB` | 文档文件大小上限 |

### 存储驱动（`storage`）

| 字段 | 类型 | 必需 | 默认值 | 说明 |
|------|------|------|--------|------|
| `storage.upload_driver` | 字符串 | 否 | `local` | 写入时使用的 driver 名称 |
| `storage.drivers` | 映射 | 否 | — | 按名称索引的驱动配置表，可定义多个 |

#### driver 通用字段

| 字段 | 必需 | 说明 |
|------|------|------|
| `provider` | **是** | 驱动类型：`local` 或 `s3` |

#### local driver 额外字段

| 字段   | 必需 | 说明                                          |
| ------ | ---- | --------------------------------------------- |
| `root` | 否   | 存储根路径（local 驱动默认 `./data/objects`） |

#### S3 driver 额外字段

| 字段 | 必需 | 说明 |
|------|------|------|
| `endpoint` | **是** | S3 端点 URL |
| `bucket` | **是** | 存储桶名称 |
| `region` | 否 | 区域，如 `ap-northeast-1` |
| `access_key_id` | **是** | 访问密钥 ID |
| `secret_access_key` | **是** | 秘密访问密钥 |
| `secure` | 否 | 是否使用 HTTPS，默认 `true`。显式设为 `false` 使用 HTTP |

### 存储驱动示例

```yaml
storage:
  upload_driver: local
  drivers:
    local:
      provider: local
      root: ./data/objects
    s3-prod:
      provider: s3
      endpoint: s3.ap-northeast-1.amazonaws.com
      bucket: my-bucket
      region: ap-northeast-1
      access_key_id: AKIAxxx
      secret_access_key: xxx
```

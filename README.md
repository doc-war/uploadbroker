# UploadBroker

轻量级资源代理网关 —— 专为 AI 推理平台的临时文件上传、读取与生命周期管理设计。不重新发明存储，而是做现有存储（Local / S3 / R2 / OSS）的统一安全外壳。

它就是临时存储界的 Redis。

```text
Client →  uploadbroker → Local / S3 / OSS / R2
```

或

```text
Client → Platform (认证/授权) → uploadbroker → Local / S3 / OSS / R2
```

## 快速开始

```bash
# 编译
go build -o uploadbroker

# 运行（使用默认 config）
./uploadbroker

# 指定配置文件
./uploadbroker --config=/path/to/config.yaml
```

## API

| 端点 | 方法 | 说明 |
|---|---|---|
| `/v1/upload` | POST | multipart 上传文件 |
| `/v1/health` | GET | 健康检查 |
| `/tmp/{expire}/{shard}/{key}/{token}.{ext}` | GET | 读取资源 |

### 上传

```
POST /v1/upload
Content-Type: multipart/form-data

file: <binary>
expires: 1-24 (可选，小时)
sign: <hmac> (可选，需配置 hmac_secret)
timestamp: <unix> (可选)
```

### 读取

资源 URL 由 Broker 签发，格式：

```
/tmp/{expire}/{shard}/{key}/{token}.{ext}
```

- expire — Unix 秒级过期时间
- shard — key 前 2 位（目录分片）
- key — BLAKE2b-256 内容哈希
- token — URL 签名
- ext — MIME 推导扩展名

## 支持资源类型

| 类型 | MIME | 默认限制 |
|---|---|---|
| 图片 | png / jpeg / webp | 2 MB |
| 音频 | mp3 / wav / m4a / aac | 3 MB |
| 视频 | mp4 / webm | 10 MB |
| 文档 | txt / pdf | 2 MB |

## 配置

```yaml
listen: 127.0.0.1:9001                # 监听地址，端口传0表示自动选择空闲端口
base_url: https://upload.example.com  # 注入对外资源 URL 前缀（必填）（部署环境依赖）
url_blake2b_salts:                    # URL 签名盐值（最多 2 个，省略则使用空盐）
  - current-salt
url_prefix: tmp                       # URL 路径前缀（默认 tmp）
metadata_db: ./data/broker.db         # SQLite 数据库路径（默认 ./data/broker.db）
cleanup_interval: 10m                 # 过期资源清理间隔（默认 10m）
default_ttl: 24h                      # 默认 TTL（默认 24h）
hmac_secret: ""                       # 可选，上传 HMAC 签名密钥

limits:
  image: 2MB                          # 图片最大体积（单位 MB，仅支持整数，默认 2MB）
  audio: 3MB                          # 音频（单位 MB，默认 3MB）
  video: 10MB                         # 视频（单位 MB，默认 10MB）
  document: 2MB                       # 文档（单位 MB，默认 2MB）

storage:
  upload_driver: local                # 写入使用的驱动名称
  drivers:
    local:
      provider: local                 # 本地文件系统
      root: ./data/objects            # 存储根目录
    aws:
      provider: s3                    # S3 兼容服务（AWS/OSS/R2/MinIO）
      endpoint: s3.ap-northeast-1.amazonaws.com
      bucket: my-bucket
      region: ap-northeast-1
      access_key_id: xxx
      secret_access_key: xxx
      # secure: true                  # HTTPS（默认 true，false=HTTP）
```

### 存储驱动

所有驱动统一通过 `provider` 区分类型，driver name 自由命名，可配任意多个。

| 参数 | provider | 说明 |
|---|---|---|
| `provider: local` | local | 本地文件系统 |
| `root` | local | 存储根目录 |
| `provider: s3` | s3 | 任何 S3 兼容服务 |
| `endpoint` | s3 | 服务地址（如 s3.amazonaws.com） |
| `bucket` | s3 | 存储桶 |
| `region` | s3 | 区域（AWS 必填，MinIO 可空） |
| `access_key_id` | s3 | 访问密钥 |
| `secret_access_key` | s3 | 访问密钥 |
| `secure` | s3 | HTTPS（默认 true，`false`=HTTP） |

写入由 `upload_driver` 指定，读取按 `rec.Backend` 自动路由。

支持列表：

| 服务 | endpoint 示例 |
|---|---|
| AWS S3 | `s3.{region}.amazonaws.com` |
| 阿里云 OSS | `oss-{region}.aliyuncs.com` |
| Cloudflare R2 | `{account}.r2.cloudflarestorage.com` |
| 腾讯 COS | `cos.{region}.myqcloud.com` |
| MinIO | `192.168.1.100:9000` |

## 安全

- **魔数检测** — `http.DetectContentType` 读取文件头前 512 字节确认真实格式
- **扩展名校验** — 文件名扩展名必须与魔数检测结果一致，防伪装上传
- **HMAC 上传校验** — 可选，配置 `hmac_secret` 后需携带签名
- **URL 签名** — BLAKE2b-256 签名，防 URL 篡改
- **自动过期** — TTL 到期自动清理
- **路径穿越防护** — Local Driver 限制 `..` 访问

## 错误码

| Code | 说明 |
|---|---|
| 0 | Success |
| 40001 | Missing File |
| 40002 | Empty File |
| 40003 | Unsupported MIME |
| 40004 | File Too Large |
| 40005 | Invalid TTL |
| 40006 | MIME Mismatch（扩展名与魔数不一致）|
| 40401 | Invalid URL |
| 40402 | Resource Not Found |
| 50000 | Internal Error |
| 50001 | Storage Error |

## 技术栈

- Go 1.25 单二进制部署
- SQLite（WAL + busy_timeout）
- 存储驱动插件化（`map[string]storage.Storage`）
- Local Driver：文件系统日期分片
- S3 Driver：minio-go/v7，兼容全部 S3 服务
- BLAKE2b-256 内容寻址 + URL 签名
- 无外部运行时依赖

## 目录结构

```
├── main.go                    # 入口：配置加载、驱动初始化、启动
├── uploadbroker.yaml          # 默认配置
├── internal/
│   ├── api/                   # HTTP handlers（upload / read / health）
│   ├── config/                # YAML 配置解析
│   ├── hash/                  # BLAKE2b + HMAC + URL 签名
│   ├── metadata/              # SQLite 元数据存储
│   ├── mime/                  # MIME 检测 + 扩展名校验
│   └── storage/               # Storage 接口 + Local/S3 驱动
├── docs/                      # 设计文档
└── data/                      # 运行时数据
```

## License

MIT

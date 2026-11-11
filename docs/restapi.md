# REST API

`/api/v1` 是给脚本、CI 和自动化平台用的管理接口：JSON 请求体、标准 HTTP 方法、明确的错误码。后台页面能做的事（客户端、隧道、域名）都能通过它完成。

老的表单式接口（`/index/add`、`/client/list`，见 [web api](api.md)）仍然可用，但它需要自己算 `auth_key` 与时间戳，且参数是表单。新接入建议直接用本文的 REST 接口。

## 开启与配置

API 默认**关闭**，在 `nps.conf` 里配置 `api_key` 后生效，改完重启 nps。

```ini
#启用 API，值就是调用方要带的密钥，建议 32 位以上随机串
api_key=7f3c1a9e5b2d48c6a0e9f4b7d1c2e8a5
#只允许这些来源调用，格式同 allow_ip（单 IP / CIDR / 前缀通配）
api_allow_ip=203.0.113.10,10.0.0.0/8
#每个来源 IP 每分钟的请求上限，0 表示不限制
api_rate_limit=300
#只读模式：拒绝所有写操作，只允许查询
api_read_only=false
```

`api_key` 为空时所有 `/api/v1` 请求返回 `503 api_disabled`，探活接口 `/api/v1/health` 除外。

## 认证

三种方式任选其一：

```bash
#1. Bearer（推荐）
curl -H "Authorization: Bearer <api_key>" http://127.0.0.1:8080/api/v1/tunnels

#2. 自定义头
curl -H "X-Api-Key: <api_key>" http://127.0.0.1:8080/api/v1/tunnels

#3. 后台 web 账号（Basic，便于调试，等价于登录后台的权限）
curl -u admin:123 http://127.0.0.1:8080/api/v1/tunnels
```

认证失败返回 `401` 并带 `WWW-Authenticate`，来源不在 `api_allow_ip` 里返回 `403`。

## 通用约定

- 请求体必须是 JSON，字段名拼错会直接 `400`（不会静默忽略）。
- 请求体上限 1MB。
- 响应固定带 `X-Request-Id`，同一个 id 也出现在响应体的 `request_id` 里，便于对日志。

成功：

```json
{"ok": true, "data": { ... }, "request_id": "a1b2c3d4"}
```

失败：

```json
{"ok": false, "error": {"code": "invalid_argument", "message": "port must be between 1 and 65535"}, "request_id": "a1b2c3d4"}
```

| HTTP | code | 含义 |
| --- | --- | --- |
| 400 | `invalid_argument` | 参数校验失败 |
| 401 | `unauthorized` | 缺少或错误的凭据 |
| 403 | `forbidden` | 来源 IP 不在白名单 |
| 403 | `read_only` | 只读模式拒绝了写操作 |
| 404 | `not_found` | 资源不存在 |
| 409 | `conflict` | 端口已被占用、重名、vkey 重复、隧道已在运行/未运行 |
| 413 | `payload_too_large` | 请求体超过 1MB |
| 429 | `rate_limited` | 触发 `api_rate_limit` |
| 503 | `api_disabled` | 没有配置 `api_key` |

## 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/health` | 探活，**无需认证** |
| GET | `/api/v1/status` | 仪表盘统计（与后台首页同一份数据） |
| GET | `/api/v1/tunnels` | 隧道列表，支持 `mode` `client_id` `search` `state` `page` `limit` |
| POST | `/api/v1/tunnels` | 创建隧道（默认立即启动） |
| GET | `/api/v1/tunnels/{id}` | 隧道详情，附 `warnings` 配置提醒 |
| PATCH | `/api/v1/tunnels/{id}` | 局部更新，只改传了的字段（`PUT` 等价） |
| DELETE | `/api/v1/tunnels/{id}` | 停止并删除 |
| POST | `/api/v1/tunnels/{id}/start` | 启动 |
| POST | `/api/v1/tunnels/{id}/stop` | 停止（保留配置） |
| GET | `/api/v1/clients` | 客户端列表，`include_vkey=true` 返回完整 vkey |
| POST | `/api/v1/clients` | 创建客户端，响应里带一次完整 vkey |
| GET | `/api/v1/clients/{id}` | 客户端详情 |
| PATCH | `/api/v1/clients/{id}` | 改备注、状态、限速、限额、vkey |
| DELETE | `/api/v1/clients/{id}` | 删除，存在隧道/域名时需 `cascade=true` |
| GET | `/api/v1/hosts` | 域名隧道列表 |
| POST | `/api/v1/hosts` | 新增域名隧道 |
| GET | `/api/v1/hosts/{id}` | 域名隧道详情 |
| DELETE | `/api/v1/hosts/{id}` | 删除域名隧道 |

列表接口统一返回：

```json
{"ok": true, "data": {"items": [], "total": 0, "page": 1, "limit": 20}}
```

`limit` 默认 20，最大 200。`state` 取 `all` / `running` / `stopped`。

## 隧道字段

创建请求体（`POST /api/v1/tunnels`）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `client_id` | int | 必填，隧道归属的客户端 |
| `mode` | string | `tcp` `udp` `socks5` `httpProxy` `ftp` `secret` `p2p` `file`，默认 `tcp` |
| `port` | int | 服务端公网端口，`secret`/`p2p` 忽略 |
| `target` | string | 内网目标 `host:port`，`socks5`/`file` 可空 |
| `remark` | string | 备注 |
| `status` | bool | 创建后是否立即启动，默认 `true` |
| `local_proxy` | bool | 目标是否由客户端本地解析（目标在客户端那侧） |
| `server_ip` | string | 监听地址，空表示所有网卡 |
| `ip_acl` | string | 允许访问的来源，逗号分隔；支持单 IP、CIDR、`192.168.1.*`；空则用全局 `allow_ip` |
| `ip_deny` | string | 拒绝的来源，优先级高于 `ip_acl` |
| `conn_rate` | int | 每个来源每秒新建连接上限，0 用全局值 |
| `auth_user` | string | TCP 隧道 Basic 认证用户名 |
| `auth_pass` | string | TCP 隧道 Basic 认证密码 |
| `tls_enable` | bool | TCP 隧道用 TLS 承载，访问者必须用 TLS 连接 |
| `tls_cert_file` | string | 隧道专用证书，需与 `tls_key_file` 成对出现；空则用 `bridge_cert_file` 或内建自签证书 |
| `tls_key_file` | string | 隧道专用私钥 |
| `local_path` | string | `file` 模式的本地目录 |
| `strip_pre` | string | `file` 模式的 URL 前缀 |
| `password` | string | `secret`/`p2p` 的密钥，其它模式忽略 |

`PATCH` 用同样的字段名，但都是可选的；未出现的字段保持原值。

返回体里**不会**出现 `password` 与 `auth_pass`，`auth_enabled` 只表示是否开启了认证。

几个约束：

- Basic 认证与 TLS 只对 `tcp` 生效，其它模式传了会 `400`。
- `ip_acl` / `ip_deny` 写错格式会 `400`，不会静默变成"不限制"。
- 同一模式族内端口不能重复（`tcp`/`udp` 可以共用同一个端口号）；端口被别的进程占用返回 `409`。
- 改 `port` / `mode` / `tls_*` / `local_path` 会重建监听，已有连接会断开；改 `ip_acl` / `auth_*` 不会断连。

## 示例：新建一个 3389 隧道

```bash
API=http://127.0.0.1:8080/api/v1
KEY=7f3c1a9e5b2d48c6a0e9f4b7d1c2e8a5

#1. 建客户端，拿到 vkey（只在这里返回一次完整值）
curl -s -X POST "$API/clients" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"remark":"办公网","max_tunnel_num":5}'
#{"ok":true,"data":{"id":2,"vkey":"febbktackhv6vweg", ...}}

#2. 建隧道：只允许公司出口 IP，开 Basic 认证 + TLS
curl -s -X POST "$API/tunnels" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{
        "client_id": 2,
        "mode": "tcp",
        "port": 33890,
        "target": "10.0.0.8:3389",
        "remark": "内网 RDP",
        "ip_acl": "203.0.113.10",
        "auth_user": "rdp",
        "auth_pass": "change-me",
        "tls_enable": true
      }'

#3. 停 / 起 / 删
curl -s -X POST "$API/tunnels/1/stop"  -H "Authorization: Bearer $KEY"
curl -s -X POST "$API/tunnels/1/start" -H "Authorization: Bearer $KEY"
curl -s -X DELETE "$API/tunnels/1"     -H "Authorization: Bearer $KEY"
```

PowerShell 里用 `Invoke-RestMethod`：

```powershell
$h = @{ Authorization = "Bearer $env:NPS_API_KEY" }
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:8080/api/v1/tunnels?state=running" -Headers $h
```

## 域名隧道

`POST /api/v1/hosts` 的字段：`client_id`、`host`（如 `app.example.com` 或 `*.example.com`）、`scheme`（`all`/`http`/`https`）、`location`、`target`、`remark`、`header`、`host_change`、`cert_file`、`key_file`、`local_proxy`。

域名隧道建好后立即对 http/https 代理入口生效，不需要启动。

## 客户端删除

```bash
#有隧道时会返回 409，避免误删
curl -X DELETE "$API/clients/2" -H "Authorization: Bearer $KEY"

#连同它名下的隧道与域名一起删
curl -X DELETE "$API/clients/2?cascade=true" -H "Authorization: Bearer $KEY"
```

## 安全建议

- `api_key` 用随机长串，和 `web_password` 分开，不要复用。
- 只在对内网或固定出口开放 API：配置 `api_allow_ip`，或干脆只监听 `127.0.0.1` 由反向代理转发。
- 自动化脚本只需要读数据时，把 `api_read_only=true` 打开。
- 公网暴露 3389/22 这类端口时，务必配 `ip_acl` + `auth_user`/`auth_pass`，必要时再开 `tls_enable`，否则扫描器会直接打到内网服务上（爆破会锁账号）。

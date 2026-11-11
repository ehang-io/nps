# nps / npc 使用教程

本文对应 `dist/` 目录下的构建产物，覆盖服务端部署、客户端接入、多服务端穿透、后台管理，以及 3389 这类高危端口的防护配置。

---

## 一、产物目录与命名

`build.dist.ps1` 会把全平台产物统一输出到仓库根下的 `dist/`，命名沿用仓库 `build.sh` 的规则：

```
dist/
├── BUILD-INFO.txt                  构建信息（版本、工具链、每个包的大小与状态）
├── SHA256SUMS.txt                  所有 tar.gz 的 SHA256 校验值
├── linux_amd64_server.tar.gz       服务端
├── linux_amd64_client.tar.gz       客户端
├── linux_arm_v7_server.tar.gz      ARMv7（树莓派等）
├── windows_amd64_server.tar.gz
├── windows_386_client.tar.gz
├── darwin_arm64_server.tar.gz      Apple Silicon
└── ...
```

命名格式：`<os>_<arch>[_v<arm版本>]_<server|client>.tar.gz`

包内结构（与官方发布一致，解压即用）：

```
server 包: nps[.exe]  conf/{nps.conf,tasks.json,clients.json,hosts.json,server.key,server.pem}  web/views  web/static  scripts/{nps-server.sh,npc-client.sh}
client 包: npc[.exe]  conf/npc.conf  conf/multi_account.conf
```

支持的目标（17 个平台 × server/client = 34 个包）：`linux/amd64|386|arm64|mips|mipsle|mips64|mips64le`、`linux/arm` v5/v6/v7、`freebsd/amd64|386|arm`、`darwin/amd64|arm64`、`windows/amd64|386`。

校验下载完整性：

```bash
sha256sum -c SHA256SUMS.txt
```

---

## 二、重新构建

### 全量构建（推荐）

```powershell
# Windows PowerShell
powershell -ExecutionPolicy Bypass -File build.dist.ps1
```

```bash
# Linux / macOS 交叉编译同样可行（脚本是 PowerShell，pwsh 亦可）
pwsh -File build.dist.ps1 -Version 0.26.13
```

常用参数：

| 参数 | 说明 |
| --- | --- |
| `-Version 0.26.13` | 写入 `BUILD-INFO.txt` 的版本号 |
| `-OutDir dist2` | 换输出目录 |
| `-Only linux/amd64,windows/amd64` | 只构建指定平台 |
| `-ServerOnly` / `-ClientOnly` | 只出服务端 / 只出客户端 |
| `-NoArchive` | 只生成二进制与暂存目录，不打包 |
| `-CleanStage` | 保留暂存目录（不加则每次启动清空 `dist/.stage`） |

编译参数固定为 `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -extldflags -static"`，产物是静态链接，可直接丢到任意同架构系统上跑。

### 快速本地构建

```bash
make          # 等价于 go build ./cmd/nps 和 ./cmd/npc
make test     # race + 覆盖率
```

`npc_sdk`（cgo 动态库）不在 `build.dist.ps1` 范围里，需要 gcc/mingw 工具链：

```bash
CGO_ENABLED=1 go build -tags sdk -buildmode=c-shared -o npc_sdk.so cmd/npc/sdk.go
```

---

## 三、服务端部署

### Linux（systemd）

```bash
tar -xzf linux_amd64_server.tar.gz
cd ./
chmod +x nps
sudo ./nps install      # 注册服务（会拷贝到 /etc/nps，配置/data 落在 /etc/nps）
sudo nps start
sudo nps status
```

前台调试用 `./nps` 即可，日志直接打到终端；`./nps -version` 查看版本。

### Windows

解压后以管理员身份运行一次 `nps.exe install` 注册服务，之后 `nps.exe start` / `stop` / `restart` / `uninstall`。直接双击 `nps.exe` 是前台运行。

### Docker

```bash
docker run -d --name nps --net=host -v /opt/nps/conf:/conf ffdfgdfg/nps
```

### 必开端口（`conf/nps.conf`）

| 配置项 | 默认 | 用途 |
| --- | --- | --- |
| `bridge_port` | 8024 | 客户端连接端口（`bridge_type=tcp` 时） |
| `web_port` | 8080 | 后台管理 |
| `http_proxy_port` | 80 | HTTP 代理穿透入口 |
| `https_proxy_port` | 443 | HTTPS 代理穿透入口 |
| `p2p_port` | 6000 | P2P 模式（用 p2p 才需要） |

启用 p2p 必须同时配 `p2p_ip`（本机公网地址）和 `p2p_port`，并在防火墙放行 `p2p_port ~ p2p_port+2` 三个 UDP 端口。两项都缺时服务端不会开启 p2p，客户端会明确报 `p2p is not enabled on this server, set p2p_port (and p2p_ip) in nps.conf`；`p2p_ip` 留空时依次回退 `public_ip`、`external_ip`。

上线前至少改这三处：`public_vkey`、`web_username`、`web_password`；`runmode` 保持 `pro` 以外还要注意 `web_host` 只影响访问域名，不填也能登录。

`bridge_type` 决定客户端到服务端的控制通道：`tcp`（默认，明文）、`kcp`（UDP 加速）、`tls`（控制通道 TLS，证书取 `bridge_cert_file` / `bridge_key_file`，未配置用内建自签）。切到 `tls` 后客户端必须用 `-type=tls` / `conn_type=tls`，两端不一致会直接连不上。

---

## 四、客户端接入

### 配置文件方式

`conf/npc.conf`：

```ini
[common]
server_addr=nps.example.com:8024
conn_type=tcp
vkey=你的public_vkey
auto_reconnection=true
```

启动：

```bash
./npc -config=conf/npc.conf
./npc -config=conf/npc.conf -log=file -log_path=npc.log
```

### 命令行方式

```bash
./npc -server=nps.example.com:8024 -vkey=你的vkey -type=tcp
```

常用参数：`-type`（`tcp` / `kcp` / `tls`）、`-tls_verify`（`-type=tls` 时校验服务端证书，自签部署不要开）、`-proxy`（走 socks5 / http 代理出网）、`-log_level 0~7`、`-disconnect_timeout`。`bridge_type=tls` 的服务端必须用 `-type=tls` 连接；`conn_type=tls` 时是否校验证书也可写在 `conf/npc.conf` 的 `tls_verify`。

Windows 下服务模式同样用 `npc.exe install` / `start` / `stop`；注意服务模式读取的是安装目录下的 `conf\npc.conf`，命令行参数会被配置文件覆盖。

### 客户端里的隧道定义

`npc.conf` 中除 `[common]` 外的每个 `[section]` 就是一条隧道，`mode` 决定类型：`tcp` / `udp` / `httpProxy` / `socks5` / `secret` / `p2p` / `file`。示例见包内自带的 `conf/npc.conf`。

---

## 五、多服务端穿透

一个内网服务同时挂到多个 nps 节点，每个节点各自独立通道、独立断线重连；任一节点故障不影响其他节点上的隧道。

`conf/npc.conf`：

```ini
[common]
server_addr=nps-a.example.com:8024,nps-b.example.com:8024
conn_type=tcp
vkey=123
```

命令行：

```bash
./npc -server=10.0.0.1:8024,10.0.0.2:8024 -vkey=123 -type=tcp
```

行为说明：

- 分隔符支持逗号、分号和空白，`server_addr` 里的第一个地址仍是“主节点”（`./npc status -config=conf/npc.conf` 显示它，`npc_vkey.txt` 也只写它）。
- `[common]` 下的 `host` / 隧道 section 会在**每个**服务端各注册一份，所以两个节点上都要有对应的穿透端口或域名指向同一内网服务。
- 日志按 `[服务端地址]` 前缀区分，例如 `[nps-b.example.com:8024] Reconnecting...`。
- `secret` / `p2p` 模式的本地监听端口全局只启动一次，不会因为多服务端冲突。

验证：

```bash
# 两台 nps 的 bridge 各在 8025 / 8026
./npc -server=127.0.0.1:8025,127.0.0.1:8026 -vkey=123
# 然后在两个后台分别加一条 tcp 隧道，curl 两个公网端口应都能打通同一内网服务
```

服务端侧无需额外配置，多服务端是纯客户端能力。

---

## 六、3389 等端口的安全配置

**问题**：穿透 3389（RDP）后，任何公网 IP 只要和穿透端口建连，就会被一路转发到内网 3389。扫描器与爆破脚本会持续尝试登录，触发 Windows 账户锁定策略，正常用户被锁在门外。

**做法**：在建立隧道**之前**判断来源地址，不通过的直接 RST，扫描器连服务指纹都拿不到，爆破请求根本不会到达内网机器。

`conf/nps.conf` 全局默认：

```ini
# 只允许这些来源（单 IP / CIDR / 前缀通配），其余在建连阶段就断开
allow_ip=203.0.113.9,198.51.100.0/24,10.0.0.*
# 明确拒绝（优先级高于 allow_ip）
deny_ip=8.8.8.8

# 单来源 IP 每窗口最多新建 N 条连接，超限封禁 tunnel_ban_seconds 秒
tunnel_conn_rate=10
tunnel_conn_window=1
tunnel_ban_seconds=600
```

后台「新增/编辑隧道」页有三个同名字段（`ip_acl` / `ip_deny` / `conn_rate`），**隧道级配置优先于全局配置**，所以只给 3389 那条隧道开白名单即可，不必影响其他隧道。

推荐做法：

1. 3389 隧道填 `ip_acl=你的办公出口 IP`，`conn_rate=5`；
2. Windows 侧把 RDP 的“账户锁定阈值”设为较大值或改用“允许列表 + 强密码”；
3. 只有 `allow_ip` 命中且限速通过的来源才会建立到内网的连接，扫描器在被拒时也看不到 `220`/`RDP` 之类指纹。

被拦截的连接会以 10 秒一条摘要写 debug 日志（`log_level=7` 才可见），不会因为扫描量把日志刷爆。字段语义与实现细节见 `docs/feature.md`。

> 注意 `deny_ip` 优先于 `allow_ip`；写错了可能导致自己断连，改配置后记得 `nps restart`。

### 端口级连接策略（首字节守卫）

```ini
# nps.conf，默认 15 秒；设为 0 关闭
first_byte_timeout=15
```

命中常见端口表（3389 / 22 / 23 / 80 / 443 / 8080 / 8443 / 5900 / 5901 等）的 tcp 隧道，
会在建立隧道之前等一次客户端首包：

- 正常客户端（RDP、SSH、浏览器）毫秒级就发出首包，转发路径完全不变；
- 扫描器/指纹探测脚本连上之后通常不发数据，超过 `first_byte_timeout` 就被关闭，
  且**不会触达内网目标**，客户端连接数配额也不会被占用；
- 公网端口没命中画像表时，会回退用内网目标端口判断（公网 13389 → 内网 3389 同样生效）；
- FTP（20/21）与 MySQL（3306）是**服务端先说话**，自动跳过，不会被误杀。

Linux 上还会给隧道连接设置 `TCP_USER_TIMEOUT`（交互类 2 分钟），
RDP 客户端掉线后连接能被尽快回收，而不是挂十几分钟。详见 `docs/feature.md`「常见端口优化」。

---

## 七、FTP 隧道（被动模式 + 主动模式）

FTP 的控制通道通了只代表能登录，列目录、下载还要另开一条数据连接，而这条连接的
地址和端口是控制通道临时协商出来的。直连场景下没问题，穿透场景下必然失败：

| 模式 | 谁发起数据连接 | 报文里的地址 | 不改写的结果 |
| --- | --- | --- | --- |
| 被动 PASV / EPSV | 客户端 → 服务端 | 内网 IP + 随机端口 | 客户端连不上，卡在 `150` 之后 |
| 主动 PORT / EPRT | 服务端 → 客户端 | 客户端自己的地址 | 内网 FTP 连不到该地址 |

nps 在 `mode=ftp` 的隧道上（内网目标端口是 21 的 `tcp` 隧道默认也会自动套用）把这两类
报文改写成「服务端公网地址 + 中继端口」，数据连接进来后再经隧道送到真正的对端：

```
227 Entering Passive Mode (10,0,0,8,196,130)   内网 FTP 原始回答（156*256+130 之类的随机端口）
227 Entering Passive Mode (203,0,113,7,156,65)  nps 改写后：40001
```

### 服务端配置

```ini
# nps.conf
#对外通告的地址：留空取 external_ip，再留空取控制连接的本地地址。
#服务端在 NAT 后面时必须显式配置这一项。
ftp_public_ip=203.0.113.7
#被动模式中继端口范围。不配置也能用，但随机高位端口通常过不了安全组，
#所以要固定一段并在安全组 / 防火墙里整段放行。
ftp_pasv_ports=40000-40050
#主动模式中继端口范围，留空则复用被动范围
ftp_active_ports=40100-40150
#中继端口等不到数据连接就释放（秒）
ftp_relay_timeout=60
#auto（默认）：tcp 隧道打到 21 端口也自动改写；off：只改写 mode=ftp 的隧道
ftp_alg=auto
```

### 创建隧道

后台「穿透管理 → FTP 隧道 → 新增」，或者在一条 TCP 隧道里把内网目标写成 `10.0.0.8:21`。

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/tunnels \
  -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" \
  -d '{"client_id":2,"mode":"ftp","port":21,"target":"10.0.0.8:21"}'
```

客户端侧不需要任何额外配置，仍然是 `./npc -server=... -vkey=...`。

### 行为说明

- 每个数据连接单独占用一个中继端口，传输结束即释放；同一会话最多同时保留 8 个中继端口，
  超出时关闭最早的那个
- 被动模式的内网目标固定是「隧道目标地址 + FTP 通告的端口」，因为很多 FTP 服务的 PASV
  回答里是 `0.0.0.0` 或另一块网卡的内网地址
- 主动模式只在客户端通告的地址能从内网一侧连通时成功。客户端在 NAT 后面、通告的是私网
  地址时本来就无法使用主动模式，这是 FTP 协议本身的限制；失败只记一条 warn，不影响控制通道
- 客户端执行 `AUTH TLS`（显式 FTPS）后控制通道变加密，该会话自动停止改写并记一条日志，
  行为回到改造前。需要加密时建议用隧道级 TLS（编辑隧道勾选 `tls`）承载控制通道
- 不再需要在内网 FTP 上手工固定 PASV 端口段，也不必为每个数据端口各建一条隧道

对照排查：`nps.log` 里 `ftp passive mode: internal port 40001 -> 203.0.113.7:40010`、
`ftp active mode: 1.2.3.4:5000 -> 203.0.113.7:40100` 分别是两种模式的中继记录。

## 八、TCP 隧道的 Basic 认证与 TLS 加密

来源白名单（第六节）解决的是“谁能连”，Basic 认证 + TLS 解决的是“连上的人能不能用、链路是否明文”。两者可以叠加：白名单先挡掉扫描器，认证再挡掉拿到端口的人。

### 后台配置

「穿透管理 → TCP 隧道 → 新增/编辑」多出三个字段：

| 字段 | 说明 |
| --- | --- |
| 访问用户名 `auth_user` | Basic 认证用户名，留空表示不认证 |
| 访问密码 `auth_pass` | Basic 认证密码，建议与 TLS 同时开启 |
| TLS 加密 `tls` | 开启后该端口只接受 TLS 连接；证书取 `nps.conf` 的 `bridge_cert_file` / `bridge_key_file`，都没配则用内建自签证书 |

`conf/npc.conf` 的隧道 section 写法相同：

```ini
[tcp3389]
mode=tcp
server_port=10000
target_addr=127.0.0.1:3389
auth_user=visitor
auth_pass=change-me
tls=true
```

### 访问者怎么带凭据

认证在**建立隧道之前**完成，未通过认证的连接既拿不到内网服务的 banner，也不会被转发。支持三种握手，按访问者能力选一种：

```bash
# 1) 行协议：先认证，之后是同一条 TCP 流
openssl s_client -quiet -connect nps.example.com:10000
AUTH Basic dmlzaXRvcjpjaGFuZ2UtbWU=      # base64(visitor:change-me)

# 2) HTTP CONNECT（HTTP 代理语义，stunnel / ncat --proxy 用这个）
CONNECT 127.0.0.1:3389 HTTP/1.1
Proxy-Authorization: Basic dmlzaXRvcjpjaGFuZ2UtbWU=

# 3) 普通 HTTP 请求：Authorization: Basic ...，认证头会被剥离后再转发给内网
```

RDP 客户端本身不会发认证帧，需要一层本地包装。stunnel 的 `protocol = connect` 正好是 HTTP CONNECT 客户端：

```ini
[rdp]
client = yes
accept = 127.0.0.1:13389
connect = nps.example.com:10000
protocol = connect
protocolHost = nps.example.com:10000
protocolAuthentication = basic
protocolUsername = visitor
protocolPassword = change-me
```

> 该 stunnel 片段按官方 `protocol = connect` 语义编写，本仓库未在 stunnel 上实测；如果握手异常，先用上面的 `openssl s_client` 手工验证服务端行为。认证通过后 RDP 客户端连 `127.0.0.1:13389` 即可。

### 认证失败的表现

- HTTP 形态：`401 Unauthorized` + `WWW-Authenticate: Basic`
- 行协议：`AUTH FAILED`
- 首包既不是 HTTP 也不是认证帧：直接 RST，不回任何字节（按扫描器处理）

被拒绝的连接同样按 10 秒一条摘要写 debug 日志，不会刷屏。

### 证书

隧道没写 `tls_cert_file` / `tls_key_file` 时依次回退：`nps.conf` 的 `bridge_cert_file` / `bridge_key_file` → 内建自签证书。换正式证书：

```bash
./scripts/nps-server.sh gencert     # 生成 conf/server.pem / server.key（带 SAN）
```

`bridge_type=tls` 与隧道 TLS 共用这套证书，所以想全站上 TLS 时只维护一份即可。

---

## 九、服务端 / 客户端管理脚本

`scripts/` 下两个脚本封装了常用的启停与证书操作，避免每次手敲一长串参数：

```bash
# 服务端
./scripts/nps-server.sh run                # 前台调试
./scripts/nps-server.sh install            # 注册系统服务
./scripts/nps-server.sh start|stop|restart|status|uninstall|update
./scripts/nps-server.sh gencert            # 生成/更新 TLS 证书
./scripts/nps-server.sh tls on|off         # bridge 切到 tls / 切回 tcp（同时写好证书路径）

# 客户端（-server 多地址、tls、tls_verify、代理、basic 凭据统一入口）
./scripts/npc-client.sh run --server 10.0.0.1:8024,10.0.0.2:8024 --vkey 123 --type tls
./scripts/npc-client.sh install --server 10.0.0.1:8024 --vkey 123 --type tcp
./scripts/npc-client.sh start|stop|restart|status|uninstall
```

`nps-server.sh tls on` 会顺手把 `bridge_type=tls`、`bridge_cert_file`、`bridge_key_file` 写进 `conf/nps.conf`，之后客户端必须用 `-type=tls`（或 `conn_type=tls`）连接。`npc-client.sh install` 的参数会被写进系统服务的启动命令行，`npc-client.sh run --user/--pass` 则写入 `conf/npc.conf` 的 `basic_username` / `basic_password`（这两个只对服务端 `public_vkey` 下发配置的模式生效）。

脚本是 bash，Windows 请在 WSL / Git Bash 里用，或直接按 `docs/usage-zh.md` 里的命令手敲。

---

## 十、后台管理界面

访问 `http://服务器IP:8080`，默认 `admin/123`（务必改）。界面已按 Light-Year Admin 风格重做：

- 左侧导航分四组：概览 / 客户端与节点 / 穿透管理 / 帮助与支持，当前页高亮
- 首页 4 张 KPI 卡片：连接端口、客户端总数、在线客户端、TCP 连接
- 隧道列表页仍在「穿透管理」下，新增/编辑表单支持来源白名单、拒绝列表、限速三项
- 底部版权条不再遮挡表单与侧栏最后一项

多用户：`allow_user_login=true` 开启后可在 `/login/register` 注册，配置文件方式见 `allow_user_register`、`allow_user_change_username`。

---

## 十一、常见问题

| 现象 | 排查方向 |
| --- | --- |
| 客户端连不上 | `bridge_port` 是否放行、`vkey` 是否等于服务端 `public_vkey`、`bridge_type` 是否与 `conn_type` 一致（tcp/kcp） |
| 后台能登录但没有客户端 | `ip_limit=true` 时是否把客户端出口 IP 加进了允许列表 |
| 穿透端口连上就断 | 隧道配置了 `ip_acl`/`deny_ip` 且当前来源不在名单，或被 `tunnel_conn_rate` 封禁；看 `log_level=7` 的 debug 日志 |
| 隧道开了 tls 后连不上、也没有任何响应 | 客户端用明文连了 TLS 端口（TLS 握手失败直接断开）；改用 TLS 连接，或去掉隧道 TLS |
| 隧道认证总是 401 / AUTH FAILED | 凭据用 `base64(user:pass)`，注意大小写；行协议要带 `Basic` 前缀；密码含冒号时 base64 里的分隔仍是第一个冒号 |
| `-type=tls` 报证书错误 | 服务端用自签证书时不要开 `-tls_verify` / `tls_verify=true`，或换成受信任的 `bridge_cert_file` |
| 服务端提示拿不到外部 IP | 属于预期行为：不再向第三方服务查询出口 IP，需要时手动填 `external_ip` |
| 全新部署启动即退出 | 检查 `conf/` 下 `clients.json`、`hosts.json`、`tasks.json` 是否可写；确认 `web/views`、`web/static` 与二进制在同一级目录 |
| 想指定服务名/日志 | `log_level`、`log_path`；pprof 用 `pprof_port` |

配置文件与数据的默认位置：安装模式在 `/etc/nps`（Linux）、`C:\Program Files\nps`（Windows），前台运行则在当前目录下找 `conf/nps.conf`。

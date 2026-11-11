# 扩展功能
## 缓存支持
对于web站点来说，一些静态文件往往消耗更大的流量，且在内网穿透中，静态文件还需到客户端获取一次，这将导致更大的流量消耗。nps在域名解析代理中支持对静态文件进行缓存。

即假设一个站点有a.css，nps将只需从npc客户端读取一次该文件，然后把该文件的内容放在内存中，下一次将不再对npc客户端进行请求而直接返回内存中的对应内容。该功能默认是关闭的，如需开启请在`nps.conf`中设置`http_cache=true`，并设置`http_cache_length`（缓存文件的个数，消耗内存，不宜过大，0表示不限制个数）

## 数据压缩支持

由于是内网穿透，内网客户端与服务端之间的隧道存在大量的数据交换，为节省流量，加快传输速度，由此本程序支持SNNAPY形式的压缩。


- 所有模式均支持数据压缩
- 在web管理或客户端配置文件中设置


## 加密传输

如果公司内网防火墙对外网访问进行了流量识别与屏蔽，例如禁止了ssh协议等，通过设置 配置文件，将服务端与客户端之间的通信内容加密传输，将会有效防止流量被拦截。
- nps现在默认每次启动时随机生成tls证书，用于加密传输



## 站点保护
域名代理模式所有客户端共用一个http服务端口，在知道域名后任何人都可访问，一些开发或者测试环境需要保密，所以可以设置用户名和密码，nps将通过 Http Basic Auth 来保护，访问时需要输入正确的用户名和密码。


- 在web管理或客户端配置文件中设置

## host修改

由于内网站点需要的host可能与公网域名不一致，域名代理支持host修改功能，即修改request的header中的host字段。

**使用方法：在web管理中设置**

## 自定义header

支持对header进行新增或者修改，以配合服务的需要

## 404页面配置
支持域名解析模式的自定义404页面，修改/web/static/page/error.html中内容即可，暂不支持静态文件等内容

## 流量限制

支持客户端级流量限制，当该客户端入口流量与出口流量达到设定的总量后会拒绝服务
，域名代理会返回404页面，其他代理会拒绝连接,使用该功能需要在`nps.conf`中设置`allow_flow_limit`，默认是关闭的。

## 带宽限制

支持客户端级带宽限制，带宽计算方式为入口和出口总和，权重均衡,使用该功能需要在`nps.conf`中设置`allow_rate_limit`，默认是关闭的。

## 负载均衡
本代理支持域名解析模式和tcp代理的负载均衡，在web域名添加或者编辑中内网目标分行填写多个目标即可实现轮训级别的负载均衡

## 端口白名单
为了防止服务端上的端口被滥用，可在nps.conf中配置allow_ports限制可开启的端口，忽略或者不填表示端口不受限制，格式：

```ini
allow_ports=9001-9009,10001,11000-12000
```

## 端口范围映射
当客户端以配置文件的方式启动时，可以将本地的端口进行范围映射，仅支持tcp和udp模式，例如：

```ini
[tcp]
mode=tcp
server_port=9001-9009,10001,11000-12000
target_port=8001-8009,10002,13000-14000
```

逗号分隔，可单个或者范围，注意上下端口的对应关系，无法一一对应将不能成功
## 端口范围映射到其他机器
```ini
[tcp]
mode=tcp
server_port=9001-9009,10001,11000-12000
target_port=8001-8009,10002,13000-14000
target_ip=10.1.50.2
```
填写target_ip后则表示映射的该地址机器的端口，忽略则便是映射本地127.0.0.1,仅范围映射时有效

## KCP协议支持

在网络质量非常好的情况下，例如专线，内网，可以开启略微降低延迟。如需使用可在nps.conf中修改`bridge_type`为kcp
，设置后本代理将开启udp端口（`bridge_port`）

注意：当服务端为kcp时，客户端连接时也需要使用相同配置，无配置文件模式加上参数type=kcp,配置文件模式在配置文件中设置tp=kcp

## 域名泛解析
支持域名泛解析，例如将host设置为*.proxy.com，a.proxy.com、b.proxy.com等都将解析到同一目标，在web管理中或客户端配置文件中将host设置为此格式即可。

## URL路由
本代理支持根据URL将同一域名转发到不同的内网服务器，可在web中或客户端配置文件中设置，此参数也可忽略，例如在客户端配置文件中

```ini
[web1]
host=a.proxy.com
target_addr=127.0.0.1:7001
location=/test
[web2]
host=a.proxy.com
target_addr=127.0.0.1:7002
location=/static
```
对于`a.proxy.com/test`将转发到`web1`，对于`a.proxy.com/static`将转发到`web2`

## 限制ip访问
如果将一些危险性高的端口例如ssh端口暴露在公网上，可能会带来一些风险，本代理支持限制ip访问。

**使用方法:** 在配置文件nps.conf中设置`ip_limit`=true，设置后仅通过注册的ip方可访问。

**ip注册**：

**方式一：**
在需要访问的机器上，运行客户端

```
./npc register -server=ip:port -vkey=公钥或客户端密钥 time=2
```

time为有效小时数，例如time=2，在当前时间后的两小时内，本机公网ip都可以访问nps代理.

**方式二：**
此外nps的web登陆也可提供验证的功能，成功登陆nps web admin后将自动为登陆的ip注册两小时的允许访问权限。


**注意：** 本机公网ip并不是一成不变的，请自行注意有效期的设置，同时同一网络下，多人也可能是在公用同一个公网ip。

## 穿透端口的来源白名单（推荐给 3389 等端口用）
`ip_limit` 是“注册后才能连”的弱限制，端口本身仍然对全公网可达：扫描器只要能建 TCP 连接，
就能把请求一路转给内网的 3389，Windows 的账户锁定策略会因此被爆破触发（连续失败若干次即锁定账号）。

来源白名单在监听层就拦住非法来源：非白名单地址的连接在读取第一个字节前就关闭（并发送 RST），
扫描器看不到真实服务的 banner，爆破请求根本到不了内网。

**全局默认值（nps.conf，可被单个隧道覆盖）**

```
#单个 ip、CIDR、前缀通配，逗号分隔
allow_ip=1.2.3.4,10.0.0.0/8,192.168.1.*
deny_ip=8.8.8.8
#每个来源 ip 每秒允许的新建连接数，超出后封禁 tunnel_ban_seconds 秒；不填表示不限速
tunnel_conn_rate=10
tunnel_conn_window=1
tunnel_ban_seconds=600
```

**单个隧道**：web 新建/编辑隧道时填写“允许访问的 IP”“拒绝访问的 IP”“单 IP 连接速率”，
优先级高于 nps.conf 的全局值。对 `tcp`、`socks5`、`udp`、`http正向代理` 生效。

`deny_ip` 优先于 `allow_ip`；两者都留空表示不做来源限制（保持原有行为）。
被拦截的连接只按 10 秒一条的摘要写入 debug 日志，不会刷爆日志文件。
## TCP 隧道的 Basic 认证与 TLS（隧道级）
来源白名单解决“谁能连”，这一节解决“连上的人能不能用”和“链路是否明文”。两者都作用在
建立隧道之前，可以叠加使用。

**隧道字段**

```
auth_user=visitor       # 留空表示不认证
auth_pass=change-me
tls=true                # 该端口只接受 TLS 连接
tls_cert_file=conf/a.pem # 可选，缺省用 bridge_cert_file，再缺省用内建自签证书
tls_key_file=conf/a.key
```

web 后台的 TCP 隧道新增/编辑页有同名字段，隧道级配置优先于全局。

**处理顺序**（`server/proxy/tcp.go` 的 accept 回调）

1. 来源守卫检查（`allow_ip` / `deny_ip` / 连接速率），不通过直接 RST；
2. `tls=true` 时做 TLS 握手（10 秒超时），失败直接断开；
3. `auth_user`/`auth_pass` 非空时做一次 Basic 认证（10 秒超时），失败按形态回 401 或 `AUTH FAILED`，
   首包既不是 HTTP 也不是认证帧时静默 RST；
4. 认证通过才占用客户端连接数并向内网发起转发，认证帧之外已读到的字节通过 `preReadConn`
   回放给目标，不影响协议本身。

**三种握手**

- 行协议：`AUTH Basic <base64(user:pass)>`
- HTTP CONNECT：`CONNECT host:port HTTP/1.1` + `Proxy-Authorization: Basic ...`，通过后回
  `200 Connection established`，适合 stunnel / ncat --proxy
- 普通 HTTP 请求：`Authorization: Basic ...`，认证头在转发前被剥离

**桥接通道 TLS**

`bridge_type=tls` 让客户端到服务端的控制通道也走 TLS，证书同样取
`bridge_cert_file` / `bridge_key_file`（缺省内建自签，带 `localhost` / `127.0.0.1` SAN）。
客户端用 `conn_type=tls`（或 `npc -type=tls`），默认不校验自签证书，`tls_verify=true` 可打开校验。
`bridge_port` 与 web/https 端口复用时（pmux）不支持 tls 桥接，会直接报错，换独立端口即可。
## 常见端口优化（3389 / 22 / 21 / 80 / 443）
内网穿透 90% 的用途就是把 3389(RDP)、22(SSH)、21(FTP)、80/443(Web) 暴露到公网，
它们的握手特征完全不同，一套参数硬套一定出问题。nps 内置了一张端口画像表
（`lib/common/portprofile.go`），在建立隧道时自动套用：

| 端口 | 协议特征 | 首字节守卫 | 失效连接回收 |
| --- | --- | --- | --- |
| 3389 / 22 / 23 | 客户端先说话 | 15s 内无数据即关闭 | TCP_USER_TIMEOUT 2 分钟 |
| 80 / 443 / 8080 / 8443 | 客户端先发请求 | 同上 | 2 分钟 |
| 5900 / 5901 | VNC，客户端先说话 | 同上 | 2 分钟 |
| 21 / 20（FTP） | **服务端先发 220** | 跳过，绝不等待 | 5 分钟 |
| 3306（MySQL） | **服务端先发握手包** | 跳过 | 5 分钟 |
| 其他端口 | 未知 | 默认不等待 | 5 分钟 |

**首字节守卫**：RDP/SSH/HTTP 都是客户端先发数据，端口扫描器连上之后往往什么都不发
（只等 banner）。这类连接以前会一直挂着，既占用客户端连接数又暴露端口真实状态；
现在会在建立隧道之前等一次首包，超时即关闭，而且**不会触达内网目标**。

```
# nps.conf，默认 15 秒，设为 0 关闭
first_byte_timeout=15
```

判定顺序：先看公网端口，公网端口没命中画像表时回退看内网目标端口
（例如公网 13389 → 内网 3389 同样生效）。FTP、MySQL 这类“服务端先说话”的协议自动跳过。

**失效连接回收（Linux）**：RDP 客户端拔网线/切 Wi-Fi 后连接不会正常关闭，
服务端会一直往不可达的对端重传，默认十几分钟才失败，期间协程与 fd 都占着。
nps 在 Linux 上对每条隧道连接设置 `TCP_USER_TIMEOUT`（交互类 2 分钟），
让失效会话尽快释放；Windows/macOS 没有等价选项，靠 KeepAlive（30 秒）处理。

**其他配套**：公网侧与内网侧连接都开 `TCP_NODELAY`（小包即时下发）、`SO_KEEPALIVE`
（NAT/安全组 5~10 分钟回收空闲条目的问题），半关闭（`CloseWrite`）保证
“请求发完就 shutdown(SHUT_WR)”的客户端仍能收完响应（见 `lib/common/closewrite.go`）。

## FTP 数据通道（被动模式 + 主动模式）
FTP 是唯一一个「控制通道通了还不算通」的常见协议：文件内容走另一条数据连接，
而这条连接的地址和端口是由控制通道里的报文临时协商的。

| 模式 | 协商方向 | 原始报文里的地址 | 不改写的结果 |
| --- | --- | --- | --- |
| 被动 PASV/EPSV | 服务端 → 客户端 | 内网 IP + 随机端口 | 公网客户端连不上，卡在 `150` 之后 |
| 主动 PORT/EPRT | 客户端 → 服务端 | 客户端自己的地址 | 内网 FTP 连不到这个地址 |

nps 在 `mode=ftp` 的隧道上（`mode=tcp` 且内网目标端口是 21 的隧道默认也按这个处理）
把两类报文里的地址统一改成**服务端公网地址 + 中继端口**，数据连接进来后再经隧道
送到真正的对端：

```
227 Entering Passive Mode (10,0,0,8,196,130)   内网 FTP 的原始回答
227 Entering Passive Mode (203,0,113,7,156,65)  nps 改写后，156*256+65 = 40001
```

```ini
# nps.conf
#对外通告的地址，留空则用 external_ip，再为空则用控制连接的本地地址
ftp_public_ip=203.0.113.7
#被动模式中继端口范围（建议在安全组/防火墙里整段放行）
ftp_pasv_ports=40000-40050
#主动模式中继端口范围，留空则复用被动范围
ftp_active_ports=40100-40150
#中继端口等不到数据连接就释放（秒）
ftp_relay_timeout=60
#auto：tcp 隧道打到 21 端口也自动改写；off：只改写 mode=ftp 的隧道
ftp_alg=auto
```

后台「FTP 隧道」页面或 REST API 都能创建：

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/tunnels \
  -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" \
  -d '{"client_id":2,"mode":"ftp","port":21,"target":"10.0.0.8:21"}'
```

几点行为说明：

- 中继端口每个数据连接用一个，传输结束即释放；同一控制连接同时最多保留 `ftp_max_relay` 个
  （默认 32），超出时回收最早的那个。多线程客户端（lftp 默认 5 条、FileZilla 可配到 10 条）
  会同时占多个中继端口，上限过小会把还在等数据连接的中继端口回收掉，并发传输随机失败
- 中继端口上的数据连接同样受隧道的来源白名单约束（`ip_acl` / `deny_ip`）：被动模式要求与
  控制连接同源，主动模式由内网 FTP 服务发起、只判黑名单
- 配了 `ftp_pasv_ports` 却整段都被占用时，中继直接失败并记 warn，**不会**偷偷换成随机端口 ——
  随机高位端口几乎不可能在云安全组里放行，静默降级只会让客户端连上去后一直卡在 `150`。
  只有完全没配范围时才用随机端口，并在启动时提示一次
- `EPSV` 只通告端口不带地址，因此服务端没有可用公网 IPv4（纯 IPv6 或多层 NAT）时被动模式
  仍可用于支持 `EPSV` 的客户端，`PASV` / `PORT` / `EPRT` 才需要 `ftp_public_ip`
- 少数 FTP 服务的 `227` 是多行响应（`227-...`）或不带括号，这两种写法同样会被识别并改写
- 被动模式的内网目标固定用**隧道目标地址 + FTP 通告的端口**，因为很多 FTP 服务的
  PASV 回答里是 `0.0.0.0` 或者另一块网卡的内网地址
- 主动模式只有当客户端通告的地址在内网一侧可达时才成功（客户端在 NAT 后面、通告的是
  私网地址时本来就无法主动模式，这是 FTP 协议的限制）；失败只记一条 warn，不影响控制通道
- 客户端执行 `AUTH TLS`（显式 FTPS）后控制通道变加密，这条会话的改写会自动停掉并记一条日志，
  行为回到改造前；需要加密传输时建议用 `mode=ftp` + 隧道级 TLS（`tls_enable`）承载控制通道
- 不再需要在内网 FTP 上手工固定 PASV 端口段，也不必为每个数据端口各建一条隧道

## 客户端最大连接数
为防止恶意大量长连接，影响服务端程序的稳定性，可以在web或客户端配置文件中为每个客户端设置最大连接数。该功能针对`socks5`、`http正向代理`、`域名代理`、`tcp代理`、`udp代理`、`私密代理`生效,使用该功能需要在`nps.conf`中设置`allow_connection_num_limit=true`，默认是关闭的。

## 客户端最大隧道数限制
nps支持对客户端的隧道数量进行限制，该功能默认是关闭的，如需开启，请在`nps.conf`中设置`allow_tunnel_num_limit=true`。
## 端口复用
在一些严格的网络环境中，对端口的个数等限制较大，nps支持强大端口复用功能。将`bridge_port`、 `http_proxy_port`、 `https_proxy_port` 、`web_port`都设置为同一端口，也能正常使用。

- 使用时将需要复用的端口设置为与`bridge_port`一致即可，将自动识别。
- 如需将web管理的端口也复用，需要配置`web_host`也就是一个二级域名以便区分

## 多路复用

nps主要通信默认基于多路复用，无需开启。

多路复用基于TCP滑动窗口原理设计，动态计算延迟以及带宽来算出应该往网络管道中打入的流量。
由于主要通信大多采用TCP协议，并无法探测其实时丢包情况，对于产生丢包重传的情况，采用较大的宽容度，
5分钟的等待时间，超时将会关闭当前隧道连接并重新建立，这将会抛弃当前所有的连接。
在Linux上，可以通过调节内核参数来适应不同应用场景。

对于需求大带宽又有一定的丢包的场景，可以保持默认参数不变，尽可能少抛弃连接
高并发下可根据[Linux系统限制](## Linux系统限制) 调整

对于延迟敏感而又有一定丢包的场景，可以适当调整TCP重传次数
`tcp_syn_retries`, `tcp_retries1`, `tcp_retries2`
高并发同上
nps会在系统主动关闭连接的时候拿到报错，进而重新建立隧道连接

## 环境变量渲染
npc支持环境变量渲染以适应在某些特殊场景下的要求。

**在无配置文件启动模式下：**
设置环境变量
```
export NPC_SERVER_ADDR=1.1.1.1:8024
export NPC_SERVER_VKEY=xxxxx
```
直接执行./npc即可运行

**在配置文件启动模式下：**
```ini
[common]
server_addr={{.NPC_SERVER_ADDR}}
conn_type=tcp
vkey={{.NPC_SERVER_VKEY}}
auto_reconnection=true
[web]
host={{.NPC_WEB_HOST}}
target_addr={{.NPC_WEB_TARGET}}
```
在配置文件中填入相应的环境变量名称，npc将自动进行渲染配置文件替换环境变量

## 健康检查

当客户端以配置文件模式启动时，支持多节点的健康检查。配置示例如下

```ini
[health_check_test1]
health_check_timeout=1
health_check_max_failed=3
health_check_interval=1
health_http_url=/
health_check_type=http
health_check_target=127.0.0.1:8083,127.0.0.1:8082

[health_check_test2]
health_check_timeout=1
health_check_max_failed=3
health_check_interval=1
health_check_type=tcp
health_check_target=127.0.0.1:8083,127.0.0.1:8082
```
**health关键词必须在开头存在**

第一种是http模式，也就是以get的方式请求目标+url，返回状态码为200表示成功

第一种是tcp模式，也就是以tcp的方式与目标建立连接，能成功建立连接表示成功

如果失败次数超过`health_check_max_failed`，nps则会移除该npc下的所有该目标，如果失败后目标重新上线，nps将自动将目标重新加入。

项 | 含义
---|---
health_check_timeout |  健康检查超时时间
health_check_max_failed |  健康检查允许失败次数
health_check_interval |  健康检查间隔
health_check_type |  健康检查类型
health_check_target |  健康检查目标，多个以逗号（,）分隔
health_check_type |  健康检查类型
health_http_url |  健康检查url，仅http模式适用

## 日志输出

日志输出级别

**对于npc：**
```
-log_level=0~7 -log_path=npc.log
```
```
LevelEmergency->0  LevelAlert->1

LevelCritical->2 LevelError->3

LevelWarning->4 LevelNotice->5

LevelInformational->6 LevelDebug->7
```
默认为全输出,级别为0到7

**对于nps：**

在`nps.conf`中设置相关配置即可

## pprof性能分析与调试

可在服务端与客户端配置中开启pprof端口，用于性能分析与调试，注释或留空相应参数为关闭。

默认为关闭状态

## 自定义客户端超时检测断开时间

客户端与服务端间会间隔5s相互发送延迟测量包，这个时间间隔不可修改。
可修改延迟测量包丢包的次数，默认为60也就是5分钟都收不到一个延迟测量回包，则会断开客户端连接。
值得注意的是需要客户端的socket关闭，才会进行重连，也就是当客户端无法收到服务端的fin包时，只有客户端自行关闭socket才行。
也就是假如服务端设置为较低值，而客户端设置较高值，而此时服务端断开连接而客户端无法收到服务端的fin包，客户端也会继续等着直到触发客户端的超时设置。

在`nps.conf`或`npc.conf`中设置`disconnect_timeout`即可，客户端还可附带`-disconnect_timeout=60`参数启动

## 多服务端穿透

同一个内网服务可以同时挂到多个 nps 服务端节点上，客户端会为每个服务端建立**独立的控制通道与隧道**，任何一个节点掉线都不影响其它节点，也不会中断已有的本地监听。

在 `npc.conf` 的 `[common]` 中把 `server_addr` 写成多个地址即可，支持逗号、分号或空格分隔：

```ini
[common]
server_addr=nps-a.example.com:8024,nps-b.example.com:8024
conn_type=tcp
vkey=123
auto_reconnection=true
```

命令行方式同样支持：

```bash
./npc -server=10.0.0.1:8024,10.0.0.2:8024 -vkey=123 -type=tcp
```

行为说明：

- `[common]` 下除 `server_addr` 外的配置，以及所有 host / tunnel 段，会分别注册到每个服务端；同一个 `server_port` 在不同机器上互不影响。
- 每个服务端单独重连（间隔 5 秒），失败日志带 `[服务端地址]` 前缀，便于区分节点。
- `secret` / `p2p` 的本地监听端口（`local_port`）只在客户端启动一次，不会因为节点数量而重复绑定。
- `file` 模式的本地文件服务跟随各自的服务端建立独立隧道。
- 公网模式（服务端返回临时 vkey）下，`npc_vkey.txt` 只记录第一个服务端的 vkey，用于 `npc status` 查询；其余服务端各自使用自己的 vkey 运行，互不覆盖。

## 后台管理界面

管理界面在保留原有功能与语言切换机制的前提下，引入了 Light-Year-Admin-Template v5 的视觉风格，样式集中在 `web/static/css/lightyear.css`，通过覆盖层实现，不改动既有 DOM 结构与脚本：

- 深蓝渐变侧边栏，菜单按「概览 / 客户端与节点 / 穿透管理 / 帮助与支持」分组，当前页高亮为主色胶囊
- 浅灰画布 + 白色圆角卡片 + 柔和阴影，卡片 hover 提升
- 仪表盘 KPI 改为彩色图标卡片（连接端口 / 客户端总数 / 在线客户端 / TCP 连接）
- 表格表头、行 hover、分页、搜索框、按钮统一为主题色
- 输入框聚焦主色描边，登录页与注册页使用浮动标签（标签随聚焦 / 有值上浮，不再只是占位符）

新增分组标题的词条位于 `web/static/page/languages.xml`（`word-group-overview`、`word-group-node`、`word-group-tunnel`、`word-group-support`），需要其它语言时按同样格式补充即可。


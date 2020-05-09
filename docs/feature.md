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

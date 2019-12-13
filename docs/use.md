# 基本使用
## 无配置文件模式
此模式的各种配置在服务端web管理中完成,客户端除运行一条命令外无需任何其他设置
```
 ./npc -server=ip:port -vkey=web界面中显示的密钥
```
## 配置文件模式
此模式使用nps的公钥或者客户端私钥验证，各种配置在客户端完成，同时服务端web也可以进行管理
```
 ./npc -config=npc配置文件路径
```
可自行添加systemd service，例如：`npc.service`
```
[Unit]
Description=npc - convenient proxy server client
Documentation=https://github.com/cnlh/nps/
After=network-online.target remote-fs.target nss-lookup.target
Wants=network-online.target

[Service]
Type=simple
KillMode=process
Restart=always
RestartSec=15s
StandardOutput=append:/var/log/nps/npc.log
ExecStartPre=/bin/echo 'Starting npc'
ExecStopPost=/bin/echo 'Stopping npc'
ExecStart=/absolutely path to/npc -server=ip:port -vkey=web界面中显示的密钥

[Install]
WantedBy=multi-user.target
```
## 配置文件说明
[示例配置文件](https://github.com/cnlh/nps/tree/master/conf/npc.conf)
#### 全局配置
```ini
[common]
server_addr=1.1.1.1:8284
conn_type=tcp
vkey=123
username=111
password=222
compress=true
crypt=true
rate_limit=10000
flow_limit=100
remark=test
max_conn=10
```
项 | 含义
---|---
server_addr | 服务端ip:port
conn_type | 与服务端通信模式(tcp或kcp)
vkey|服务端配置文件中的密钥(非web)
username|socks5或http(s)密码保护用户名(可忽略)
password|socks5或http(s)密码保护密码(可忽略)
compress|是否压缩传输(true或false或忽略)
crypt|是否加密传输(true或false或忽略)
rate_limit|速度限制，可忽略
flow_limit|流量限制，可忽略
remark|客户端备注，可忽略
max_conn|最大连接数，可忽略
#### 域名代理

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[web1]
host=a.proxy.com
target_addr=127.0.0.1:8080,127.0.0.1:8082
host_change=www.proxy.com
header_set_proxy=nps
```
项 | 含义
---|---
web1 | 备注
host | 域名(http|https都可解析)
target_addr|内网目标，负载均衡时多个目标，逗号隔开
host_change|请求host修改
header_xxx|请求header修改或添加，header_proxy表示添加header proxy:nps

#### tcp隧道模式

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[tcp]
mode=tcp
target_addr=127.0.0.1:8080
server_port=9001
```
项 | 含义
---|---
mode | tcp
server_port | 在服务端的代理端口
tartget_addr|内网目标

#### udp隧道模式

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[udp]
mode=udp
target_addr=127.0.0.1:8080
server_port=9002
```
项 | 含义
---|---
mode | udp
server_port | 在服务端的代理端口
target_addr|内网目标
#### http代理模式

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[http]
mode=httpProxy
server_port=9003
```
项 | 含义
---|---
mode | httpProxy
server_port | 在服务端的代理端口
#### socks5代理模式

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[socks5]
mode=socks5
server_port=9004
multi_account=multi_account.conf
```
项 | 含义
---|---
mode | socks5
server_port | 在服务端的代理端口
multi_account | socks5多账号配置文件（可选),配置后使用basic_username和basic_password无法通过认证
#### 私密代理模式

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[secret_ssh]
mode=secret
password=ssh2
target_addr=10.1.50.2:22
```
项 | 含义
---|---
mode | secret
password | 唯一密钥
target_addr|内网目标

#### p2p代理模式

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[p2p_ssh]
mode=p2p
password=ssh2
target_addr=10.1.50.2:22
```
项 | 含义
---|---
mode | p2p
password | 唯一密钥
target_addr|内网目标


#### 文件访问模式
利用nps提供一个公网可访问的本地文件服务，此模式仅客户端使用配置文件模式方可启动

```ini
[common]
server_addr=1.1.1.1:8284
vkey=123
[file]
mode=file
server_port=9100
local_path=/tmp/
strip_pre=/web/
````

项 | 含义
---|---
mode | file
server_port | 服务端开启的端口
local_path|本地文件目录
strip_pre|前缀

对于`strip_pre`，访问公网`ip:9100/web/`相当于访问`/tmp/`目录

#### 断线重连
```ini
[common]
auto_reconnection=true
```

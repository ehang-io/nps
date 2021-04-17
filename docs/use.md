# 基本使用
## 无配置文件模式
此模式的各种配置在服务端web管理中完成,客户端除运行一条命令外无需任何其他设置
```
 ./npc -server=ip:port -vkey=web界面中显示的密钥
```
## 注册到系统服务(开机启动、守护进程)
对于linux、darwin
- 注册：`sudo ./npc install 其他参数（例如-server=xx -vkey=xx或者-config=xxx）`
- 启动：`sudo npc start`
- 停止：`sudo npc stop`
- 如果需要更换命令内容需要先卸载`./npc uninstall`，再重新注册

对于windows，使用管理员身份运行cmd

- 注册：`npc.exe install 其他参数（例如-server=xx -vkey=xx或者-config=xxx）`
- 启动：`npc.exe start`
- 停止：`npc.exe stop`
- 如果需要更换命令内容需要先卸载`npc.exe uninstall`，再重新注册
- 如果需要当客户端退出时自动重启客户端，请按照如图所示配置
![image](https://github.com/ehang-io/nps/blob/master/docs/windows_client_service_configuration.png?raw=true)

注册到服务后，日志文件windows位于当前目录下，linux和darwin位于/var/log/npc.log

## 客户端更新
首先进入到对于的客户端二进制文件目录

请首先执行`sudo npc stop`或者`npc.exe stop`停止运行，然后

对于linux
```shell
 sudo npc-update update
```
对于windows
```shell
npc-update.exe update
```

更新完成后，执行执行`sudo npc start`或者`npc.exe start`重新运行即可完成升级

如果无法更新成功，可以直接自行下载releases压缩包然后覆盖原有的npc二进制文件

## 配置文件模式
此模式使用nps的公钥或者客户端私钥验证，各种配置在客户端完成，同时服务端web也可以进行管理
```
 ./npc -config=npc配置文件路径
```
## 配置文件说明
[示例配置文件](https://github.com/ehang-io/nps/tree/master/conf/npc.conf)
#### 全局配置
```ini
[common]
server_addr=1.1.1.1:8024
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
#pprof_addr=0.0.0.0:9999
```
项 | 含义
---|---
server_addr | 服务端ip/域名:port
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
pprof_addr|debug pprof ip:port
#### 域名代理

```ini
[common]
server_addr=1.1.1.1:8024
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
server_addr=1.1.1.1:8024
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
server_addr=1.1.1.1:8024
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
server_addr=1.1.1.1:8024
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
server_addr=1.1.1.1:8024
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
server_addr=1.1.1.1:8024
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
server_addr=1.1.1.1:8024
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
server_addr=1.1.1.1:8024
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

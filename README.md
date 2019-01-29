# easyProxy
![](https://img.shields.io/github/stars/cnlh/easyProxy.svg)   ![](https://img.shields.io/github/forks/cnlh/easyProxy.svg) ![](https://img.shields.io/github/license/cnlh/easyProxy.svg)

easyProxy是一款轻量级、高性能、功能最为强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何tcp、udp上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**，可实现在非内网环境下如同使用vpn一样访问内网资源和设备的效果。

目前市面上提供类似服务的有花生壳、TeamView、GoToMyCloud等等，但要使用第三方的公网服务器就必须为第三方付费，并且这些服务都有各种各样的限制，此外，由于数据包会流经第三方，因此对数据安全也是一大隐患。


支持客户端与服务端连接中断自动重连，多路传输，大大的提高请求处理速度，go语言编写，无第三方依赖，各个平台都已经编译在release中，普通个人场景下，内存使用量在10M以下。

## 背景
![image](https://github.com/cnlh/easyProxy/blob/master/image/web.png?raw=true)
1. web管理模式，可配置多条tcp、udp隧道，多个域名代理等等----> [web管理模式](#web管理模式)


2. 想在外网通过ssh连接内网的机器，做云服务器到内网服务器端口的映射，或者做微信公众号开发、小程序开发等---->[tcp隧道模式](#tcp隧道模式)

3. 在非内网环境下使用内网dns，或者需要通过udp访问内网机器等---->[udp隧道模式](#udp隧道模式)

4. 在外网使用HTTP代理访问内网站点---->[http代理模式](#http代理模式)

5. 搭建一个内网穿透ss，在外网如同使用内网vpn一样访问内网资源或者设备----> [socks5代理模式](#socks5代理模式)


## 目录

* [安装](#安装)
    * [编译安装](#源码安装)
    * [release安装](#release安装)
* [web管理](#web管理模式)（多隧道时推荐）
    * [启动](#启动)
    * [配置文件说明](#服务端配置文件)
* 单隧道模式及介绍
    * [tcp隧道模式](#tcp隧道模式)
    * [udp隧道模式](#udp隧道模式)
    * [socks5代理模式](#socks5代理模式)
    * [http代理模式](#http代理模式)

* [相关功能](#相关功能)
   * [数据压缩支持](#数据压缩支持)
   * [站点密码保护](#站点保护)
   * [加密传输](#加密传输)
   * [TCP多路复用](#多路复用)
   * [host修改](#host修改)
   * [自定义header](#自定义header)
   * [自定义404页面](#404页面配置)
   * [流量限制](#流量限制)
   * [带宽限制](#带宽限制)
* [相关说明](#相关说明)
   * [流量统计](#流量统计)
   * [连接池](#连接池)
   * [热更新支持](#热更新支持)
   * [获取用户真实ip](#获取用户真实ip)
   * [客户端地址显示](#客户端地址显示)
* [web API](#web API)
   *[]

## 安装

### release安装
> https://github.com/cnlh/easyProxy/releases

下载对应的系统版本即可，服务端和客户端是单独的，go语言开发，无需任何第三方依赖

### 源码安装
- 安装源码(另有snappy、beego包)
> go get github.com/cnlh/easyProxy
- 编译
> go build cmd/server/proxy_server.go

> go build cmd/client/proxy_client.go

## web管理模式

![image](https://github.com/cnlh/easyProxy/blob/master/image/web2.png?raw=true)
### 介绍

可在网页上配置和管理各个tcp、udp隧道、内网站点代理等等，功能极为强大，操作也非常方便。
### 服务端配置文件
- /conf/app.conf

名称 | 含义
---|---
httpport | web管理端口
password | web界面管理密码
hostPort | 域名代理模式监听端口
tcpport  | 服务端客户端通信端口

**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**

### 启动


- 服务端

```
 ./proxy_server
```

- 客户端
```
 ./proxy_server -server=ip:port -vkey=web界面中显示的
```

进入web管理界面，有详细的命令

- 配置

进入web界面，公网ip:web界面端口（默认8080），密码默认为123



## tcp隧道模式

### 场景及原理
较为适用于处理tcp连接，例如ssh，同时也适用于http等，访问服务端的8024端口相当于访问内网10.1.50.202机器的4000端口，构成如下所示的隧道。

![image](https://github.com/cnlh/easyProxy/blob/master/image/tcp.png?raw=true)

例如：

**背景:**

- 内网机器10.1.50.203提供了web服务80端口

- 有VPS一个,公网IP:123.206.77.88

**需求:**

在家里能够通过访问VPS的8024端口访问到内网机器A的80端口

### 使用
- 服务端

```
./proxy_server -mode=tunnelServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024 -target=10.1.50.203:80
```

名称 | 含义
---|---
mode | 运行模式
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | 外部访问端口
target | 目标地址，格式如上

- 客户端


```
./proxy_client -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 与nginx配合实现访问a.ourcauc.com等同访问10.1.50.203:80效果，将该域名解析道云服务器，nginx配置
```
server {
    listen 80;
    server_name a.ourcauc.com;
    location / {
            #其他配置，例如ssl
            proxy_pass http://127.0.0.1:8024;
        }
}
```
## udp隧道模式

### 场景及原理

**背景**
- 内网机器A提供了DNS解析服务,10.1.50.210:53端口

- 有VPS一个,公网IP:123.206.77.88

**需求:**
在家里能够通过设置本地dns为123.206.77.88,使用内网机器A进行域名解析服务.

访问vps的53端口相当于访问10.1.50.210的53端口，构成如下所示的隧道。

![image](https://github.com/cnlh/easyProxy/blob/master/image/udp.png?raw=true)


### 使用
- 服务端

```
./proxy_server -mode=udpServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=53 -target=10.1.50.210:53
```

名称 | 含义
---|---
mode | 运行模式
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | 公网vps的访问端口
target | 目标地址，格式如上

- 客户端


```
./proxy_client -server=ip:port -vkey=DKibZF5TXvic1g3kY
```




## socks5代理模式

### 场景及原理

**原理**

主要用于socks5代理，也就是和ss类似，不过是代理内网。使用此模式时，可在非内网环境下配置本机的socks5代理（服务器ip、sock5代理端口），即可实现socks5代理，达到访问内网的网站的效果，配合proxifier等全局代理软件，即可如同使用内网vpn一样，访问内网网站，通过ssh连接内网机器等等……。
![image](https://github.com/cnlh/easyProxy/blob/master/image/sock5.png?raw=true)

### 使用
- 服务端

```
./proxy_server -mode=socks5Server -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
```

名称 | 含义
---|---
mode | 运行模式
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | 代理的http端口（socks5连接端口）
u | 验证的用户名
p | 验证的密码

**说明**：用户名和密码验证模式，仅部分socks5客户端支持，例如proxifier。命令行执行加上，web管理模式中可单独配置


```
-u=user -p=password
```

- 客户端


```
./proxy_client -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 需要使用内网代理的机器

```
配置socks5代理即可，ip为外网服务器ip，端口为httpport，即可在外网环境使用内网啦！也可使用proxifier等全局代理软件。
```
如果设置了用户名和密码，记得填上用户名和密码(仅部分客户端支持密码验证)



## http代理模式

### 场景及原理
主要用于HTTP代理，区别也就是HTTP代理和sock5代理的区别。使用此模式时，可在非内网环境下配置本机的HTTP代理（服务器ip、HTTP代理端口），即可实现HTTP代理，达到访问内网的网站的效果。
![image](https://github.com/cnlh/easyProxy/blob/master/image/httpProxy.png?raw=true)


### 使用
- 服务端

```
./proxy_server -mode=httpProxyServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
```

名称 | 含义
---|---
mode | 运行模式
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | http代理连接端口
authip | 免验证ip，适用于web api

- 客户端



```
./proxy_client -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 需要使用内网代理的机器


```
配置HTTP代理即可，ip为外网服务器ip，端口为httpport，即可在外网环境访问内网啦！
```

## 相关功能

### 数据压缩支持

由于是内网穿透，内网客户端与服务端之间的隧道存在大量的数据交换，为节省流量，加快传输速度，由此本程序支持SNNAPY形式的压缩。


- 所有模式均支持数据压缩，可以与加密同时使用
- 开启此功能会增加cpu和内存消耗
- 在server端加上参数 -compress=snappy（或在web管理中设置）
```
-compress=snappy
```

### 加密传输

如果公司内网防火墙对外网访问进行了流量识别与屏蔽，例如禁止了ssh协议等，通过设置 配置文件，将服务端与客户端之间的通信内容加密传输，将会有效防止流量被拦截。

- 开启此功能会增加cpu和内存消耗
- 在server端加上参数 -crypt=true（或在web管理中设置）
```
-crypt=true
```

### 多路复用

客户端和服务器端之间的连接支持多路复用，不再需要为每一个用户请求创建一个连接，使连接建立的延迟降低，并且避免了大量文件描述符的占用。


- 在server端加上参数 -mux=true（或在web管理中设置）

```
-mux=true
```


### 站点保护
域名代理模式所有客户端共用一个http服务端口，在知道域名后任何人都可访问，一些开发或者测试环境需要保密，所以可以设置用户名和密码，easyProxy将通过 Http Basic Auth 来保护，访问时需要输入正确的用户名和密码。


- web管理中可配置

### host修改

由于内网站点需要的host可能与公网域名不一致，域名代理支持host修改功能，即修改request的header中的host字段。

**使用方法：在web管理中设置**

### 自定义header

支持对header进行新增或者修改，以配合服务的需要

### 404页面配置
支持域名解析模式的自定义404页面，修改/web/static/page/error.html中内容即可，暂不支持静态文件等内容

### 流量限制

支持客户端级流量限制，当该客户端入口流量与出口流量达到设定的总量后会拒绝服务
，域名代理会返回404页面，其他代理会拒绝连接

### 带宽限制

支持客户端级带宽限制，带宽计算方式为入口和出口总和，权重均衡

## 相关说明

### 获取用户真实ip

在域名代理模式中，可以通过request请求 header 中的 X-Forwarded-For 和 X-Real-IP 来获取用户真实 IP。

**本代理前会在每一个请求中添加了这两个 header。**

### 热更新支持
在web管理中的修改将实时使用，无需重启客户端或者服务端

### 客户端地址显示
在web管理中将显示客户端的连接地址

### 流量统计
可统计显示每个代理使用的流量，由于压缩和加密等原因，会和实际环境中的略有差异

### 连接池
 easyProxy会预先和后端服务建立起指定数量的连接，每次接收到用户请求后，会从连接池中取出一个连接和用户连接关联起来，避免了等待与后端服务建立连接时间。

## web API

### 客户端

#### 添加客户端
```
POST /client/add/
```
参数 | 含义
---|---
remark | 备注
u | 用户名
p | 密码
compress | 压缩（snappy或空）
crypt | 是否加密（1或者0）
mux | 是否TCP复用（1或者0）
rate_limit|带宽限制
flow_limit|流量限制

#### 添加客户端
```
POST /client/edit/
```
参数 | 含义
---|---
id | id
remark | 备注
u | 用户名
p | 密码
compress | 压缩（snappy或空）
crypt | 是否加密（1或者0）
mux | 是否TCP复用（1或者0）
rate_limit|带宽限制
flow_limit|流量限制

#### 更改状态
```
POST /client/changestatus/
```
参数 | 含义
---|---
id | id
status|1或0

#### 删除客户端
```
POST /client/del/
```
参数 | 含义
---|---
id | id
#### 获取单个客户端
```
POST /client/getclient/
```
参数 | 含义
---|---
id | id
#### 获取客户端列表
```
POST /client/list/
```

参数 | 含义
---|---
start | 开始
length | 长度

### 域名代理

#### 添加域名代理

```
POST /index/addhost/
```

参数 | 含义
---|---
host | 域名
target | 内网目标地址
header | header修改
hostchange | host修改
remark | 备注
client_id | 客户端id
#### 删除域名代理
```
POST /index/delhost/
```

参数 | 含义
---|---
host | 域名

#### 修改域名代理
```
POST /index/edithost/
```

参数 | 含义
---|---
nhost | 修改后的域名
host | 修改之前的域名
target | 内网目标地址
header | header修改
hostchange | host修改
remark | 备注
client_id | 客户端id

#### 获取域名代理列表
```
POST /index/hostlist/
```

参数 | 含义
---|---
start | 开始
length | 长度
client_id | 客户端id（为空获取所有）
#### 获取单个host
```
POST /index/gethost/
```

参数 | 含义
---|---
host|域名


### 其他代理

#### 获取隧道列表

```
POST /index/gettunnel/
```

参数 | 含义
---|---
client_id|客户端id(为空则忽略客户端限制)
type|类型(udpServer、tunnelServer、socks5Server、httpProxyServer，为空则忽略类型限制)
start|开始
length|长度

#### 添加

```
POST /index/add/
```

参数 | 含义
---|---
port|监听端口
type|类型(udpServer、tunnelServer、socks5Server、httpProxyServer)
start|开始
u|验证用户名
p|验证密码
compress|压缩（空或snappy）
crypt|是否加密（1或0）
mux|是否tcp复用（1或0）
use_client|是否使用客户端配置（1或0）
remark|备注
client_id|客户端id

#### 修改

```
POST /index/edit/
```

参数 | 含义
---|---
id|id
port|监听端口
type|类型(udpServer、tunnelServer、socks5Server、httpProxyServer)
start|开始
u|验证用户名
p|验证密码
compress|压缩（空或snappy）
crypt|是否加密（1或0）
mux|是否tcp复用（1或0）
use_client|是否使用客户端配置（1或0）
remark|备注
client_id|客户端id

#### 停止隧道

```
POST /index/stop/
```

参数 | 含义
---|---
id|id

#### 删除隧道

```
POST /index/del/
```

参数 | 含义
---|---
id|id
#### 开始隧道

```
POST /index/start/
```

参数 | 含义
---|---
id|id
#### 获取单条隧道详细

```
POST /index/getonetunnel/
```

参数 | 含义
---|---
id|id
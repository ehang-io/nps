# nps
![](https://img.shields.io/github/stars/cnlh/nps.svg)   ![](https://img.shields.io/github/forks/cnlh/nps.svg) ![](https://img.shields.io/github/license/cnlh/nps.svg)

nps是一款轻量级、高性能、功能强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何**tcp、udp**上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**、**p2p等**，并带有功能强大的web管理端。


## 背景
![image](https://github.com/cnlh/nps/blob/master/image/web.png?raw=true)

1. 做微信公众号开发、小程序开发等----> 域名代理模式


2. 想在外网通过ssh连接内网的机器，做云服务器到内网服务器端口的映射，----> tcp代理模式

3. 在非内网环境下使用内网dns，或者需要通过udp访问内网机器等----> udp代理模式

4. 在外网使用HTTP代理访问内网站点----> http代理模式

5. 搭建一个内网穿透ss，在外网如同使用内网vpn一样访问内网资源或者设备----> socks5代理模式


## 目录

* [安装](#安装)
    * [编译安装](#源码安装)
    * [release安装](#release安装)
* [使用示例（以web主控模式为主）](#使用示例)
    * [统一准备工作](#统一准备工作(必做))
    * [http|https域名解析](#域名解析)
    * [内网ssh连接即tcp隧道](#tcp隧道)
    * [内网dns解析即udp隧道](#udp隧道)
    * [内网socks5代理](#socks5代理)
    * [内网http正向代理](#http正向代理)
    * [内网安全私密代理](#私密代理)
    * [p2p穿透](#p2p服务)
    * [简单的内网文件访问服务](#文件访问模式)
* [服务端](#web管理模式)
    * [服务端启动](#服务端启动)
       * [服务端测试](#服务端测试)
       * [服务端启动](#服务端启动)
       * [web管理](#web管理)
       * [服务端停止或重启](#服务端停止或重启)
    * [配置文件说明](#服务端配置文件)

    * [使用https](#使用https)
    * [与nginx配合](#与nginx配合)
    * [关闭http|https代理](#关闭代理)
    * [将nps安装到系统](#将nps安装到系统)
    * [流量数据持久化](#流量数据持久化)
    * [自定义客户端连接密钥](#自定义客户端连接密钥)
    * [关闭公钥访问](#关闭公钥访问)
    * [关闭web管理](#关闭web管理)
* [客户端](#客户端)
    * [客户端启动](#客户端启动)
        * [无配置文件模式](#无配置文件模式)
        * [配置文件模式](#配置文件模式)
    * [配置文件说明](#配置文件说明)
        * [全局配置](#全局配置)
        * [域名代理](#域名代理)
        * [tcp隧道](#tcp隧道模式)
        * [udp隧道](#udp隧道模式)
        * [http正向代理](#http代理模式)
        * [socks5代理](#socks5代理模式)
        * [私密代理](#私密代理模式)
        * [p2p服务](#p2p代理)
        * [文件访问代理](#文件访问模式)
    * [断线重连](#断线重连)
    * [状态检查](#状态检查)
    * [重载配置文件](#重载配置文件)
    * [通过代理连接nps](#通过代理连接nps)
    * [日志输出级别](#日志输出级别)

* [相关功能](#相关功能)
   * [数据压缩支持](#数据压缩支持)
   * [站点密码保护](#站点保护)
   * [加密传输](#加密传输)
   * [host修改](#host修改)
   * [自定义header](#自定义header)
   * [自定义404页面](#404页面配置)
   * [流量限制](#流量限制)
   * [带宽限制](#带宽限制)
   * [负载均衡](#负载均衡)
   * [端口白名单](#端口白名单)
   * [端口范围映射](#端口范围映射)
   * [端口范围映射到其他机器](#端口范围映射到其他机器)
   * [守护进程](#守护进程)
   * [KCP协议支持](#KCP协议支持)
   * [域名泛解析](#域名泛解析)
   * [URL路由](#URL路由)
   * [限制ip访问](#限制ip访问)
   * [客户端最大连接数限制](#客户端最大连接数)
   * [端口复用](#端口复用)
   * [环境变量渲染](#环境变量渲染)
   * [健康检查](#健康检查)

* [相关说明](#相关说明)
   * [流量统计](#流量统计)
   * [热更新支持](#热更新支持)
   * [获取用户真实ip](#获取用户真实ip)
   * [客户端地址显示](#客户端地址显示)
   * [客户端与服务端版本对比](#客户端与服务端版本对比)
* [简单的性能测试](#简单的性能测试)
   * [qps](#qps)
   * [速度测试](#速度测试)
   * [内存和cpu](#内存和cpu)
   * [额外消耗连接数](#额外消耗连接数)
* [webAPI](#webAPI)
* [贡献](#贡献)
* [交流群](#交流群)



## 安装

### release安装
> https://github.com/cnlh/nps/releases

下载对应的系统版本即可，服务端和客户端是单独的，go语言开发，无需任何第三方依赖

### 源码安装
- 安装源码
> go get -u github.com/cnlh/nps...
- 编译
> go build cmd/nps/nps.go

> go build cmd/npc/npc.go

## 使用示例

### 统一准备工作（必做）
- 开启服务端，假设公网服务器ip为1.1.1.1，配置文件中`bridgePort`为8284，配置文件中`web_port`为8080
- 访问1.1.1.1:8080
- 在客户端管理中创建一个客户端，记录下验证密钥
- 内网客户端运行（windows使用cmd运行加.exe）

```shell
./npc -server=1.1.1.1:8284 -vkey=客户端的密钥
```

### 域名解析

**适用范围：** 小程序开发、微信公众号开发、产品演示

**假设场景：**
- 有一个域名proxy.com，有一台公网机器ip为1.1.1.1
- 两个内网开发站点127.0.0.1:81，127.0.0.1:82
- 想通过（http|https://）a.proxy.com访问127.0.0.1:81，通过（http|https://）b.proxy.com访问127.0.0.1:82

**使用步骤**
- 将*.proxy.com解析到公网服务器1.1.1.1
- 点击刚才创建的客户端的域名管理，添加两条规则规则：1、域名：`a.proxy.com`，内网目标：`127.0.0.1:81`，2、域名：`b.proxy.com`，内网目标：`127.0.0.1:82`

现在访问（http|https://）`a.proxy.com`，`b.proxy.com`即可成功

**https:** 如需使用https请在配置文件中将https端口设置为443，和将对应的证书文件路径添加到配置文件中，上面添加的这条记录将会把http、https都转发到内网目标

### tcp隧道


**适用范围：**  ssh、远程桌面等tcp连接场景

**假设场景：**
 想通过访问公网服务器1.1.1.1的8001端口，连接内网机器10.1.50.101的22端口，实现ssh连接

**使用步骤**
- 在刚才创建的客户端隧道管理中添加一条tcp隧道，填写监听的端口（8001）、内网目标ip和目标端口（10.1.50.101:22），保存。
- 访问公网服务器ip（1.1.1.1）,填写的监听端口(8001)，相当于访问内网ip(10.1.50.101):目标端口(22)，例如：`ssh -p 8001 root@1.1.1.1`

### udp隧道

**适用范围：**  内网dns解析等udp连接场景

**假设场景：**
内网有一台dns（10.1.50.102:53），在非内网环境下想使用该dns，公网服务器为1.1.1.1

**使用步骤**
- 在刚才创建的客户端的隧道管理中添加一条udp隧道，填写监听的端口（53）、内网目标ip和目标端口（10.1.50.102:53），保存。
- 修改需要使用的内网dns为127.0.0.1，则相当于使用10.1.50.202作为dns服务器

### socks5代理


**适用范围：**  在外网环境下如同使用vpn一样访问内网设备或者资源

**假设场景：**
想将公网服务器1.1.1.1的8003端口作为socks5代理，达到访问内网任意设备或者资源的效果

**使用步骤**
- 在刚才创建的客户端隧道管理中添加一条socks5代理，填写监听的端口（8003），保存。
- 在外网环境的本机配置socks5代理，ip为公网服务器ip（1.1.1.1），端口为填写的监听端口(8003)，即可畅享内网了

### http正向代理

**适用范围：**  在外网环境下使用http正向代理访问内网站点

**假设场景：**
想将公网服务器1.1.1.1的8004端口作为http代理，访问内网网站
**使用步骤**

- 在刚才创建的客户端隧道管理中添加一条http代理，填写监听的端口（8004），保存。
- 在外网环境的本机配置http代理，ip为公网服务器ip（1.1.1.1），端口为填写的监听端口(8004)，即可访问了

### 私密代理

**适用范围：**  无需占用多余的端口、安全性要求较高可以防止其他人连接的tcp服务，例如ssh。

**假设场景：**
无需新增多的端口实现访问内网服务器10.1.50.2的22端口

**使用步骤**
- 在刚才创建的客户端中添加一条私密代理，并设置唯一密钥和内网目标10.1.50.2:22
- 在需要连接ssh的机器上以配置文件模式启动客户端，内容如下

```ini
[common]
server=1.1.1.1:8284
tp=tcp
vkey=123
[secret_ssh]
password=1111
port=1000
```
**注意：** secret前缀必须存在，password为web管理上添加的唯一密钥

假设用户名为root，现在执行`ssh -p 1000 root@127.0.0.1`即可访问ssh

### p2p服务

**适用范围：**  大流量传输场景，流量不经过公网服务器，但是由于p2p穿透和nat类型关系较大，成功率一般，可穿透所有非对称型nat。

**假设场景：**
内网1机器ip为10.1.50.2    内网2机器ip为10.2.50.2

想通过访问机器1的2001端口---->访问到内网2机器的22端口

**使用步骤**
- 在`nps.conf`中设置`p2p_ip`和`p2p_port`
- 在刚才刚才创建的客户端中添加一条p2p代理，并设置唯一密钥p2pssh
- 在需要连接的机器上(即机器1)以配置文件模式启动客户端，内容如下

```ini
[common]
server=1.1.1.1:8284
tp=tcp
vkey=123
[p2p_ssh]
password=p2pssh
port=2001
```
**注意：** p2p前缀必须存在，password为web管理上添加的唯一密钥

假设机器2用户名为root，现在在机器1上执行`ssh -p 2001 root@127.0.0.1`即可访问机器2的ssh




## web管理模式

![image](https://github.com/cnlh/nps/blob/master/image/web2.png?raw=true)
### 介绍

可在网页上配置和管理各个tcp、udp隧道、内网站点代理，http、https解析等，功能强大，操作方便。


**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**

### 启动


#### 服务端测试
```shell
 ./nps test
```
如有错误请及时修改配置文件，无错误可继续进行下去
#### 服务端启动
```shell
 ./nps start
```
如果无需daemon运行，去掉start即可

#### web管理

进入web界面，公网ip:web界面端口（默认8080），密码默认为123

进入web管理界面，有详细的说明

#### 服务端停止或重启
如果是daemon启动
```shell
 ./nps stop|restart
```

### 服务端配置文件
- /conf/nps.conf

名称 | 含义
---|---
web_port | web管理端口
web_password | web界面管理密码
web_username | web界面管理账号
bridge_port  | 服务端客户端通信端口
pem_path | ssl certFile绝对路径
key_path | ssl keyFile绝对路径
https_proxy_port | 域名代理https代理监听端口
http_proxy_port | 域名代理http代理监听端口
auth_key|web api密钥
bridge_type|客户端与服务端连接方式kcp或tcp
public_vkey|客户端以配置文件模式启动时的密钥，设置为空表示关闭客户端配置文件连接模式
ip_limit|是否限制ip访问，true或false或忽略
flow_store_interval|服务端流量数据持久化间隔，单位分钟，忽略表示不持久化
log_level|日志输出级别
auth_crypt_key | 获取服务端authKey时的aes加密密钥，16位
p2p_ip| 服务端Ip，使用p2p模式必填
p2p_port|p2p模式开启的udp端口

### 使用https

**方式一：** 类似于nginx实现https的处理

在配置文件中将https_proxy_port设置为443或者其他你想配置的端口，和将对应的证书文件路径添加到配置文件中，然后就和http代理一样了，例如

- 需要访问`https://a.proxy.com` 对应内网`127.0.0.1:80`

- 在域名代理中添加`a.proxy.com` 内网目标`127.0.0.1:80` 即可将所有到达本代理的http(s)请求都转发到127.0.0.1:80

**方式二：**

在`nps.conf`中将`https_just_proxy`设置为true，并且打开`https_proxy_port`端口，然后nps将直接转发https请求到内网服务器上，由内网服务器进行https处理

### 与nginx配合

有时候我们还需要在云服务器上运行nginx来保证静态文件缓存等，本代理可和nginx配合使用，在配置文件中将httpProxyPort设置为非80端口，并在nginx中配置代理，例如httpProxyPort为8024时
```
server {
    listen 80;
    server_name *.proxy.com;
    location / {
        proxy_set_header Host  $http_host;
        proxy_pass http://127.0.0.1:8024;
    }
}
```
如需使用https也可在nginx监听443端口并配置ssl，并将本代理的httpsProxyPort设置为空关闭https即可，例如httpProxyPort为8024时

```
server {
    listen 443;
    server_name *.proxy.com;
    ssl on;
    ssl_certificate  certificate.crt;
    ssl_certificate_key private.key;
    ssl_session_timeout 5m;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
    ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
    ssl_prefer_server_ciphers on;
    location / {
        proxy_set_header Host  $http_host;
        proxy_pass http://127.0.0.1:8024;
    }
}
```
### 关闭代理

如需关闭http代理可在配置文件中将http_proxy_port设置为空，如需关闭https代理可在配置文件中将https_proxy_port设置为空。

### 将nps安装到系统
如果需要长期并且方便的运行nps服务端，可将nps安装到操作系统中，可执行命令

```
(./nps|nps.exe) install
```
安装成功后，对于linux，darwin，将会把配置文件和静态文件放置于/etc/nps/，并将可执行文件nps复制到/usr/bin/nps或者/usr/local/bin/nps，安装成功后可在任何位置执行

```
nps test|start|stop|restart|status
```
对于windows系统，将会把配置文件和静态文件放置于C:\Program Files\nps，安装成功后可将可执行文件nps.exe复制到任何位置执行

```
nps.exe test|start|stop|restart|status
```

### 流量数据持久化
服务端支持将流量数据持久化，默认情况下是关闭的，如果有需求可以设置`nps.conf`中的`flow_store_interval`参数，单位为分钟

**注意：** nps不会持久化通过公钥连接的客户端

### 自定义客户端连接密钥
web上可以自定义客户端连接的密钥，但是必须具有唯一性
### 关闭公钥访问
可以将`nps.conf`中的`public_vkey`设置为空或者删除

### 关闭web管理
可以将`nps.conf`中的`web_port`设置为空或者删除

## 客户端

### 客户端启动
#### 无配置文件模式
此模式的各种配置在服务端web管理中完成,客户端除运行一条命令外无需任何其他设置
```
 ./npc -server=ip:port -vkey=web界面中显示的密钥
```
#### 配置文件模式
此模式使用nps的公钥或者客户端私钥验证，各种配置在客户端完成，同时服务端web也可以进行管理
```
 ./npc -config=npc配置文件路径
```
#### 配置文件说明
[示例配置文件](https://github.com/cnlh/nps/tree/master/conf/npc.conf)
##### 全局配置
```ini
[common]
server=1.1.1.1:8284
tp=tcp
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
server | 服务端ip:port
tp | 与服务端通信模式(tcp或kcp)
vkey|服务端配置文件中的密钥(非web)
username|socks5或http(s)密码保护用户名(可忽略)
username|socks5或http(s)密码保护密码(可忽略)
compress|是否压缩传输(true或false或忽略)
crypt|是否加密传输(true或false或忽略)
rate_limit|速度限制，可忽略
flow_limit|流量限制，可忽略
remark|客户端备注，可忽略
max_conn|最大连接数，可忽略
##### 域名代理

```ini
[common]
server=1.1.1.1:8284
vkey=123
[web1]
host=a.proxy.com
target=127.0.0.1:8080,127.0.0.1:8082
host_change=www.proxy.com
header_set_proxy=nps
```
项 | 含义
---|---
web1 | 备注
host | 域名(http|https都可解析)
target|内网目标，负载均衡时多个目标，逗号隔开
host_change|请求host修改
header_xxx|请求header修改或添加，header_proxy表示添加header proxy:nps

##### tcp隧道模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[tcp]
mode=tcp
target=127.0.0.1:8080
port=9001
```
项 | 含义
---|---
mode | tcp
port | 在服务端的代理端口
target|内网目标

##### udp隧道模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[udp]
mode=udp
target=127.0.0.1:8080
port=9002
```
项 | 含义
---|---
mode | udp
port | 在服务端的代理端口
target|内网目标
##### http代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[http]
mode=httpProxy
port=9003
```
项 | 含义
---|---
mode | httpProxy
port | 在服务端的代理端口
##### socks5代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[socks5]
mode=socks5
port=9004
```
项 | 含义
---|---
mode | socks5
port | 在服务端的代理端口
##### 私密代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[secret_ssh]
mode=secret
password=ssh2
target=10.1.50.2:22
```
项 | 含义
---|---
mode | secret
password | 唯一密钥
target|内网目标

##### p2p代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[p2p_ssh]
mode=p2p
password=ssh2
target=10.1.50.2:22
```
项 | 含义
---|---
mode | p2p
password | 唯一密钥
target|内网目标


##### 文件访问模式
利用nps提供一个公网可访问的本地文件服务，此模式仅客户端使用配置文件模式方可启动

```ini
[common]
server=1.1.1.1:8284
vkey=123
[file]
mode=file
port=9100
local_path=/tmp/
strip_pre=/web/
````

项 | 含义
---|---
mode | file
port | 服务端开启的端口
local_path|本地文件目录
strip_pre|前缀

对于`strip_pre`，访问公网`ip:9100/web/`相当于访问`/tmp/`目录

#### 断线重连
```ini
[common]
auto_reconnection=true
```

#### 状态检查
```
 ./npc status -config=npc配置文件路径
```
#### 重载配置文件
```
 ./npc restart -config=npc配置文件路径
```

#### 通过代理连接nps
有时候运行npc的内网机器无法直接访问外网，此时可以可以通过socks5代理连接nps

对于配置文件方式启动,设置
```ini
[common]
proxy_socks5_url=socks5://111:222@127.0.0.1:8024
```
对于无配置文件模式,加上参数

```
-proxy=socks5://111:222@127.0.0.1:8024
```
即socks5://username:password@ip:port

#### 日志输出级别
```
-log_level=0~7
```
```
LevelEmergency->0  LevelAlert->1

LevelCritical->2 LevelError->3

LevelWarning->4 LevelNotice->5

LevelInformational->6 LevelDebug->7
```
默认为全输出,级别为0到7
## 相关功能

### 数据压缩支持

由于是内网穿透，内网客户端与服务端之间的隧道存在大量的数据交换，为节省流量，加快传输速度，由此本程序支持SNNAPY形式的压缩。


- 所有模式均支持数据压缩
- 在web管理或客户端配置文件中设置


### 加密传输

如果公司内网防火墙对外网访问进行了流量识别与屏蔽，例如禁止了ssh协议等，通过设置 配置文件，将服务端与客户端之间的通信内容加密传输，将会有效防止流量被拦截。
- nps使用tls加密，所以一定要保留conf目录下的密钥文件，同时也可以自行生成
- 在web管理或客户端配置文件中设置



### 站点保护
域名代理模式所有客户端共用一个http服务端口，在知道域名后任何人都可访问，一些开发或者测试环境需要保密，所以可以设置用户名和密码，nps将通过 Http Basic Auth 来保护，访问时需要输入正确的用户名和密码。


- 在web管理或客户端配置文件中设置

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

### 负载均衡
本代理支持域名解析模式和tcp代理的负载均衡，在web域名添加或者编辑中内网目标分行填写多个目标即可实现轮训级别的负载均衡

### 端口白名单
为了防止服务端上的端口被滥用，可在nps.conf中配置allow_ports限制可开启的端口，忽略或者不填表示端口不受限制，格式：

```ini
allow_ports=9001-9009,10001,11000-12000
```

### 端口范围映射
当客户端以配置文件的方式启动时，可以将本地的端口进行范围映射，仅支持tcp和udp模式，例如：

```ini
[tcp]
mode=tcp
port=9001-9009,10001,11000-12000
target=8001-8009,10002,13000-14000
```

逗号分隔，可单个或者范围，注意上下端口的对应关系，无法一一对应将不能成功
### 端口范围映射到其他机器
```ini
[tcp]
mode=tcp
port=9001-9009,10001,11000-12000
target=8001-8009,10002,13000-14000
targetAddr=10.1.50.2
```
填写targetAddr后则表示映射的该地址机器的端口，忽略则便是映射本地127.0.0.1,仅范围映射时有效
### 守护进程
本代理支持守护进程，使用示例如下，服务端客户端所有模式通用,支持linux，darwin，windows。
```
./(nps|npc) start|stop|restart|status 若有其他参数可加其他参数
```
```
(nps|npc).exe start|stop|restart|status 若有其他参数可加其他参数
```
### KCP协议支持

KCP 是一个快速可靠协议，能以比 TCP浪费10%-20%的带宽的代价，换取平均延迟降低 30%-40%，在弱网环境下对性能能有一定的提升。可在nps.conf中修改`bridge_type`为kcp
，设置后本代理将开启udp端口（`bridge_port`）

注意：当服务端为kcp时，客户端连接时也需要使用相同配置，无配置文件模式加上参数type=kcp,配置文件模式在配置文件中设置tp=kcp

### 域名泛解析
支持域名泛解析，例如将host设置为*.proxy.com，a.proxy.com、b.proxy.com等都将解析到同一目标，在web管理中或客户端配置文件中将host设置为此格式即可。

### URL路由
本代理支持根据URL将同一域名转发到不同的内网服务器，可在web中或客户端配置文件中设置，此参数也可忽略，例如在客户端配置文件中

```ini
[web1]
host=a.proxy.com
target=127.0.0.1:7001
location=/test
[web2]
host=a.proxy.com
target=127.0.0.1:7002
location=/static
```
对于`a.proxy.com/test`将转发到`web1`，对于`a.proxy.com/static`将转发到`web2`

### 限制ip访问
如果将一些危险性高的端口例如ssh端口暴露在公网上，可能会带来一些风险，本代理支持限制ip访问。

**使用方法:** 在配置文件nps.conf中设置`ip_limit`=true，设置后仅通过注册的ip方可访问。

**ip注册**： 在需要访问的机器上，运行客户端

```
./npc register -server=ip:port -vkey=公钥或客户端密钥 time=2
```

time为有效小时数，例如time=2，在当前时间后的两小时内，本机公网ip都可以访问nps代理.

**注意：** 本机公网ip并不是一成不变的，请自行注意有效期的设置，同时同一网络下，多人也可能是在公用同一个公网ip。
### 客户端最大连接数
为防止恶意大量长连接，影响服务端程序的稳定性，可以在web或客户端配置文件中为每个客户端设置最大连接数。该功能针对`socks5`、`http正向代理`、`域名代理`、`tcp代理`、`私密代理`生效。

### 端口复用
在一些严格的网络环境中，对端口的个数等限制较大，nps支持强大端口复用功能。将`bridge_port`、 `http_proxy_port`、 `https_proxy_port` 、`web_port`都设置为同一端口，也能正常使用。

- 使用时将需要复用的端口设置为与`bridge_port`一致即可，将自动识别。
- 如需将web管理的端口也复用，需要配置`web_host`也就是一个二级域名以便区分

### 环境变量渲染
npc支持环境变量渲染以适应在某些特殊场景下的要求。

**在无配置文件启动模式下：**
设置环境变量
```
export NPC_SERVER_ADDR=1.1.1.1:8284
export NPC_SERVER_VKEY=xxxxx
```
直接执行./npc即可运行

**在配置文件启动模式下：**
```ini
[common]
server={{.NPC_SERVER_ADDR}}
tp=tcp
vkey={{.NPC_SERVER_VKEY}}
auto_reconnection=true
[web]
host={{.NPC_WEB_HOST}}
target={{.NPC_WEB_TARGET}}
```
在配置文件中填入相应的环境变量名称，npc将自动进行渲染配置文件替换环境变量

### 健康检查

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

### 客户端与服务端版本对比
为了程序正常运行，客户端与服务端的版本必须一致，否则将导致客户端无法成功连接致服务端。

## 简单的性能测试

### qps
![image](https://github.com/cnlh/nps/blob/master/image/qps.png?raw=true)
### 速度测试
**测试环境：** 1M带宽云服务器，理论125kb/s，带宽与代理无关，与服务器带宽和内网客户端外网带宽关系较大。
![image](https://github.com/cnlh/nps/blob/master/image/speed.png?raw=true)


### 内存和cpu

**1000次性能测试后**
![image](https://github.com/cnlh/nps/blob/master/image/cpu1.png?raw=true)

**启动时**
![image](https://github.com/cnlh/nps/blob/master/image/cpu2.png?raw=true)

### 额外消耗连接数
为了最大化的提升效率和并发，客户端与服务端之间仅两条tcp连接，减少建立连接的时间消耗和多余socket连接对机器性能的影响。

## webAPI

### webAPI验证说明
- 采用auth_key的验证方式
- 在提交的每个请求后面附带两个参数，`auth_key` 和`timestamp`

```
auth_key的生成方式为：md5(配置文件中的auth_key+当前时间戳)
```

```
timestamp为当前时间戳
```

**注意：** 为保证安全，时间戳的有效范围为20秒内，所以每次提交请求必须重新生成。

### 获取服务端authKey

如果想获取authKey，服务端提供获取authKey的接口

```
POST /auth/getauthkey
```
将返回加密后的authKey，采用aes cbc加密，请使用与服务端配置文件中cryptKey相同的密钥进行解密


### 详细文档
- 此文档近期可能更新较慢，建议自行抓包

为方便第三方扩展，在web模式下可利用webAPI进行相关操作，详情见
[webAPI文档](https://github.com/cnlh/nps/wiki/webAPI%E6%96%87%E6%A1%A3)

## 贡献
#### **欢迎参与到制作docker、图标、文档翻译等工作**
- 如果遇到bug可以直接提交至dev分支
- 使用遇到问题可以通过issues反馈
- 项目处于开发阶段，还有很多待完善的地方，如果可以贡献代码，请提交 PR 至 dev 分支
- 如果有新的功能特性反馈，可以通过issues或者qq群反馈



## 交流群

![二维码.jpeg](https://i.loli.net/2019/02/15/5c66c32a42074.jpeg)
# nps
![](https://img.shields.io/github/stars/cnlh/nps.svg)   ![](https://img.shields.io/github/forks/cnlh/nps.svg) ![](https://img.shields.io/github/license/cnlh/nps.svg)

nps是一款轻量级、高性能、功能强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何**tcp、udp**上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**、**p2p等**，并带有功能强大的web管理端。


## 背景
![image](https://github.com/cnlh/nps/blob/master/image/web.png?raw=true)

1. 做微信公众号开发、小程序开发等----> 域名代理模式


2. 想在外网通过ssh连接内网的机器，做云服务器到内网服务器端口的映射，----> tcp代理模式

3. 在非内网环境下使用内网dns，或者需要通过udp访问内网机器等----> udp代理模式

4. 在外网使用HTTP代理访问内网站点----> http代理模式

5. 搭建一个内网穿透ss，在外网如同使用内网vpn一样访问内网资源或者设备----> socks5代理模式


## 目录

* [安装](#安装)
    * [编译安装](#源码安装)
    * [release安装](#release安装)
* [使用示例（以web主控模式为主）](#使用示例)
    * [统一准备工作](#统一准备工作(必做))
    * [http|https域名解析](#域名解析)
    * [内网ssh连接即tcp隧道](#tcp隧道)
    * [内网dns解析即udp隧道](#udp隧道)
    * [内网socks5代理](#socks5代理)
    * [内网http正向代理](#http正向代理)
    * [内网安全私密代理](#私密代理)
    * [p2p穿透](#p2p服务)
    * [简单的内网文件访问服务](#文件访问模式)
* [服务端](#web管理模式)
    * [服务端启动](#服务端启动)
       * [服务端测试](#服务端测试)
       * [服务端启动](#服务端启动)
       * [web管理](#web管理)
       * [服务端停止或重启](#服务端停止或重启)
    * [配置文件说明](#服务端配置文件)

    * [使用https](#使用https)
    * [与nginx配合](#与nginx配合)
    * [关闭http|https代理](#关闭代理)
    * [将nps安装到系统](#将nps安装到系统)
    * [流量数据持久化](#流量数据持久化)
    * [自定义客户端连接密钥](#自定义客户端连接密钥)
    * [关闭公钥访问](#关闭公钥访问)
    * [关闭web管理](#关闭web管理)
* [客户端](#客户端)
    * [客户端启动](#客户端启动)
        * [无配置文件模式](#无配置文件模式)
        * [配置文件模式](#配置文件模式)
    * [配置文件说明](#配置文件说明)
        * [全局配置](#全局配置)
        * [域名代理](#域名代理)
        * [tcp隧道](#tcp隧道模式)
        * [udp隧道](#udp隧道模式)
        * [http正向代理](#http代理模式)
        * [socks5代理](#socks5代理模式)
        * [私密代理](#私密代理模式)
        * [p2p服务](#p2p代理)
        * [文件访问代理](#文件访问模式)
    * [断线重连](#断线重连)
    * [状态检查](#状态检查)
    * [重载配置文件](#重载配置文件)
    * [通过代理连接nps](#通过代理连接nps)
    * [日志输出级别](#日志输出级别)

* [相关功能](#相关功能)
   * [数据压缩支持](#数据压缩支持)
   * [站点密码保护](#站点保护)
   * [加密传输](#加密传输)
   * [host修改](#host修改)
   * [自定义header](#自定义header)
   * [自定义404页面](#404页面配置)
   * [流量限制](#流量限制)
   * [带宽限制](#带宽限制)
   * [负载均衡](#负载均衡)
   * [端口白名单](#端口白名单)
   * [端口范围映射](#端口范围映射)
   * [端口范围映射到其他机器](#端口范围映射到其他机器)
   * [守护进程](#守护进程)
   * [KCP协议支持](#KCP协议支持)
   * [域名泛解析](#域名泛解析)
   * [URL路由](#URL路由)
   * [限制ip访问](#限制ip访问)
   * [客户端最大连接数限制](#客户端最大连接数)
   * [端口复用](#端口复用)
   * [环境变量渲染](#环境变量渲染)
   * [健康检查](#健康检查)

* [相关说明](#相关说明)
   * [流量统计](#流量统计)
   * [热更新支持](#热更新支持)
   * [获取用户真实ip](#获取用户真实ip)
   * [客户端地址显示](#客户端地址显示)
   * [客户端与服务端版本对比](#客户端与服务端版本对比)
* [简单的性能测试](#简单的性能测试)
   * [qps](#qps)
   * [速度测试](#速度测试)
   * [内存和cpu](#内存和cpu)
   * [额外消耗连接数](#额外消耗连接数)
* [webAPI](#webAPI)
* [贡献](#贡献)
* [交流群](#交流群)



## 安装

### release安装
> https://github.com/cnlh/nps/releases

下载对应的系统版本即可，服务端和客户端是单独的，go语言开发，无需任何第三方依赖

### 源码安装
- 安装源码
> go get -u github.com/cnlh/nps...
- 编译
> go build cmd/nps/nps.go

> go build cmd/npc/npc.go

## 使用示例

### 统一准备工作（必做）
- 开启服务端，假设公网服务器ip为1.1.1.1，配置文件中`bridgePort`为8284，配置文件中`web_port`为8080
- 访问1.1.1.1:8080
- 在客户端管理中创建一个客户端，记录下验证密钥
- 内网客户端运行（windows使用cmd运行加.exe）

```shell
./npc -server=1.1.1.1:8284 -vkey=客户端的密钥
```

### 域名解析

**适用范围：** 小程序开发、微信公众号开发、产品演示

**假设场景：**
- 有一个域名proxy.com，有一台公网机器ip为1.1.1.1
- 两个内网开发站点127.0.0.1:81，127.0.0.1:82
- 想通过（http|https://）a.proxy.com访问127.0.0.1:81，通过（http|https://）b.proxy.com访问127.0.0.1:82

**使用步骤**
- 将*.proxy.com解析到公网服务器1.1.1.1
- 点击刚才创建的客户端的域名管理，添加两条规则规则：1、域名：`a.proxy.com`，内网目标：`127.0.0.1:81`，2、域名：`b.proxy.com`，内网目标：`127.0.0.1:82`

现在访问（http|https://）`a.proxy.com`，`b.proxy.com`即可成功

**https:** 如需使用https请在配置文件中将https端口设置为443，和将对应的证书文件路径添加到配置文件中，上面添加的这条记录将会把http、https都转发到内网目标

### tcp隧道


**适用范围：**  ssh、远程桌面等tcp连接场景

**假设场景：**
 想通过访问公网服务器1.1.1.1的8001端口，连接内网机器10.1.50.101的22端口，实现ssh连接

**使用步骤**
- 在刚才创建的客户端隧道管理中添加一条tcp隧道，填写监听的端口（8001）、内网目标ip和目标端口（10.1.50.101:22），保存。
- 访问公网服务器ip（1.1.1.1）,填写的监听端口(8001)，相当于访问内网ip(10.1.50.101):目标端口(22)，例如：`ssh -p 8001 root@1.1.1.1`

### udp隧道

**适用范围：**  内网dns解析等udp连接场景

**假设场景：**
内网有一台dns（10.1.50.102:53），在非内网环境下想使用该dns，公网服务器为1.1.1.1

**使用步骤**
- 在刚才创建的客户端的隧道管理中添加一条udp隧道，填写监听的端口（53）、内网目标ip和目标端口（10.1.50.102:53），保存。
- 修改需要使用的内网dns为127.0.0.1，则相当于使用10.1.50.202作为dns服务器

### socks5代理


**适用范围：**  在外网环境下如同使用vpn一样访问内网设备或者资源

**假设场景：**
想将公网服务器1.1.1.1的8003端口作为socks5代理，达到访问内网任意设备或者资源的效果

**使用步骤**
- 在刚才创建的客户端隧道管理中添加一条socks5代理，填写监听的端口（8003），保存。
- 在外网环境的本机配置socks5代理，ip为公网服务器ip（1.1.1.1），端口为填写的监听端口(8003)，即可畅享内网了

### http正向代理

**适用范围：**  在外网环境下使用http正向代理访问内网站点

**假设场景：**
想将公网服务器1.1.1.1的8004端口作为http代理，访问内网网站
**使用步骤**

- 在刚才创建的客户端隧道管理中添加一条http代理，填写监听的端口（8004），保存。
- 在外网环境的本机配置http代理，ip为公网服务器ip（1.1.1.1），端口为填写的监听端口(8004)，即可访问了

### 私密代理

**适用范围：**  无需占用多余的端口、安全性要求较高可以防止其他人连接的tcp服务，例如ssh。

**假设场景：**
无需新增多的端口实现访问内网服务器10.1.50.2的22端口

**使用步骤**
- 在刚才创建的客户端中添加一条私密代理，并设置唯一密钥和内网目标10.1.50.2:22
- 在需要连接ssh的机器上以配置文件模式启动客户端，内容如下

```ini
[common]
server=1.1.1.1:8284
tp=tcp
vkey=123
[secret_ssh]
password=1111
port=1000
```
**注意：** secret前缀必须存在，password为web管理上添加的唯一密钥

假设用户名为root，现在执行`ssh -p 1000 root@127.0.0.1`即可访问ssh

### p2p服务

**适用范围：**  大流量传输场景，流量不经过公网服务器，但是由于p2p穿透和nat类型关系较大，成功率一般，可穿透所有非对称型nat。

**假设场景：**
内网1机器ip为10.1.50.2    内网2机器ip为10.2.50.2

想通过访问机器1的2001端口---->访问到内网2机器的22端口

**使用步骤**
- 在`nps.conf`中设置`p2p_ip`和`p2p_port`
- 在刚才刚才创建的客户端中添加一条p2p代理，并设置唯一密钥p2pssh
- 在需要连接的机器上(即机器1)以配置文件模式启动客户端，内容如下

```ini
[common]
server=1.1.1.1:8284
tp=tcp
vkey=123
[p2p_ssh]
password=p2pssh
port=2001
```
**注意：** p2p前缀必须存在，password为web管理上添加的唯一密钥

假设机器2用户名为root，现在在机器1上执行`ssh -p 2001 root@127.0.0.1`即可访问机器2的ssh




## web管理模式

![image](https://github.com/cnlh/nps/blob/master/image/web2.png?raw=true)
### 介绍

可在网页上配置和管理各个tcp、udp隧道、内网站点代理，http、https解析等，功能强大，操作方便。


**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**

### 启动


#### 服务端测试
```shell
 ./nps test
```
如有错误请及时修改配置文件，无错误可继续进行下去
#### 服务端启动
```shell
 ./nps start
```
如果无需daemon运行，去掉start即可

#### web管理

进入web界面，公网ip:web界面端口（默认8080），密码默认为123

进入web管理界面，有详细的说明

#### 服务端停止或重启
如果是daemon启动
```shell
 ./nps stop|restart
```

### 服务端配置文件
- /conf/nps.conf

名称 | 含义
---|---
web_port | web管理端口
web_password | web界面管理密码
web_username | web界面管理账号
bridge_port  | 服务端客户端通信端口
pem_path | ssl certFile绝对路径
key_path | ssl keyFile绝对路径
https_proxy_port | 域名代理https代理监听端口
http_proxy_port | 域名代理http代理监听端口
auth_key|web api密钥
bridge_type|客户端与服务端连接方式kcp或tcp
public_vkey|客户端以配置文件模式启动时的密钥，设置为空表示关闭客户端配置文件连接模式
ip_limit|是否限制ip访问，true或false或忽略
flow_store_interval|服务端流量数据持久化间隔，单位分钟，忽略表示不持久化
log_level|日志输出级别
auth_crypt_key | 获取服务端authKey时的aes加密密钥，16位
p2p_ip| 服务端Ip，使用p2p模式必填
p2p_port|p2p模式开启的udp端口

### 使用https

在配置文件中将httpsProxyPort设置为443或者其他你想配置的端口，和将对应的证书文件路径添加到配置文件中，然后就和http代理一样了，例如

- 需要访问`https://a.proxy.com` 对应内网`127.0.0.1:80`

- 在域名代理中添加`a.proxy.com` 内网目标`127.0.0.1:80` 即可将所有到达本代理的http(s)请求都转发到127.0.0.1:80

### 与nginx配合

有时候我们还需要在云服务器上运行nginx来保证静态文件缓存等，本代理可和nginx配合使用，在配置文件中将httpProxyPort设置为非80端口，并在nginx中配置代理，例如httpProxyPort为8024时
```
server {
    listen 80;
    server_name *.proxy.com;
    location / {
        proxy_set_header Host  $http_host;
        proxy_pass http://127.0.0.1:8024;
    }
}
```
如需使用https也可在nginx监听443端口并配置ssl，并将本代理的httpsProxyPort设置为空关闭https即可，例如httpProxyPort为8024时

```
server {
    listen 443;
    server_name *.proxy.com;
    ssl on;
    ssl_certificate  certificate.crt;
    ssl_certificate_key private.key;
    ssl_session_timeout 5m;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
    ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
    ssl_prefer_server_ciphers on;
    location / {
        proxy_set_header Host  $http_host;
        proxy_pass http://127.0.0.1:8024;
    }
}
```
### 关闭代理

如需关闭http代理可在配置文件中将http_proxy_port设置为空，如需关闭https代理可在配置文件中将https_proxy_port设置为空。

### 将nps安装到系统
如果需要长期并且方便的运行nps服务端，可将nps安装到操作系统中，可执行命令

```
(./nps|nps.exe) install
```
安装成功后，对于linux，darwin，将会把配置文件和静态文件放置于/etc/nps/，并将可执行文件nps复制到/usr/bin/nps或者/usr/local/bin/nps，安装成功后可在任何位置执行

```
nps test|start|stop|restart|status
```
对于windows系统，将会把配置文件和静态文件放置于C:\Program Files\nps，安装成功后可将可执行文件nps.exe复制到任何位置执行

```
nps.exe test|start|stop|restart|status
```

### 流量数据持久化
服务端支持将流量数据持久化，默认情况下是关闭的，如果有需求可以设置`nps.conf`中的`flow_store_interval`参数，单位为分钟

**注意：** nps不会持久化通过公钥连接的客户端

### 自定义客户端连接密钥
web上可以自定义客户端连接的密钥，但是必须具有唯一性
### 关闭公钥访问
可以将`nps.conf`中的`public_vkey`设置为空或者删除

### 关闭web管理
可以将`nps.conf`中的`web_port`设置为空或者删除

## 客户端

### 客户端启动
#### 无配置文件模式
此模式的各种配置在服务端web管理中完成,客户端除运行一条命令外无需任何其他设置
```
 ./npc -server=ip:port -vkey=web界面中显示的密钥
```
#### 配置文件模式
此模式使用nps的公钥或者客户端私钥验证，各种配置在客户端完成，同时服务端web也可以进行管理
```
 ./npc -config=npc配置文件路径
```
#### 配置文件说明
[示例配置文件](https://github.com/cnlh/nps/tree/master/conf/npc.conf)
##### 全局配置
```ini
[common]
server=1.1.1.1:8284
tp=tcp
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
server | 服务端ip:port
tp | 与服务端通信模式(tcp或kcp)
vkey|服务端配置文件中的密钥(非web)
username|socks5或http(s)密码保护用户名(可忽略)
username|socks5或http(s)密码保护密码(可忽略)
compress|是否压缩传输(true或false或忽略)
crypt|是否加密传输(true或false或忽略)
rate_limit|速度限制，可忽略
flow_limit|流量限制，可忽略
remark|客户端备注，可忽略
max_conn|最大连接数，可忽略
##### 域名代理

```ini
[common]
server=1.1.1.1:8284
vkey=123
[web1]
host=a.proxy.com
target=127.0.0.1:8080,127.0.0.1:8082
host_change=www.proxy.com
header_set_proxy=nps
```
项 | 含义
---|---
web1 | 备注
host | 域名(http|https都可解析)
target|内网目标，负载均衡时多个目标，逗号隔开
host_change|请求host修改
header_xxx|请求header修改或添加，header_proxy表示添加header proxy:nps

##### tcp隧道模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[tcp]
mode=tcp
target=127.0.0.1:8080
port=9001
```
项 | 含义
---|---
mode | tcp
port | 在服务端的代理端口
target|内网目标

##### udp隧道模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[udp]
mode=udp
target=127.0.0.1:8080
port=9002
```
项 | 含义
---|---
mode | udp
port | 在服务端的代理端口
target|内网目标
##### http代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[http]
mode=httpProxy
port=9003
```
项 | 含义
---|---
mode | httpProxy
port | 在服务端的代理端口
##### socks5代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[socks5]
mode=socks5
port=9004
```
项 | 含义
---|---
mode | socks5
port | 在服务端的代理端口
##### 私密代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[secret_ssh]
mode=secret
password=ssh2
target=10.1.50.2:22
```
项 | 含义
---|---
mode | secret
password | 唯一密钥
target|内网目标

##### p2p代理模式

```ini
[common]
server=1.1.1.1:8284
vkey=123
[p2p_ssh]
mode=p2p
password=ssh2
target=10.1.50.2:22
```
项 | 含义
---|---
mode | p2p
password | 唯一密钥
target|内网目标


##### 文件访问模式
利用nps提供一个公网可访问的本地文件服务，此模式仅客户端使用配置文件模式方可启动

```ini
[common]
server=1.1.1.1:8284
vkey=123
[file]
mode=file
port=9100
local_path=/tmp/
strip_pre=/web/
````

项 | 含义
---|---
mode | file
port | 服务端开启的端口
local_path|本地文件目录
strip_pre|前缀

对于`strip_pre`，访问公网`ip:9100/web/`相当于访问`/tmp/`目录

#### 断线重连
```ini
[common]
auto_reconnection=true
```

#### 状态检查
```
 ./npc status -config=npc配置文件路径
```
#### 重载配置文件
```
 ./npc restart -config=npc配置文件路径
```

#### 通过代理连接nps
有时候运行npc的内网机器无法直接访问外网，此时可以可以通过socks5代理连接nps

对于配置文件方式启动,设置
```ini
[common]
proxy_socks5_url=socks5://111:222@127.0.0.1:8024
```
对于无配置文件模式,加上参数

```
-proxy=socks5://111:222@127.0.0.1:8024
```
即socks5://username:password@ip:port

#### 日志输出级别
```
-log_level=0~7
```
```
LevelEmergency->0  LevelAlert->1

LevelCritical->2 LevelError->3

LevelWarning->4 LevelNotice->5

LevelInformational->6 LevelDebug->7
```
默认为全输出,级别为0到7
## 相关功能

### 数据压缩支持

由于是内网穿透，内网客户端与服务端之间的隧道存在大量的数据交换，为节省流量，加快传输速度，由此本程序支持SNNAPY形式的压缩。


- 所有模式均支持数据压缩
- 在web管理或客户端配置文件中设置


### 加密传输

如果公司内网防火墙对外网访问进行了流量识别与屏蔽，例如禁止了ssh协议等，通过设置 配置文件，将服务端与客户端之间的通信内容加密传输，将会有效防止流量被拦截。
- nps使用tls加密，所以一定要保留conf目录下的密钥文件，同时也可以自行生成
- 在web管理或客户端配置文件中设置



### 站点保护
域名代理模式所有客户端共用一个http服务端口，在知道域名后任何人都可访问，一些开发或者测试环境需要保密，所以可以设置用户名和密码，nps将通过 Http Basic Auth 来保护，访问时需要输入正确的用户名和密码。


- 在web管理或客户端配置文件中设置

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

### 负载均衡
本代理支持域名解析模式和tcp代理的负载均衡，在web域名添加或者编辑中内网目标分行填写多个目标即可实现轮训级别的负载均衡

### 端口白名单
为了防止服务端上的端口被滥用，可在nps.conf中配置allow_ports限制可开启的端口，忽略或者不填表示端口不受限制，格式：

```ini
allow_ports=9001-9009,10001,11000-12000
```

### 端口范围映射
当客户端以配置文件的方式启动时，可以将本地的端口进行范围映射，仅支持tcp和udp模式，例如：

```ini
[tcp]
mode=tcp
port=9001-9009,10001,11000-12000
target=8001-8009,10002,13000-14000
```

逗号分隔，可单个或者范围，注意上下端口的对应关系，无法一一对应将不能成功
### 端口范围映射到其他机器
```ini
[tcp]
mode=tcp
port=9001-9009,10001,11000-12000
target=8001-8009,10002,13000-14000
targetAddr=10.1.50.2
```
填写targetAddr后则表示映射的该地址机器的端口，忽略则便是映射本地127.0.0.1,仅范围映射时有效
### 守护进程
本代理支持守护进程，使用示例如下，服务端客户端所有模式通用,支持linux，darwin，windows。
```
./(nps|npc) start|stop|restart|status 若有其他参数可加其他参数
```
```
(nps|npc).exe start|stop|restart|status 若有其他参数可加其他参数
```
### KCP协议支持

KCP 是一个快速可靠协议，能以比 TCP浪费10%-20%的带宽的代价，换取平均延迟降低 30%-40%，在弱网环境下对性能能有一定的提升。可在nps.conf中修改`bridge_type`为kcp
，设置后本代理将开启udp端口（`bridge_port`）

注意：当服务端为kcp时，客户端连接时也需要使用相同配置，无配置文件模式加上参数type=kcp,配置文件模式在配置文件中设置tp=kcp

### 域名泛解析
支持域名泛解析，例如将host设置为*.proxy.com，a.proxy.com、b.proxy.com等都将解析到同一目标，在web管理中或客户端配置文件中将host设置为此格式即可。

### URL路由
本代理支持根据URL将同一域名转发到不同的内网服务器，可在web中或客户端配置文件中设置，此参数也可忽略，例如在客户端配置文件中

```ini
[web1]
host=a.proxy.com
target=127.0.0.1:7001
location=/test
[web2]
host=a.proxy.com
target=127.0.0.1:7002
location=/static
```
对于`a.proxy.com/test`将转发到`web1`，对于`a.proxy.com/static`将转发到`web2`

### 限制ip访问
如果将一些危险性高的端口例如ssh端口暴露在公网上，可能会带来一些风险，本代理支持限制ip访问。

**使用方法:** 在配置文件nps.conf中设置`ip_limit`=true，设置后仅通过注册的ip方可访问。

**ip注册**： 在需要访问的机器上，运行客户端

```
./npc register -server=ip:port -vkey=公钥或客户端密钥 time=2
```

time为有效小时数，例如time=2，在当前时间后的两小时内，本机公网ip都可以访问nps代理.

**注意：** 本机公网ip并不是一成不变的，请自行注意有效期的设置，同时同一网络下，多人也可能是在公用同一个公网ip。
### 客户端最大连接数
为防止恶意大量长连接，影响服务端程序的稳定性，可以在web或客户端配置文件中为每个客户端设置最大连接数。该功能针对`socks5`、`http正向代理`、`域名代理`、`tcp代理`、`私密代理`生效。

### 端口复用
在一些严格的网络环境中，对端口的个数等限制较大，nps支持强大端口复用功能。将`bridge_port`、 `http_proxy_port`、 `https_proxy_port` 、`web_port`都设置为同一端口，也能正常使用。

- 使用时将需要复用的端口设置为与`bridge_port`一致即可，将自动识别。
- 如需将web管理的端口也复用，需要配置`web_host`也就是一个二级域名以便区分

### 环境变量渲染
npc支持环境变量渲染以适应在某些特殊场景下的要求。

**在无配置文件启动模式下：**
设置环境变量
```
export NPC_SERVER_ADDR=1.1.1.1:8284
export NPC_SERVER_VKEY=xxxxx
```
直接执行./npc即可运行

**在配置文件启动模式下：**
```ini
[common]
server={{.NPC_SERVER_ADDR}}
tp=tcp
vkey={{.NPC_SERVER_VKEY}}
auto_reconnection=true
[web]
host={{.NPC_WEB_HOST}}
target={{.NPC_WEB_TARGET}}
```
在配置文件中填入相应的环境变量名称，npc将自动进行渲染配置文件替换环境变量

### 健康检查

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

### 客户端与服务端版本对比
为了程序正常运行，客户端与服务端的版本必须一致，否则将导致客户端无法成功连接致服务端。

## 简单的性能测试

### qps
![image](https://github.com/cnlh/nps/blob/master/image/qps.png?raw=true)
### 速度测试
**测试环境：** 1M带宽云服务器，理论125kb/s，带宽与代理无关，与服务器带宽和内网客户端外网带宽关系较大。
![image](https://github.com/cnlh/nps/blob/master/image/speed.png?raw=true)


### 内存和cpu

**1000次性能测试后**
![image](https://github.com/cnlh/nps/blob/master/image/cpu1.png?raw=true)

**启动时**
![image](https://github.com/cnlh/nps/blob/master/image/cpu2.png?raw=true)

### 额外消耗连接数
为了最大化的提升效率和并发，客户端与服务端之间仅两条tcp连接，减少建立连接的时间消耗和多余socket连接对机器性能的影响。

## webAPI

### webAPI验证说明
- 采用auth_key的验证方式
- 在提交的每个请求后面附带两个参数，`auth_key` 和`timestamp`

```
auth_key的生成方式为：md5(配置文件中的auth_key+当前时间戳)
```

```
timestamp为当前时间戳
```

**注意：** 为保证安全，时间戳的有效范围为20秒内，所以每次提交请求必须重新生成。

### 获取服务端authKey

如果想获取authKey，服务端提供获取authKey的接口

```
POST /auth/getauthkey
```
将返回加密后的authKey，采用aes cbc加密，请使用与服务端配置文件中cryptKey相同的密钥进行解密

**注意：** nps配置文件中`auth_crypt_key`需为16位
- 解密密钥长度128
- 偏移量与密钥相同
- 补码方式pkcs5padding
- 解密串编码方式 十六进制

### 详细文档
- 此文档近期可能更新较慢，建议自行抓包

为方便第三方扩展，在web模式下可利用webAPI进行相关操作，详情见
[webAPI文档](https://github.com/cnlh/nps/wiki/webAPI%E6%96%87%E6%A1%A3)

## 贡献
#### **欢迎参与到制作docker、图标、文档翻译等工作**
- 如果遇到bug可以直接提交至dev分支
- 使用遇到问题可以通过issues反馈
- 项目处于开发阶段，还有很多待完善的地方，如果可以贡献代码，请提交 PR 至 dev 分支
- 如果有新的功能特性反馈，可以通过issues或者qq群反馈



## 交流群

![二维码.jpeg](https://i.loli.net/2019/02/15/5c66c32a42074.jpeg)

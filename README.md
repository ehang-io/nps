# nps
![](https://img.shields.io/github/stars/cnlh/nps.svg)   ![](https://img.shields.io/github/forks/cnlh/nps.svg) ![](https://img.shields.io/github/license/cnlh/nps.svg)

nps是一款轻量级、高性能、功能最为强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何tcp、udp上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**，可实现在非内网环境下如同使用vpn一样访问内网资源和设备的效果。

目前市面上提供类似服务的有花生壳、TeamView、GoToMyCloud等等，但要使用第三方的公网服务器就必须为第三方付费，并且这些服务都有各种各样的限制，此外，由于数据包会流经第三方，因此对数据安全也是一大隐患。


go语言编写，无第三方依赖，各个平台都已经编译在release中。

## 背景
![image](https://github.com/cnlh/nps/blob/master/image/web.png?raw=true)
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
    * [快速启动](#启动)
       * [服务端测试](#服务端测试)
       * [服务端启动](#服务端启动)
       * [web管理](#web管理)
       * [客户端启动](#客户端启动)
       * [服务端停止或重启](#服务端停止或重启)
    * [配置文件说明](#服务端配置文件)
    * [详细使用说明](#详细说明)
       * [http|https域名解析](#域名解析)
       * [tcp隧道](#tcp隧道)
       * [udp隧道](#udp隧道)
       * [socks5代理](#socks5代理)
       * [http正向代理](#http正向代理)
    * [使用https](#使用https)
    * [与nginx配合](#与nginx配合)
    * [关闭http|https代理](#关闭代理)
    * [将nps安装到系统](#将nps安装到系统)
* 单隧道模式及介绍
    * [tcp隧道模式](#tcp隧道模式)
    * [udp隧道模式](#udp隧道模式)
    * [socks5代理模式](#socks5代理模式)
    * [http代理模式](#http代理模式)

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
   * [守护进程](#守护进程)
* [相关说明](#相关说明)
   * [流量统计](#流量统计)
   * [热更新支持](#热更新支持)
   * [获取用户真实ip](#获取用户真实ip)
   * [客户端地址显示](#客户端地址显示)
* [简单的性能测试](#简单的性能测试)
   * [qps](#qps)
   * [速度测试](#速度测试)
   * [内存和cpu](#内存和cpu)
   * [额外消耗连接数](#额外消耗连接数)
* [webAPI](#webAPI)



## 安装

### release安装
> https://github.com/cnlh/nps/releases

下载对应的系统版本即可，服务端和客户端是单独的，go语言开发，无需任何第三方依赖

### 源码安装
- 安装源码(另有snappy、beego包)
> go get github.com/cnlh/nps
- 编译
> go build cmd/nps/nps.go

> go build cmd/npc/npc.go

## web管理模式

![image](https://github.com/cnlh/nps/blob/master/image/web2.png?raw=true)
### 介绍

可在网页上配置和管理各个tcp、udp隧道、内网站点代理，http、https解析等，功能极为强大，操作也非常方便。


**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**

### 启动


#### 服务端测试
```
 ./nps test
```
如有错误请及时修改配置文件，无错误可继续进行下去
#### 服务端启动
```
 ./nps start
```
如果无需daemon运行，去掉start即可

#### web管理

进入web界面，公网ip:web界面端口（默认8080），密码默认为123

进入web管理界面，有详细的说明

#### 客户端启动
```
 ./npc -server=ip:port -vkey=web界面中显示的密钥
```
#### 服务端停止或重启
如果是daemon启动
```
 ./nps stop|restart
```

### 服务端配置文件
- /conf/app.conf

名称 | 含义
---|---
httpport | web管理端口
password | web界面管理密码
tcpport  | 服务端客户端通信端口
pemPath | ssl certFile绝对路径
keyPath | ssl keyFile绝对路径
httpsProxyPort | 域名代理https代理监听端口
httpProxyPort | 域名代理http代理监听端口
authip|web api免验证IP地址

### 详细说明

#### 域名解析

**适用范围：** 小程序开发、微信公众号开发、产品演示

**假设场景：**
- 有一个域名proxy.com，有一台公网机器ip为1.1.1.1
- 两个内网开发站点127.0.0.1:81，127.0.0.1:82
- 想通过（http|https://）a.proxy.com访问127.0.0.1:81，通过（http|https://）b.proxy.com访问127.0.0.1:82
- 例如配置文件中tcpport为8284

**使用步骤**
- 将*.proxy.com解析到公网服务器1.1.1.1
- 在客户端管理中创建一个客户端，记录下验证密钥
- 点击该客户端的域名管理，添加两条规则规则：1、域名：a.proxy.com，内网目标：127.0.0.1:81，2、域名：b.proxy.com，内网目标：127.0.0.1:82
- 内网客户端运行

```
./npc -server=1.1.1.1:8284 -vkey=客户端的密钥
```
现在访问（http|https://）a.proxy.com，b.proxy.com即可成功

**https:** 如需使用https请在配置文件中将https端口设置为443，和将对应的证书文件路径添加到配置文件中，上面添加的这条记录将会把http、https都转发到内网目标

#### tcp隧道


**适用范围：**  ssh、远程桌面等tcp连接场景

**假设场景：**
 想通过访问公网服务器1.1.1.1的8001端口，连接内网机器10.1.50.101的22端口，实现ssh连接，例如配置文件中tcpport为8284

**使用步骤**
- 在客户端管理中创建一个客户端，记录下验证密钥
- -内网客户端运行
```
./npc -server=1.1.1.1:8284 -vkey=客户端的密钥
```
- 在该客户端隧道管理中添加一条tcp隧道，填写监听的端口（8001）、内网目标ip和目标端口（10.1.50.101:22），选择压缩方式，保存。
- 访问公网服务器ip（127.0.0.1）,填写的监听端口(8001)，相当于访问内网ip(10.1.50.101):目标端口(22)，例如：ssh -p 8001 root@127.0.0.1

#### udp隧道



**适用范围：**  内网dns解析等udp连接场景

**假设场景：**
内网有一台dns（10.1.50.102:53），在非内网环境下想使用该dns，公网服务器为1.1.1.1，例如配置文件中tcpport为8284

**使用步骤**
- 在客户端管理中创建一个客户端，记录下验证密钥
- -内网客户端运行
```
./npc -server=1.1.1.1:8284 -vkey=客户端的密钥
```
- 在该客户端的隧道管理中添加一条udp隧道，填写监听的端口（53）、内网目标ip和目标端口（10.1.50.102:53），选择压缩方式，保存。
- 修改本机dns为127.0.0.1，则相当于使用10.1.50.202作为dns服务器

#### socks5代理


**适用范围：**  在外网环境下如同使用vpn一样访问内网设备或者资源

**假设场景：**
想将公网服务器1.1.1.1的8003端口作为socks5代理，达到访问内网任意设备或者资源的效果，例如配置文件中tcpport为8284

**使用步骤**
- 在客户端管理中创建一个客户端，记录下验证密钥
- -内网客户端运行
```
./npc -server=1.1.1.1:8284 -vkey=客户端的密钥
```
- 在该客户端隧道管理中添加一条socks5代理，填写监听的端口（8003），验证用户名和密码自行选择（建议先不填，部分客户端不支持，proxifer支持），选择压缩方式，保存。
- 在外网环境的本机配置socks5代理，ip为公网服务器ip（127.0.0.1），端口为填写的监听端口(8003)，即可畅享内网了

#### http正向代理

**适用范围：**  在外网环境下使用http代理访问内网站点

**假设场景：**
想将公网服务器1.1.1.1的8004端口作为http代理，访问内网网站，例如配置文件中tcpport为8284

**使用步骤**
- 在客户端管理中创建一个客户端，记录下验证密钥
- -内网客户端运行
```
./npc -server=1.1.1.1:8284 -vkey=客户端的密钥
```
- 在该客户端隧道管理中添加一条http代理，填写监听的端口（8004），选择压缩方式，保存。
- 在外网环境的本机配置http代理，ip为公网服务器ip（127.0.0.1），端口为填写的监听端口(8004)，即可访问了


### 使用https

在配置文件中将httpsProxyPort设置为443或者其他你想配置的端口，和将对应的证书文件路径添加到配置文件中，然后就和http代理一样了，例如

- 需要访问https://a.proxy.com 对应内网127.0.0.1:80

- 在域名代理中添加a.proxy.com 内网目标127.0.0.1:80 即可将所有到达本代理的http(s)请求都转发到127.0.0.1:80

### 与nginx配合

有时候我们还需要在云服务器上运行nginx来保证静态文件缓存等，本代理可和nginx配合使用，在配置文件中将httpProxyPort设置为非80端口，并在nginx中配置代理，例如httpProxyPort为8024时
```
server {
    listen 80;
    server_name *.proxy.com;
    location / {
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
        proxy_pass http://127.0.0.1:8024;
    }
}
```
### 关闭代理

如需关闭http代理可在配置文件中将httpProxyPort设置为空，如需关闭https代理可在配置文件中将httpsProxyPort设置为空。

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




## tcp隧道模式

### 场景及原理
较为适用于处理tcp连接，例如ssh，同时也适用于http等，访问服务端的8024端口相当于访问内网10.1.50.202机器的4000端口，构成如下所示的隧道。

![image](https://github.com/cnlh/nps/blob/master/image/tcp.png?raw=true)

例如：

**背景:**

- 内网机器10.1.50.203提供了web服务80端口

- 有VPS一个,公网IP:123.206.77.88

**需求:**

在家里能够通过访问VPS的8024端口访问到内网机器A的80端口

### 使用
- 服务端

```
./nps -mode=tunnelServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024 -target=10.1.50.203:80
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
./npc -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 与nginx配合实现访问a.ourcauc.com等同访问10.1.50.203:80效果，将该域名解析道云服务器，nginx配置
```
server {
    listen 80;
    server_name a.ourcauc.com;
    location / {
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

![image](https://github.com/cnlh/nps/blob/master/image/udp.png?raw=true)


### 使用
- 服务端

```
./nps -mode=udpServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=53 -target=10.1.50.210:53
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
./npc -server=ip:port -vkey=DKibZF5TXvic1g3kY
```




## socks5代理模式

### 场景及原理

**原理**

主要用于socks5代理，也就是和ss类似，不过是代理内网。使用此模式时，可在非内网环境下配置本机的socks5代理（服务器ip、sock5代理端口），即可实现socks5代理，达到访问内网的网站的效果，配合proxifier等全局代理软件，即可如同使用内网vpn一样，访问内网网站，通过ssh连接内网机器等等……。
![image](https://github.com/cnlh/nps/blob/master/image/sock5.png?raw=true)

### 使用
- 服务端

```
./nps -mode=socks5Server -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
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
./npc -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 需要使用内网代理的机器

```
配置socks5代理即可，ip为外网服务器ip，端口为httpport，即可在外网环境使用内网啦！也可使用proxifier等全局代理软件。
```
如果设置了用户名和密码，记得填上用户名和密码(仅部分客户端支持密码验证)



## http代理模式

### 场景及原理
主要用于HTTP代理，区别也就是HTTP代理和sock5代理的区别。使用此模式时，可在非内网环境下配置本机的HTTP代理（服务器ip、HTTP代理端口），即可实现HTTP代理，达到访问内网的网站的效果。
![image](https://github.com/cnlh/nps/blob/master/image/httpProxy.png?raw=true)


### 使用
- 服务端

```
./nps -mode=httpProxyServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
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
./npc -server=ip:port -vkey=DKibZF5TXvic1g3kY
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



### 站点保护
域名代理模式所有客户端共用一个http服务端口，在知道域名后任何人都可访问，一些开发或者测试环境需要保密，所以可以设置用户名和密码，nps将通过 Http Basic Auth 来保护，访问时需要输入正确的用户名和密码。


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

### 负载均衡
本代理支持域名解析模式的负载均衡，在web域名添加或者编辑中内网目标分行填写多个目标即可实现轮训级别的负载均衡

### 守护进程
本代理支持守护进程，使用示例如下，服务端客户端所有模式通用,支持linux，darwin，windows。
```
./(nps|npc) start|stop|restart|status xxxxxx
```
```
(nps|npc).exe start|stop|restart|status xxxxxx
```

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

## 简单性能测试

### qps
![image](https://github.com/cnlh/nps/blob/master/image/qps.png?raw=true)
### 速度测试
**测试环境：** 1M带宽云服务器，理论125kb/s，带宽与代理无关，与服务器关系较大。
![image](https://github.com/cnlh/nps/blob/master/image/speed.png?raw=true)


### 内存和cpu

**1000次性能测试后**
![image](https://github.com/cnlh/nps/blob/master/image/cpu1.png?raw=true)

**启动时**
![image](https://github.com/cnlh/nps/blob/master/image/cpu2.png?raw=true)

### 额外消耗连接数
为了最大化的提升效率和并发，客户端与服务端之间仅两条tcp连接，减少建立连接的时间消耗和多余tcp连接对机器性能的影响。

## webAPI

为方便第三方扩展，在web模式下可利用webAPI进行相关操作，详情见
[webAPI文档](https://github.com/cnlh/nps/wiki/webAPI%E6%96%87%E6%A1%A3)

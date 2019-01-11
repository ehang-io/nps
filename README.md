# easyProxy
![](https://img.shields.io/github/stars/cnlh/easyProxy.svg)   ![](https://img.shields.io/github/forks/cnlh/easyProxy.svg) ![](https://img.shields.io/github/license/cnlh/easyProxy.svg)

easyProxy是一款轻量级、高性能、功能最为强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何tcp、udp上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**，可实现在非内网环境下如同使用vpn一样访问内网资源和设备的效果，同时**支持socks5验证，snnapy压缩（节省带宽和流量）、站点保护、加密传输、多路复用**。

目前市面上提供类似服务的有花生壳、TeamView、GoToMyCloud等等，但要使用第三方的公网服务器就必须为第三方付费，并且这些服务都有各种各样的限制，此外，由于数据包会流经第三方，因此对数据安全也是一大隐患。


支持客户端与服务端连接中断自动重连，多路传输，大大的提高请求处理速度，go语言编写，无第三方依赖。

## 背景
![image](https://github.com/cnlh/easyProxy/blob/master/image/web.png?raw=true)
1. web管理模式，可配置多条tcp、udp隧道，多个域名代理等等----> [web管理模式](#web管理模式)


2. 想在外网通过ssh连接内网的机器，做云服务器到内网服务器端口的映射，或者做微信公众号开发、小程序开发等---->[tcp隧道模式](#tcp隧道模式)

3. 在非内网环境下使用内网dns，或者需要通过udp访问内网机器等---->[udp隧道模式](#udp隧道模式)

4. 在外网使用HTTP代理访问内网站点---->[http代理模式](#http代理模式)

5. 搭建一个内网穿透ss，在外网如同使用内网vpn一样访问内网资源或者设备----> [socks5代理模式](#socks5代理模式)

## 特点
- [x] 支持snappy压缩,减小传输过程流量消耗
- [x] 断线自动重连
- [x] 支持多路传输,提高并发
- [x] 跨站自动匹配替换
- [x] 支持tcp隧道,提升访问效率
- [x] 支持udp隧道
- [x] 支持http代理
- [x] 支持内网穿透sock5代理，配合proxifier可达到vpn的效果，在外网访问内网资源或者设备，同时可以设置用户名和密码验证
- [x] 强大的web管理界面，可方便的设置的和管理隧道
- [x] 支持站点密码保护
- [x] 支持加密传输
- [x] 支持TCP多路复用
- [x] 支持同时开多条tcp、udp隧道等等，且只需要开一个客户端和服务端
- [x] 支持一个服务端，多个客户端模式

## 目录

1. [安装](#安装)
2. [web管理模式](#web管理模式)（多隧道时推荐）
3. [tcp隧道模式](#tcp隧道模式)
4. [udp隧道模式](#udp隧道模式)
5. [socks5代理模式](#socks5代理模式)
6. [http代理模式](#http代理模式)
7. [数据压缩支持](#数据压缩支持)
8. [站点密码保护](#站点保护)
9. [加密传输](#加密传输)
10. [TCP多路复用](#多路复用)
11. [配置文件说明](#配置文件)

## 安装

1. release安装
> https://github.com/cnlh/easyProxy/releases

下载对应的系统版本即可，服务端和客户端是单独的，go语言开发，无需任何第三方依赖

2. 源码安装
- 安装源码
> go get github.com/cnlh/easyProxy
- 编译
> go build cmd/proxy_server.go
> go build cmd/proxy_client.go

## web管理模式

![image](https://github.com/cnlh/easyProxy/blob/master/image/web2.png?raw=true)
### 介绍

可在网页上配置和管理各个tcp、udp隧道、内网站点代理等等，功能极为强大，操作也非常方便。

**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**

### 使用

**有两种模式：**

1、单客户端模式，所有的隧道流量均从这个单客户端转发。


- 服务端

```
 ./proxy_server -mode=webServer -tcpport=8284 -vkey=DKibZF5TXvic1g3kY
```
名称 | 含义
---|---
mode | 运行模式
vkey | 验证密钥
tcpport | 服务端与客户端通信端口


- 客户端

```
 ./proxy_client -server=ip:port -vkey=DKibZF5TXvic1g3kY
```
- 配置

进入web界面，公网ip:web界面端口（默认8080），密码为123

2、多客户端模式，不同的隧道流量均从不同的客户端转发。


- 服务端

```
 ./proxy_server -mode=webServer -tcpport=8284
```
名称 | 含义
---|---
mode | 运行模式
tcpport | 服务端与客户端通信端口
- 客户端

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
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header Host  $http_host;
            proxy_set_header X-Nginx-Proxy true;
            proxy_set_header Connection "";
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

- 客户端



```
./proxy_client -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 需要使用内网代理的机器


```
配置HTTP代理即可，ip为外网服务器ip，端口为httpport，即可在外网环境访问内网啦！
```

## 数据压缩支持

由于是内网穿透，内网客户端与服务端之间的隧道存在大量的数据交换，为节省流量，加快传输速度，由此本程序支持SNNAPY形式的压缩。


- 所有模式均支持数据压缩，可以与加密同时使用


- 在server端加上参数 -compress=snappy（或在web管理中设置）
```
-compress=snappy
```

## 加密传输

如果公司内网防火墙对外网访问进行了流量识别与屏蔽，例如禁止了ssh协议等，通过设置 配置文件，将服务端与客户端之间的通信内容加密传输，将会有效防止流量被拦截。


- 在server端加上参数 -crypt=true（或在web管理中设置）
```
-crypt=true
```

## 多路复用

客户端和服务器端之间的连接支持多路复用，不再需要为每一个用户请求创建一个连接，使连接建立的延迟降低，并且避免了大量文件描述符的占用。


- 在server端加上参数 -mux=true（或在web管理中设置）

```
-mux=true
```


## 站点保护
由于所有客户端共用一个 http 服务端口，任何知道你的域名和 url 的人都能访问到你部署在内网的 web 服务，但是在某些场景下需要确保只有限定的用户才能访问。

easyProxy支持通过 HTTP Basic Auth 来保护你的 web 服务，使用户需要通过用户名和密码才能访问到你的服务。

目前支持使用tcp协议的web站点保护，命令行运行时可设置
```
-u=user -p=password
```

web管理中也可配置



## 配置文件
- /conf/app.conf

名称 | 含义
---|---
httpport | web管理端口
password | web界面管理密码
hostPort | 域名代理模式监听端口


## 操作系统支持
支持Windows、Linux、MacOSX等，无第三方依赖库。

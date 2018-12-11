# easyProxy

easyProxy是一款轻量级、高性能、功能最为强大的**内网穿透**代理服务器。目前支持**tcp、udp流量转发**，可支持任何tcp、udp上层协议（访问内网网站、本地支付接口调试、ssh访问、远程桌面，内网dns解析等等……），此外还**支持内网http代理、内网socks5代理**，可实现在非内网环境下如同使用vpn一样访问内网资源和设备的效果，同时**支持socks5验证，gzip、snnapy压缩（节省带宽和流量）**。

目前市面上提供类似服务的有花生壳、TeamView、GoToMyCloud等等，但要使用第三方的公网服务器就必须为第三方付费，并且这些服务都有各种各样的限制，此外，由于数据包会流经第三方，因此对数据安全也是一大隐患。


支持客户端与服务端连接中断自动重连，多路传输，大大的提高请求处理速度，go语言编写，无第三方依赖。

## 背景	  
![image](https://github.com/cnlh/easyProxy/blob/master/image/web.png?raw=true)
1. web管理模式，可配置多条tcp、udp隧道，多个域名代理等等----> [web管理模式](#web管理模式)

2. 内网多站点配合代理。----> [http反向代理请求](#http代理请求)

3. 想在外网通过ssh连接内网的机器，做云服务器到内网服务器端口的映射，或者做微信公众号开发、小程序开发等---->[tcp隧道模式](#tcp隧道模式)

4. 在非内网环境下使用内网dns，或者需要通过udp访问内网机器等---->[udp隧道模式](#udp隧道模式)

5. 在外网使用HTTP代理访问内网站点---->[http代理模式](#http代理模式)

6. 搭建一个内网穿透ss，在外网如同使用内网vpn一样访问内网资源或者设备----> [socks5代理模式](#socks5代理模式)

## 特点
- [x] 支持gzip、snappy压缩,减小传输过程流量消耗
- [x] 支持多站点配置,兼容多个内网网站，可处理相互之间的跳转包含关系
- [x] 断线自动重连
- [x] 支持多路传输,提高并发
- [x] 跨站自动匹配替换
- [x] 支持tcp隧道,提升访问效率
- [x] 支持udp隧道
- [x] 支持http代理
- [x] 支持内网穿透sock5代理，配合proxifer可达到vpn的效果，在外网访问内网资源或者设备，同时可以设置用户名和密码验证
- [x] 强大的web管理界面，可方便的设置的和管理隧道
- [x] 支持同时开多条tcp、udp隧道等等，且只需要开一个客户端和服务端
- [x] 支持一个服务端，多个客户端模式

## 目录

1. [安装](#安装)
2. [web管理模式](#web管理模式)（推荐）
3. [tcp隧道模式](#tcp隧道模式)
4. [udp隧道模式](#udp隧道模式)
5. [http反向代理请求](#http代理请求)
6. [socks5代理模式](#sock5代理模式)
7. [http代理模式](#http代理模式)
8. [数据压缩支持](#数据压缩支持)
9. [操作系统支持](#操作系统支持)



## 安装

1. release安装
> https://github.com/cnlh/easyProxy/releases

下载对应的系统版本即可（目前linux和windows只编译了64位的），服务端和客户端共用一个程序，go语言开发，无需任何第三方依赖

2. 源码安装
- 安装源码
> go get github.com/cnlh/easyProxy
- 编译（无第三方模块）
> go build

## web管理模式

![image](https://github.com/cnlh/easyProxy/blob/master/image/web2.png?raw=true)
### 介绍

可在网页上配置和管理各个tcp、udp隧道、内网站点代理等等，功能极为强大，操作也非常方便。[演示地址](http://123.206.77.88:8081) 密码：123
### 使用

**有两种模式：**

1、单客户端模式，所有的隧道流量均从这个单客户端转发。


- 服务端

```
 ./easyProxy -mode=webServer -tcpport=8284 -vkey=DKibZF5TXvic1g3kY
```
名称 | 含义
---|---
mode | 运行模式
vkey | 验证密钥
tcpport | 服务端与客户端通信端口


- 客户端

```
 ./easyProxy -server=ip:port -vkey=DKibZF5TXvic1g3kY
```
- 配置

进入web界面，公网ip:web界面端口（默认8080），密码为123

2、多客户端模式，不同的隧道流量均从不同的客户端转发。


- 服务端

```
 ./easyProxy -mode=webServer -tcpport=8284
```
名称 | 含义
---|---
mode | 运行模式
tcpport | 服务端与客户端通信端口
- 客户端

进入web管理界面，有详细的命令

- 配置

进入web界面，公网ip:web界面端口（默认8080），密码为123

### 配置文件/conf/app.conf

名称 | 含义
---|---
httpport | web管理端口
password | web界面管理密码
hostPort | 域名代理模式监听端口

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
./easyProxy -mode=tunnelServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024 -target=10.1.50.203:80
```

名称 | 含义
---|---
mode | 运行模式(client、server不写默认client)
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | 外部访问端口
target | 目标地址，格式如上

- 客户端


```
./easyProxy -server=ip:port -vkey=DKibZF5TXvic1g3kY
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
./easyProxy -mode=udpServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=53 -target=10.1.50.210:53
```

名称 | 含义
---|---
mode | 运行模式(client、server不写默认client)
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | 公网vps的访问端口
target | 目标地址，格式如上

- 客户端


```
./easyProxy -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

## http代理请求

### 场景及原理

较为适用于http，也就是web站点的穿透，服务端与客户端之间建立连接，服务端收到http请求后，将请求发送到客户端，客户端再执行这个请求，并将结果返回给服务端，服务端收到后再返回。

<html>
<span style="color:red">特点：支持同时代理多个站点，不同站点之间有联系还可以实现匹配替换</span>
</html>

![image](https://github.com/cnlh/easyProxy/blob/master/image/http.png?raw=true)

**最终效果**：
- 访问a.server.com和访问10.1.50.203的80端口相同
- 访问b.server.com和访问10.1.50.202的80端口相同
- 访问c.server.com和访问10.1.50.201的80端口相同
### 使用 
- 服务端 

```
./easyProxy -mode=httpServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
```

名称 | 含义
---|---
mode | 运行模式(client、server不写默认client)
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | 代理的http端口（与nginx配合使用）

- 客户端

```
建立配置文件 config.json
```


```
./easyProxy -server=ip:port -config=config.json -vkey=DKibZF5TXvic1g3kY
```


 名称 | 含义
---|---
config | 配置文件路径
### 配置文件config.json

```
{
  "SiteList": [
    {
      "host": "a.ourcauc.com",
      "url": "10.1.50.203",
      "port": 80
    },
    {
      "host": "b.ourcauc.com",
      "url": "10.1.50.202",
      "port": 80
    },
    {
      "host": "c.ourcauc.com",
      "url": "10.1.50.203",
      "port": 80
    }
  ],
  "Replace": 0
}
```
 名称 | 含义
---|---
SiteList | 本地解析的域名列表
host | 域名地址
url | 内网代理的地址
port | 内网代理的地址对应的端口
Replace | 是否自动匹配替换[（查看场景）](https://github.com/cnlh/easyProxy/issues/1)


### nginx代理配置示例
```
upstream nodejs {
    server 127.0.0.1:8024;
    keepalive 64;
}
server {
    listen 80;
    server_name a.ourcauc.com b.ourcauc.com c.ourcauc.com ;
    location / {
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header Host  $http_host;
            proxy_set_header X-Nginx-Proxy true;
            proxy_set_header Connection "";
            proxy_pass      http://nodejs;
        }
}
```
## 域名配置示例
> -a	    A	    123.206.77.88

> -b	    A	    123.206.77.88

> -c	    A	    123.206.77.88

### 跨站自动匹配替换说明

例如，访问：a.ourcauc.com，该页面里面有一个超链接为10.1.50.202:80,将根据配置文件自动该将url替换为b.ourcauc.com，以达到跨站也可访问的效果，但需要提前在配置文件中配置这些站点。

如需开启，请加配置文件Replace值设置为1
>注意：开启可能导致不应该被替换的内容被替换，请谨慎开启

### 二级域名示范

[二级域名](https://github.com/cnlh/easyProxy/wiki/%E4%BD%BF%E7%94%A8%E6%95%99%E7%A8%8B)


## socks5代理模式

### 场景及原理

**原理**

主要用于socks5代理，也就是和ss类似，不过是代理内网。使用此模式时，可在非内网环境下配置本机的socks5代理（服务器ip、sock5代理端口），即可实现socks5代理，达到访问内网的网站的效果，配合proxifer等全局代理软件，即可如同使用内网vpn一样，访问内网网站，通过ssh连接内网机器等等……。
![image](https://github.com/cnlh/easyProxy/blob/master/image/sock5.png?raw=true)

### 使用 
- 服务端 

```
./easyProxy -mode=sock5ServerServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
```

名称 | 含义
---|---
mode | 运行模式(client、server不写默认client)
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | 代理的http端口（socks5连接端口）
u | 验证的用户名
p | 验证的密码

**说明**：用户名和密码验证模式，仅部分socks5客户端支持，例如proxifer。
如需验证，在服务端命令后加上
```
-u=user -p=password
```
即可

- 客户端


```
./easyProxy -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 需要使用内网代理的机器

```
配置sock5代理即可，ip为外网服务器ip，端口为httpport，即可在外网环境使用内网啦！也可使用proxifer等全局代理软件。
```
如果设置了用户名和密码，记得填上用户名和密码



## http代理模式

### 场景及原理
主要用于HTTP代理，区别也就是HTTP代理和sock5代理的区别。使用此模式时，可在非内网环境下配置本机的HTTP代理（服务器ip、HTTP代理端口），即可实现HTTP代理，达到访问内网的网站的效果。
![image](https://github.com/cnlh/easyProxy/blob/master/image/httpProxy.png?raw=true)


### 使用 
- 服务端 

```
./easyProxy -mode=httpProxyServer -vkey=DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
```

名称 | 含义
---|---
mode | 运行模式(client、server不写默认client)
vkey | 验证密钥
tcpport | 服务端与客户端通信端口
httpport | http代理连接端口

- 客户端



```
./easyProxy -server=ip:port -vkey=DKibZF5TXvic1g3kY
```

- 需要使用内网代理的机器


```
配置HTTP代理即可，ip为外网服务器ip，端口为httpport，即可在外网环境访问内网啦！
```

## 数据压缩支持

### 场景及原理
由于是内网穿透，内网客户端与服务端之间的隧道存在大量的数据交换，为节省流量，加快传输速度，由此本程序支持GZIP、SNNAPY两种形式的压缩，两者差异请自行选择。

### 注意点

- 所有模式均支持数据压缩


### 如何使用

**GZIP压缩**

- 在server端加上参数 -compress=gzip，例如在TCP隧道模式
```
./easyProxy -mode tunnelServer -vkey DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024 -target=10.1.50.203:80 -compress=gzip
```

**SNAPPY压缩**

将参数修改为snappy即可

## 操作系统支持
支持Windows、Linux、MacOSX等，无第三方依赖库。

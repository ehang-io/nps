# easyProxy
轻量级、较高性能http代理服务器，主要应用与内网穿透。支持多站点配置、客户端与服务端连接中断自动重连，多路传输，大大的提高请求处理速度，go语言编写，无第三方依赖，经过测试内存占用小，普通场景下，仅占用10m内存。

## 背景	  
我有一个小程序的需求，但是小程序的数据源必须从内网才能抓取到，但是又苦于内网服务器没有公网ip，所以只能内网穿透了。

用了一段时间ngrok做内网穿透，可能由于功能比较强大，配置起来挺麻烦的，加之开源版有内存的泄漏，很是闹心。

正好最近在看go相关的东西，所以做了一款代理服务器，功能比较简单，用于内网穿透最为合适。

## 特点

- [x] 支持多站点配置
- [x] 断线自动重连
- [x] 支持多路传输,提高并发
## 安装
1. release安装
> https://github.com/cnlh/easyProxy/releases

下载对应的系统版本即可（目前linux和windows只编译了64位的），服务端和客户端共用一个程序，go语言开发，无需任何第三方依赖

2. 源码安装
- 安装源码
> go get github.com/cnlh/easyProxy
- 编译（无第三方模块）
> go build

## 使用 
- 服务端 

```
./rproxy -mode server -vkey DKibZF5TXvic1g3kY -tcpport=8284 -httpport=8024
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
./rproxy -config config.json  
```


 名称 | 含义
---|---
config | 配置文件路径

- 详细说明

[详细教程](https://github.com/cnlh/easyProxy/wiki/%E4%BD%BF%E7%94%A8%E6%95%99%E7%A8%8B)



## 配置文件config.json

```
{
  "Server": {
    "ip": "123.206.77.88",
    "tcp": 8284,
    "vkey": "DKibZF5TXvic1g3kY",
    "num": 10
  },
  "SiteList": [
    {
      "host": "a.server.ourcauc.com",
      "url": "10.1.50.203",
      "port": 80
    },
    {
      "host": "b.server.ourcauc.com",
      "url": "10.1.50.196",
      "port": 4000
    }
  ]
}
```
 名称 | 含义
---|---
ip | 服务端ip地址
tcp | 服务端与客户端通信端口
vkey | 验证密钥
num | 服务端与客户端通信连接数
SiteList | 本地解析的域名列表
host | 域名地址
url | 内网代理的地址
port | 内网代理的地址对应的端口

## 运行流程解析



```
graph TD
A[通过域名访问对应内网服务]-->B[nginx代理转发该域名服务端监听的8024端口]
B-->C[服务端将请求发送到客户端上]
C-->D[客户端收到请求信息,根据host判断对应的内网的请求地址,执行对应请求]
D-->E[将请求结果返回给服务端]
E-->F[服务端收到后返回给访问者]
```

## nginx代理配置示例
```
upstream nodejs {
    server 127.0.0.1:8024;
    keepalive 64;
}
server {
    listen 80;
    server_name *.server.ourcauc.com;
    location / {
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header Host  $http_host:8224;
            proxy_set_header X-Nginx-Proxy true;
            proxy_set_header Connection "";
            proxy_pass      http://nodejs;
        }
}
```
## 域名配置示例
> -server	    A	    123.206.77.88

> *.server	CNAME	server.ourcauc.com.
 

## 操作系统支持  
支持Windows、Linux、MacOSX等，无第三方依赖库。  


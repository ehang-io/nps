# 增强功能
## 使用https

**方式一：** 类似于nginx实现https的处理

在配置文件中将https_proxy_port设置为443或者其他你想配置的端口，将`https_just_proxy`设置为false，nps 重启后，在web管理界面，域名新增或修改界面中修改域名证书和密钥。

**此外：** 可以在`nps.conf`中设置一个默认的https配置，当遇到未在web中设置https证书的域名解析时，将自动使用默认证书，另还有一种情况就是对于某些请求的clienthello不携带sni扩展信息，nps也将自动使用默认证书


**方式二：** 在内网对应服务器上设置https

在`nps.conf`中将`https_just_proxy`设置为true，并且打开`https_proxy_port`端口，然后nps将直接转发https请求到内网服务器上，由内网服务器进行https处理

## 与nginx配合

有时候我们还需要在云服务器上运行nginx来保证静态文件缓存等，本代理可和nginx配合使用，在配置文件中将httpProxyPort设置为非80端口，并在nginx中配置代理，例如httpProxyPort为8010时
```
server {
    listen 80;
    server_name *.proxy.com;
    location / {
        proxy_set_header Host  $http_host;
        proxy_pass http://127.0.0.1:8010;
    }
}
```
如需使用https也可在nginx监听443端口并配置ssl，并将本代理的httpsProxyPort设置为空关闭https即可，例如httpProxyPort为8020时

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
        proxy_pass http://127.0.0.1:8020;
    }
}
```
## web管理使用https
如果web管理需要使用https，可以在配置文件`nps.conf`中设置`web_open_ssl=true`，并配置`web_cert_file`和`web_key_file`
## web使用Caddy代理

如果将web配置到Caddy代理,实现子路径访问nps,可以这样配置.

假设我们想通过 `http://caddy_ip:caddy_port/nps` 来访问后台, Caddyfile 这样配置:

```Caddyfile
caddy_ip:caddy_port/nps {
  ##server_ip 为 nps 服务器IP
  ##web_port 为 nps 后台端口
  proxy / http://server_ip:web_port/nps {
	transparent
  }
}
```

nps.conf 修改 `web_base_url` 为 `/nps` 即可
```
web_base_url=/nps
```


## 关闭代理

如需关闭http代理可在配置文件中将http_proxy_port设置为空，如需关闭https代理可在配置文件中将https_proxy_port设置为空。

## 流量数据持久化
服务端支持将流量数据持久化，默认情况下是关闭的，如果有需求可以设置`nps.conf`中的`flow_store_interval`参数，单位为分钟

**注意：** nps不会持久化通过公钥连接的客户端
## 系统信息显示
nps服务端支持在web上显示和统计服务器的相关信息，但默认一些统计图表是关闭的，如需开启请在`nps.conf`中设置`system_info_display=true`

## 自定义客户端连接密钥
web上可以自定义客户端连接的密钥，但是必须具有唯一性
## 关闭公钥访问
可以将`nps.conf`中的`public_vkey`设置为空或者删除

## 关闭web管理
可以将`nps.conf`中的`web_port`设置为空或者删除

## 服务端多用户登陆
如果将`nps.conf`中的`allow_user_login`设置为true,服务端web将支持多用户登陆，登陆用户名为user，默认密码为每个客户端的验证密钥，登陆后可以进入客户端编辑修改web登陆的用户名和密码，默认该功能是关闭的。

## 用户注册功能
nps服务端支持用户注册功能，可将`nps.conf`中的`allow_user_register`设置为true，开启后登陆页将会有有注册功能，

## 监听指定ip

nps支持每个隧道监听不同的服务端端口,在`nps.conf`中设置`allow_multi_ip=true`后，可在web中控制，或者npc配置文件中(可忽略，默认为0.0.0.0)
```ini
server_ip=xxx
```
## 代理到服务端本地
在使用nps监听80或者443端口时，默认是将所有的请求都会转发到内网上，但有时候我们的nps服务器的上一些服务也需要使用这两个端口，nps提供类似于`nginx` `proxy_pass` 的功能，支持将代理到服务器本地，该功能支持域名解析，tcp、udp隧道，默认关闭。

**即：** 假设在nps的vps服务器上有一个服务使用5000端口，这时候nps占用了80端口和443，我们想能使用一个域名通过http(s)访问到5000的服务。

**使用方式：** 在`nps.conf`中设置`allow_local_proxy=true`，然后在web上设置想转发的隧道或者域名然后选择转发到本地选项即可成功。

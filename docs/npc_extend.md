# 增强功能
## nat类型检测
```
 ./npc nat -stun_addr=stun.stunprotocol.org:3478
```
如果p2p双方都是Symmetric Nat，肯定不能成功，其他组合都有较大成功率。`stun_addr`可以指定stun服务器地址。
## 状态检查
```
 ./npc status -config=npc配置文件路径
```
## 重载配置文件
```
 ./npc restart -config=npc配置文件路径
```

## 通过代理连接nps
有时候运行npc的内网机器无法直接访问外网，此时可以可以通过socks5代理连接nps

对于配置文件方式启动,设置
```ini
[common]
proxy_url=socks5://111:222@127.0.0.1:8024
```
对于无配置文件模式,加上参数

```
-proxy=socks5://111:222@127.0.0.1:8024
```
支持socks5和http两种模式

即socks5://username:password@ip:port

或http://username:password@ip:port

## 群晖支持
可在releases中下载spk群晖套件，例如`npc_x64-6.1_0.19.0-1.spk`

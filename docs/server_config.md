# 服务端配置文件
- /etc/nps/conf/nps.conf

名称 | 含义
---|---
web_port | web管理端口
web_password | web界面管理密码
web_username | web界面管理账号
web_base_url | web管理主路径,用于将web管理置于代理子路径后面
bridge_port  | 服务端客户端通信端口
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
pprof_ip|debug pprof 服务端ip
pprof_port|debug pprof 端口
disconnect_timeout|客户端连接超时，单位 5s，默认值 60，即 300s = 5mins

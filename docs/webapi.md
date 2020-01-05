获取客户端列表

url=&#39;http://你的域名或者网址/client/list/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| search | 搜索 |
| order | 排序asc 正序 desc倒序 |
| offset | 分页(第几页) |
| limit | 条数(分页显示的条数) |

***
获取单个客户端

url=&#39;http://你的域名或者网址/client/getclient/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| id | 客户端id |

***
添加客户端

url=&#39;http://你的域名或者网址/client/add/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| remark | 备注 |
| u | basic权限认证用户名 |
| p | basic权限认证密码 |
| limit | 条数(分页显示的条数) |
| vkey | 客户端验证密钥 |
| config\_conn\_allow | 是否允许客户端以配置文件模式连接 1允许 0不允许 |
| compress | 压缩1允许 0不允许 |
| crypt | 是否加密（1或者0）1允许 0不允许 |
| rate\_limit | 带宽限制 单位KB/S 空则为不限制 |
| flow\_limit | 流量限制 单位M 空则为不限制 |
| max\_conn | 客户端最大连接数量 空则为不限制 |
| max\_tunnel | 客户端最大隧道数量 空则为不限制 |

***
修改客户端(25.4版本有问题暂时不能用)

url=&#39;http://你的域名或者网址/client/edit/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| remark | 备注 |
| u | basic权限认证用户名 |
| p | basic权限认证密码 |
| limit | 条数(分页显示的条数) |
| vkey | 客户端验证密钥 |
| config\_conn\_allow | 是否允许客户端以配置文件模式连接 1允许 0不允许 |
| compress | 压缩1允许 0不允许 |
| crypt | 是否加密（1或者0）1允许 0不允许 |
| rate\_limit | 带宽限制 单位KB/S 空则为不限制 |
| flow\_limit | 流量限制 单位M 空则为不限制 |
| max\_conn | 客户端最大连接数量 空则为不限制 |
| max\_tunnel | 客户端最大隧道数量 空则为不限制 |
| id | 要修改的客户端id |

***
删除客户端

url=&#39;http://你的域名或者网址/client/del/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| id | 要删除的客户端id |

***
获取域名解析列表

url=&#39;http://你的域名或者网址/index/hostlist/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| search | 搜索(可以搜域名/备注什么的) |
| offset | 分页(第几页) |
| limit | 条数(分页显示的条数) |

***
添加域名解析

url=&#39;http://你的域名或者网址/index/addhost/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| remark | 备注 |
| host | 域名 |
| scheme | 协议类型(三种 all http https) |
| location | url路由 空则为不限制 |
| client\_id | 客户端id |
| target | 内网目标(ip:端口) |
| header | request header 请求头 |
| hostchange | request host 请求主机 |

***
修改域名解析

url=&#39;http://你的域名或者网址/index/edithost/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| remark | 备注 |
| host | 域名 |
| scheme | 协议类型(三种 all http https) |
| location | url路由 空则为不限制 |
| client\_id | 客户端id |
| target | 内网目标(ip:端口) |
| header | request header 请求头 |
| hostchange | request host 请求主机 |
| id | 需要修改的域名解析id |

***
删除域名解析

url=&#39;http://你的域名或者网址/index/delhost/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| id | 需要删除的域名解析id |

***
获取单条隧道信息

url=&#39;http://你的域名或者网址/index/getonetunnel/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| id | 隧道的id |

***
获取隧道列表

url=&#39;http://你的域名或者网址/index/gettunnel/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| client\_id | 穿透隧道的客户端id |
| type | 类型tcp udp httpProx socks5 secret p2p |
| search | 搜索 |
| offset | 分页(第几页) |
| limit | 条数(分页显示的条数) |

***
添加隧道

url=&#39;http://你的域名或者网址/index/add/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| type | 类型tcp udp httpProx socks5 secret p2p |
| remark | 备注 |
| port | 服务端端口 |
| target | 目标(ip:端口) |
| client\_id | 客户端id |

***
修改隧道

url=&#39;http://你的域名或者网址/index/edit/&#39;;

***
post提交的数据

| 参数 | 含义 |
| --- | --- |
| type | 类型tcp udp httpProx socks5 secret p2p |
| remark | 备注 |
| port | 服务端端口 |
| target | 目标(ip:端口) |
| client\_id | 客户端id |
| id | 隧道id |

***
删除隧道

url=&#39;http://你的域名或者网址/index/del/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| id | 隧道id |

***
隧道停止工作

url=&#39;http://你的域名或者网址/index/stop/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| id | 隧道id |

***
隧道开始工作

url=&#39;http://你的域名或者网址/index/start/&#39;;

post提交的数据

| 参数 | 含义 |
| --- | --- |
| id | 隧道id |
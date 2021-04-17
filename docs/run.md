# 启动
## 服务端
下载完服务器压缩包后，解压，然后进入解压后的文件夹

- 执行安装命令

对于linux|darwin ```sudo ./nps install```

对于windows，管理员身份运行cmd，进入安装目录 ```nps.exe install```

- 启动

对于linux|darwin ```sudo nps start```

对于windows，管理员身份运行cmd，进入程序目录 ```nps.exe start```

```安装后windows配置文件位于 C:\Program Files\nps，linux和darwin位于/etc/nps```

停止和重启可用，stop和restart

**如果发现没有启动成功，可以使用`nps(.exe) stop`，然后运行`nps.(exe)`运行调试，或查看日志**(Windows日志文件位于当前运行目录下，linux和darwin位于/var/log/nps.log)
- 访问服务端ip:web服务端口（默认为8080）
- 使用用户名和密码登陆（默认admin/123，正式使用一定要更改）
- 创建客户端

## 客户端
- 下载客户端安装包并解压，进入到解压目录
- 点击web管理中客户端前的+号，复制启动命令
- 执行启动命令，linux直接执行即可，windows将./npc换成npc.exe用**cmd执行**

如果使用`powershell`运行，**请将ip括起来！**

如果需要注册到系统服务可查看[注册到系统服务](/use?id=注册到系统服务)

## 版本检查
- 对客户端以及服务的均可以使用参数`-version`打印版本
- `nps -version`或`./nps -version`
- `npc -version`或`./npc -version`

## 配置
- 客户端连接后，在web中配置对应穿透服务即可
- 可以查看[使用示例](/example)

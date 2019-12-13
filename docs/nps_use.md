**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**


# 服务端测试
```shell
 ./nps test
```
如有错误请及时修改配置文件，无错误可继续进行下去
# 服务端启动
```shell
 ./nps start
```
**如果无需daemon运行或者打开后无法正常访问web管理，去掉start查看日志运行即可**

# web管理

进入web界面，公网ip:web界面端口（默认8080），密码默认为123

进入web管理界面，有详细的说明

# 服务端配置文件重载
如果是daemon启动
```shell
 ./nps reload
```
**说明：** 仅支持部分配置重载，例如`allow_user_login` `auth_crypt_key` `auth_key` `web_username` `web_password` 等，未来将支持更多


# 服务端停止或重启
如果是daemon启动
```shell
 ./nps stop|restart
```

# 将nps安装到系统
如果需要长期并且方便的运行nps服务端，可将nps安装到操作系统中，可执行命令

```
(./nps|nps.exe) install
```
安装成功后，对于linux，darwin，将会把配置文件和静态文件放置于/etc/nps/，并将可执行文件nps复制到/usr/bin/nps或者/usr/local/bin/nps，安装成功后可在任何位置执行，同时也会添加systemd配置。

```
sudo systemctl enable|disable|start|stop|restart|status nps
```
systemd，带有开机自启，自动重启配置，当进程结束后15秒会启动，日志输出至/var/log/nps/nps.log。
建议采用此方式启动，能够捕获panic信息，便于排查问题。

```
nps test|start|stop|restart|status
```
对于windows系统，将会把配置文件和静态文件放置于C:\Program Files\nps，安装成功后可将可执行文件nps.exe复制到任何位置执行

```
nps.exe test|start|stop|restart|status
```


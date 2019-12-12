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

# 使用
**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**

## web管理

进入web界面，公网ip:web界面端口（默认8080），密码默认为123

进入web管理界面，有详细的说明

## 服务端配置文件重载
```shell
 sudo nps reload
```
**说明：** 仅支持部分配置重载，例如`allow_user_login` `auth_crypt_key` `auth_key` `web_username` `web_password` 等，未来将支持更多


## 服务端停止或重启
如果是daemon启动
```shell
 ./nps stop|restart
```

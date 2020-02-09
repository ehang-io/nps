# 使用
**提示：使用web模式时，服务端执行文件必须在项目根目录，否则无法正确加载配置文件**

## web管理

进入web界面，公网ip:web界面端口（默认8080），密码默认为123

进入web管理界面，有详细的说明

## 服务端配置文件重载
对于linux、darwin
```shell
 sudo nps reload
```
对于windows
```shell
 nps.exe reload
```
**说明：** 仅支持部分配置重载，例如`allow_user_login` `auth_crypt_key` `auth_key` `web_username` `web_password` 等，未来将支持更多


## 服务端停止或重启
对于linux、darwin
```shell
 sudo nps stop|restart
```
对于windows
```shell
 nps.exe stop|restart
```
## 服务端更新
请首先执行 `sudo nps stop` 或者 `nps.exe stop` 停止运行，然后

对于linux
```shell
 sudo nps-update update
```
对于windows
```shell
 nps-update.exe update
```

更新完成后，执行执行 `sudo nps start` 或者 `nps.exe start` 重新运行即可完成升级

如果无法更新成功，可以直接自行下载releases压缩包然后覆盖原有的nps二进制文件和web目录

注意：`nps install` 之后的 nps 不在原位置，请使用 `whereis nps` 查找具体目录覆盖 nps 二进制文件

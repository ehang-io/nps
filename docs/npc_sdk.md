# npc sdk文档

```
命令行模式启动客户端
从v0.26.10开始，此函数会阻塞，直到客户端退出返回，请自行管理是否重连
serverAddr->连接地址
verifyKey->vkey
connType->连接类型（tcp or udp）
proxyUrl->连接代理

extern GoInt StartClientByVerifyKey(char* serverAddr, char* verifyKey, char* connType, char* proxyUrl);

命令行模式启动本地P2P或私密连接
extern GoInt StartLocalServer(char* serverAddr, char* verifyKey, char* connType, char* password, char* localType, char* localPortStr, char* target, char* proxyUrl);

查看当前启动的客户端状态，在线为1，离线为0
extern GoInt GetClientStatus();

关闭客户端
extern void CloseClient();

获取当前客户端版本
extern char* Version();

获取日志，实时更新
extern char* Logs();
```

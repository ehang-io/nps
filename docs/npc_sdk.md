# npc sdk文档

```
命令行模式启动客户端
p0->连接地址
p1->vkey
p2->连接类型（tcp or udp）
p3->连接代理

extern GoInt StartClientByVerifyKey(char* p0, char* p1, char* p2, char* p3);

查看当前启动的客户端状态，在线为1，离线为0
extern GoInt GetClientStatus();

关闭客户端
extern void CloseClient();

获取当前客户端版本
extern char* Version();

获取日志，实时更新
extern char* Logs();
```

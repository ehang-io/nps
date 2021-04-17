# web api

需要开启请先去掉`nps.conf`中`auth_key`的注释并配置一个合适的密钥
## webAPI验证说明
- 采用auth_key的验证方式
- 在提交的每个请求后面附带两个参数，`auth_key` 和`timestamp`

```
auth_key的生成方式为：md5(配置文件中的auth_key+当前时间戳)
```

```
timestamp为当前时间戳
```
```
curl --request POST \
  --url http://127.0.0.1:8080/client/list \
  --data 'auth_key=2a0000d9229e7dbcf79dd0f5e04bb084&timestamp=1553045344&start=0&limit=10'
```
**注意：** 为保证安全，时间戳的有效范围为20秒内，所以每次提交请求必须重新生成。

## 获取服务端时间
由于服务端与api请求的客户端时间差异不能太大，所以提供了一个可以获取服务端时间的接口

```
POST /auth/gettime
```

## 获取服务端authKey

如果想获取authKey，服务端提供获取authKey的接口

```
POST /auth/getauthkey
```
将返回加密后的authKey，采用aes cbc加密，请使用与服务端配置文件中cryptKey相同的密钥进行解密

**注意：** nps配置文件中`auth_crypt_key`需为16位
- 解密密钥长度128
- 偏移量与密钥相同
- 补码方式pkcs5padding
- 解密串编码方式 十六进制

## 详细文档
- **[详见](webapi.md)** (感谢@avengexyz)

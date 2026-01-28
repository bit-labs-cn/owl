# 验证码模块

本模块提供滑块、点选、旋转验证码的生成与校验能力，支持内存与 Redis 存储，实现由外部注入。

## 功能说明

- 生成验证码：支持 click、slide、rotate
- 校验验证码：按类型校验并自动清理已用记录
- 存储实现：MemoryStore / RedisStore
- captcha.yaml 配置文件会自动生成到 conf 目录

## 路由

- POST `/api/v1/captcha/generate`
- POST `/api/v1/captcha/verify`

## 配置示例

`captcha.yaml`：

```yaml
enabled: true
ttl: 300
type: "click"
mode: "text"
padding: 5
store: "memory" # memory redis
cleanup-interval: 60
```


## 使用示例

生成验证码：

```json
POST /api/v1/captcha/generate
{
  "type": "click"
}
```

校验验证码：

```json
POST /api/v1/captcha/verify
{
  "type": "click",
  "captchaId": "xxxx",
  "points": [{"index":1,"x":10,"y":20}]
}
```

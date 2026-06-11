# wa-app

`wa-app` 是 WA 应用链路服务，提供账号管理、号码探测、注册、登录态检查、长连接会话和消息处理能力，并内置管理 dashboard。

> [!CAUTION]
> 使用本项目即表示你同意 [NOTICE](./NOTICE) 的全部条款。本项目仅限协议建模、教学演示、授权安全研究和内部非商业验证；禁止用于商业用途、未授权目标或违反第三方服务条款的场景。

## 功能

- 账号管理：维护 WAAccount、客户端 profile、注册记录和登录态投影。
- 号码与注册：支持号码探测、SMS 探测、注册请求、OTP 提交和登录态检查。
- 连接与消息：支持长连接会话、消息接收、消息 ack、1:1 文本消息发送和会话查看。
- 数据提取：从消息中提取 OTP/Flag 候选值，并按敏感数据规则保存引用或脱敏投影。
- 管理界面：提供 dashboard，用于账号、联系人、消息、连接状态和账号资料操作。

## 部署方式

推荐使用本仓库提供的 Docker Compose 启动服务：

```sh
cp .env.example .env
docker compose pull
docker compose up -d
```

默认端口：

- Dashboard：`http://127.0.0.1:8080`
- gRPC：`127.0.0.1:50091`

### 配置

`.env` 中保留少量运行必需配置：

- `WA_APP_IMAGE_TAG`：镜像标签，生产建议使用固定版本。
- `WA_APP_AUTH_PASSWORD`：可选 dashboard 单密码登录；为空则关闭鉴权。
- `WA_APP_PG_DSN`：可选 PostgreSQL DSN；为空时使用内置 SQLite 持久化。
- `WA_APP_REDIS_URL`：可选 Redis URL；为空时使用内置 SQLite 运行态存储。
- `WA_COMMON_PROXY`：可选默认 WA 出站代理；为空则直连。
- `WA_NUMBER_PROBE_PROXY`：可选号码/SMS 探测代理；为空时使用 `WA_COMMON_PROXY`，`WA_COMMON_PROXY` 也为空则直连。
- `WA_REGISTRATION_PROXY`：可选注册与 OTP 提交代理；为空时使用 `WA_COMMON_PROXY`，`WA_COMMON_PROXY` 也为空则直连。

PostgreSQL 和 Redis 都是可选组件。需要启用时，在 `docker-compose.yml` 中取消对应服务注释，并在 `.env` 中填写 `WA_APP_PG_DSN` / `WA_APP_REDIS_URL`。

## 友情链接

- [LINUX DO - 新的理想型社区](https://linux.do/)

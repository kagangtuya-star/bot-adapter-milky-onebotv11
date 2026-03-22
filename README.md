# milky-onebot11-bridge

使用 Go 编写的 Milky -> OneBot-11 桥接中间件。

## 当前状态

当前实现已支持：

- 连接 Milky WS / HTTP 网关
- 暴露 OneBot-11 正向 WebSocket：
  - `/`
  - `/api`
  - `/event`
- 支持 OneBot-11 反向 WebSocket：
  - API / Event 双连接
  - Universal 单连接
  - 自动重连
- 可选暴露 HTTP API：
  - `/http/:action`
- 双向消息链路：
  - `send_private_msg`
  - `send_group_msg`
  - `send_msg`
- 基础查询与管理：
  - `get_login_info`
  - `get_status`
  - `get_version_info`
  - `can_send_image`
  - `can_send_record`
  - `get_group_info`
  - `get_group_list`
  - `get_group_member_info`
  - `get_group_member_list`
  - `get_msg`
  - `delete_msg`
  - `set_friend_add_request`
  - `set_group_add_request`
- 基础事件：
  - private/group message
  - poke notice
  - friend/group request
  - friend/group recall
  - lifecycle / heartbeat

## 快速开始

1. 复制配置模板：

   `cp config.example.json config.json`

2. 修改 `config.json` 中的 Milky 网关地址与 token。

3. 启动：

   `go run ./cmd/milky-ob11-bridge -config config.json`

4. 让下游 OneBot-11 bot 连接：

   `ws://<host>:<port>/`

   或：

   `ws://<host>:<port>/api`

   `ws://<host>:<port>/event`

5. 如果启用了 HTTP API，可调用：

   `POST http://<host>:<port>/http/send_group_msg`

6. 如果启用了反向 WebSocket，可在配置中填写：

   - `onebot.reverse.url`
   - `onebot.reverse.api_url`
   - `onebot.reverse.event_url`
   - `onebot.reverse.use_universal_client`

## 配置

见 [config.example.json](/mnt/e/Code/go/sealdice-adapter-milky-onebotv11/config.example.json)。

## 说明

- 当前默认 `message_format=array`，更适合和内部消息 IR 对齐。
- 反向 WebSocket 连接时会按 OneBot-11 规范带上 `X-Self-ID`、`X-Client-Role` 和可选 `Authorization` 头。
- 复杂 CQ 段和部分高级管理接口仍未实现，未实现接口会返回明确错误。
- 原始 `platform_adapter_milky*.go` 文件仅作为参考源码，已通过 build tag 排除，不参与当前 bridge 编译。

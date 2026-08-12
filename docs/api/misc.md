# 通用接口

## GET /health

兼容健康检查，语义等同进程存活检查。

**成功响应**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ok"
  }
}
```

---

## GET /live

进程存活检查。服务进程能够处理 HTTP 请求时返回 200。

---

## GET /ready

核心 Runtime 就绪检查。Runtime 未配置、初始化状态损坏或 Runtime 自检失败时返回 503；成功时返回 `status: ready` 和 Runtime 自检信息。

---

## POST /api/fkteams/login

登录获取 Token。接口始终注册，但仅在 `[server.auth] enabled = true` 时可用；未启用认证时返回 `404`。

**请求 Body**：

```json
{
  "username": "admin",
  "password": "your_password",
  "cookie_only": false
}
```

`cookie_only` 可选。Web UI 设置为 `true`，使 Token 不进入 JavaScript 响应；普通 API 客户端可省略并取得 Bearer Token。

**成功响应**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "hex_payload.hex_signature"
  }
}
```

无论是否启用 `cookie_only`，服务端都会设置有效期 7 天的 `HttpOnly`、`SameSite=Strict` Cookie；HTTPS 请求同时设置 `Secure`。Token payload 为 `username|expiry(RFC3339)`，签名为 HMAC-SHA256。

`cookie_only: true` 时成功响应不返回 Token：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "authenticated": true
  }
}
```

**失败响应**：

| 状态码 | message | 说明 |
| ------ | ------- | ---- |
| 400 | `请求格式错误` | 请求体解析失败 |
| 401 | `用户名或密码错误` | 凭证不匹配 |
| 429 | `too many authentication attempts` | 尝试次数过多，响应包含 `Retry-After` |

---

## POST /api/fkteams/logout

清除 `fk_token` Cookie。接口不要求有效 Token，因此登录已过期或认证配置刚刚变更时仍可调用。

---

## GET /api/fkteams/version

获取服务版本信息。

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "version": "0.0.1",
    "buildDate": "2026-06-10 16:53:39"
  }
}
```

---

## GET /api/fkteams/agents

获取所有可用智能体。

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "name": "coder",
      "description": "软件工程师",
      "aliases": ["code"]
    }
  ]
}
```

| 字段 | 说明 |
| ---- | ---- |
| `name` | 智能体名称 |
| `description` | 描述 |
| `aliases` | 别名数组，可能为空或省略 |

---

## GET /api/fkteams/favicon

来源图标代理。常用于 Web 前端展示搜索结果/引用来源 favicon。

**Query 参数**：

| 参数 | 类型 | 必填 | 说明 |
| ---- | ---- | ---- | ---- |
| `domain` | string | 否 | 域名 |
| `url` | string | 否 | 完整 URL；未提供 `domain` 时从 URL 提取域名 |
| `size` | int | 否 | 图标尺寸，默认由后端处理 |

**成功响应**：返回图标二进制，`Content-Type` 可能为 `image/x-icon`、`image/png`、`image/svg+xml` 等。

上游不可用时会返回 fallback SVG。

---

## POST /api/fkteams/ai/javascript/draft

根据自然语言生成 JavaScript 工具或流程 Hook 草稿。接口使用默认对话模型，返回前会校验结构、goja 语法和入口函数。

生成工具：

```json
{
  "kind": "tool",
  "instruction": "创建一个统计文本字符数的只读工具",
  "existing_ids": ["other_tool"]
}
```

生成 Hook 时将 `kind` 改为 `hook`。重写现有草稿可额外传入 `current_tool` 或 `current_hook`。返回的扩展默认不启用，客户端应让用户审查后再写入配置。

---

## POST /api/fkteams/shutdown

请求服务优雅关闭。仅当生命周期管理器可用时成功。

**成功响应**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "server is shutting down"
  }
}
```

**失败响应**：

| 状态码 | message |
| ------ | ------- |
| 500 | `shutdown not available` |

---

## POST /api/fkteams/restart

请求服务优雅重启。仅当生命周期管理器可用时成功。

**成功响应**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "server is restarting"
  }
}
```

**失败响应**：

| 状态码 | message |
| ------ | ------- |
| 500 | `restart not available` |

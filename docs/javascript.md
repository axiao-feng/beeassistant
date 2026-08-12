# JavaScript 扩展

fkteams 使用 [dop251/goja](https://github.com/dop251/goja) 执行用户脚本。打开 Web 配置页的“脚本”页签，可以手写或通过 AI 生成两类扩展：

- JavaScript 工具：注册为模型可发现、可调用的新工具。
- 流程 Hook：在运行、事件、工具调用和模型请求边界观察或控制流程。

AI 生成的草稿会先在服务端完成语法和入口函数校验，应用后仍需检查内容、启用扩展并保存配置。配置保存后会清理智能体运行缓存，新的工具和 Hook 无需重启服务即可生效。

## 自定义工具

每个工具必须提供唯一 ID、用途描述、参数 JSON Schema 和 `execute(input, context)` 函数：

```javascript
function execute(input, context) {
  const text = String(input.text || "");
  return {
    characters: Array.from(text).length,
    words: text.trim() ? text.trim().split(/\s+/).length : 0,
    call_id: context.call_id,
  };
}
```

对应的参数 Schema：

```json
{
  "type": "object",
  "properties": {
    "text": {
      "type": "string",
      "description": "需要统计的文本"
    }
  },
  "required": ["text"]
}
```

`input` 是模型生成的参数对象。`context` 包含调用元数据和按权限开放的宿主能力：

| API | 权限 | 说明 |
| --- | --- | --- |
| `context.tools.call(group, tool, args)` | `tools:<group>` 或 `tools:<group>/<tool>` | 同步调用现有内置、MCP 或其他工具组中的工具 |
| `context.storage.get(key)` | `storage` | 读取当前脚本命名空间中的 JSON 值，不存在时返回 `null` |
| `context.storage.set(key, value)` | `storage` | 原子持久化 JSON 值 |
| `context.storage.delete(key)` | `storage` | 删除值并返回是否存在 |
| `context.storage.keys()` | `storage` | 返回当前脚本的有序键列表 |
| `context.events.notice(message, level)` | `events:notice` | 向当前任务发送 `info`、`warn` 或 `error` 通知 |
| `context.log.debug/info/warn/error(value)` | 无需额外权限 | 写入带脚本 ID 的服务日志 |

顶层元数据包括 `tool_name`、`call_id`、`session_id` 和 `permissions`。返回值可以是字符串、数字、布尔值、数组或对象；非字符串值会序列化为 JSON 后返回给模型。

下面的工具读取工作区文件并保存调用次数，需要授予 `tools:file/file_read` 和 `storage`：

```javascript
function execute(input, context) {
  const count = (context.storage.get("count") || 0) + 1;
  context.storage.set("count", count);
  const file = context.tools.call("file", "file_read", { path: input.path });
  return { count, file };
}
```

工具组权限适合需要组合一组能力的脚本，例如 `tools:file`；精确工具权限适合最小授权，例如 `tools:file/file_read`。不允许调用 `javascript` 工具组，避免脚本递归创建失控的调用链。脚本调用现有工具时仍会经过工具 Hook、安全策略、审批和工作区路径限制。

启用工具并保存后，内置协调者会自动加载 `javascript` 工具组。自定义智能体可以在“智能体”页签中显式添加“JavaScript 扩展”工具组。

## 流程 Hooks

Hook 必须定义 `handle(hook)`。返回对象支持以下字段：

| 字段 | 说明 |
| --- | --- |
| `action` | `continue`、`skip` 或 `reject` |
| `message` | 拒绝等动作的说明 |
| `payload` | 改写后的载荷；需要修改时应显式返回 |

例如，在命令执行前拒绝包含 `sudo` 的参数：

```javascript
function handle(hook) {
  if (hook.payload.tool_name !== "execute") {
    return { action: "continue" };
  }
  const args = JSON.parse(hook.payload.args || "{}");
  if (/\bsudo\b/.test(String(args.command || ""))) {
    return { action: "reject", message: "sudo is not allowed" };
  }
  return { action: "continue" };
}
```

可用扩展点：

| 扩展点 | 载荷 | 可改写内容 |
| --- | --- | --- |
| `before_run` | `{input:{context,message}}` | 本轮输入 |
| `after_run` | `{input,result,error}` | 仅观察 |
| `on_event` | `{event}` | 事件或跳过分发 |
| `before_tool_call` | `{tool_name,args,meta}` | JSON 参数字符串，或拒绝调用 |
| `after_tool_call` | `{tool_name,args,result,error,meta}` | 仅观察 |
| `before_model_request` | `{messages,meta}` | 发给模型的消息 |
| `after_model_response` | `{message,usage,error,meta}` | 仅观察 |

`hook` 顶层还包含 `point`、`session_id`、`run_id` 和 `turn_id`。多个 Hook 按优先级从小到大执行，前一个 Hook 返回的载荷会传给下一个 Hook。

错误策略支持 `fail`、`warn` 和 `ignore`。未填写时，运行前扩展点默认失败即中止；事件和运行后扩展点默认记录警告并继续。

## 执行边界

- 每次调用都使用独立 goja Runtime，不共享脚本全局状态。
- 脚本最大 64 KiB，单次默认超时 200 毫秒。工具脚本可配置 10–120000 毫秒，便于等待网络或命令工具；流程 Hook 最多 5000 毫秒，避免长时间阻塞关键边界。
- 超时或上下文取消会通过 goja Interrupt 中断脚本。
- 不直接提供 `fetch`、`require`、`process`、文件系统、网络、定时器或其他 Node.js/宿主 API；需要这些能力时应显式授权并组合现有 `fetch`、`file`、`command`、MCP 等工具组。
- 持久化状态按脚本 ID 隔离，单个脚本最多 256 个键、1 MiB 数据，删除脚本不会自动删除其状态文件。
- 当前只支持同步脚本，不要使用 `async`、Promise 或异步回调。
- JavaScript 配置属于管理能力。对外开放 Web 服务时应启用认证，并只允许可信管理员编辑或启用脚本。

AI 生成只能减少样板工作，不能替代代码审查。启用前应检查循环规模、正则表达式复杂度、参数边界和返回数据大小。

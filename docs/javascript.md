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

`input` 是模型生成的参数对象。`context` 只包含 `tool_name` 和 `call_id`。返回值可以是字符串、数字、布尔值、数组或对象；非字符串值会序列化为 JSON 后返回给模型。

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
- 脚本最大 64 KiB，单次默认超时 200 毫秒，可配置范围为 10–5000 毫秒。
- 超时或上下文取消会通过 goja Interrupt 中断脚本。
- 不提供 `fetch`、`require`、`process`、文件系统、网络、定时器或其他 Node.js/宿主 API。
- 当前只支持同步脚本，不要使用 `async`、Promise 或异步回调。
- JavaScript 配置属于管理能力。对外开放 Web 服务时应启用认证，并只允许可信管理员编辑或启用脚本。

AI 生成只能减少样板工作，不能替代代码审查。启用前应检查循环规模、正则表达式复杂度、参数边界和返回数据大小。

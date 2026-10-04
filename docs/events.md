# 事件协议

蜜蜂助手的 CLI、Web、HTTP Stream、WebSocket 和聊天通道共用统一事件协议。事件由 `event_id`、`sequence`、`created_at` 标识顺序和时间，由 `type` 表示生命周期节点：`agent_started/completed`、`turn_started/completed`、`assistant_*`、`tool_call_*`、`ask_*`、`approval_*`、`member_*`、`system_notice`、`usage_reported`、`error`。

## 核心约定

- 事件核心实现位于 `internal/runtime/events`；根 `events` 包只保留外层入口使用的导出门面和类型别名，`internal/**` 不得导入根门面。
- 运行时适配器通过 `internal/runtime/events.Emitter` 和事件构造函数发出生命周期事件；适配器负责把底层框架事件翻译为协议事件，不直接把结构体字段拼装逻辑扩散到入口层。
- 所有前端可见事件必须先成为 `internal/domain/event.Event`，再经过 `internal/runtime/events.NormalizeEvent` 标准化，最后由 HTTP 边界转换为 JSON DTO。入口层不得直接拼装裸 `map[string]any` 作为业务事件推送给前端。
- 服务端业务顺序只使用 `sequence`。`stream_event_id` 仅用于 SSE/WebSocket 重连 offset，不参与业务排序；历史读取也必须返回标准 `sequence`，不得再生成独立的展示排序字段。
- `event_id`、`sequence`、`created_at`、`session_id`、`type` 是所有前端可见服务端事件的公共 envelope。运行中任务事件还必须尽量携带当前 `run_id` 和 `turn_id`；无法归属到某次用户提问的纯会话级事件需要在协议中显式说明。
- 流式分片事件只表示增量，不代表任务完成；消费者需要等待 `assistant_completed`、`tool_call_completed`、`turn_completed` 等完整事件或会话收尾后再归档结果。
- 助手实时输出拆为 `assistant_reasoning_delta` 和 `assistant_text_delta`；增量载荷只使用 `content`。历史 transcript 不保存这些增量事件，模型返回内容按语义聚合为 `agent_step` 或 `assistant_message`。
- 工具调用优先使用 `tool_call_ref` 关联；流式 `tool_call_arguments_delta`、`assistant_completed.tool_calls[]`、`tool_call_started`、`tool_call_result_delta`、`tool_call_completed` 必须保持同一个 ref，`tool_call_id` 和 `tool_call_index` 仅作为辅助身份。
- 用户提问/回答使用 `ask_requested` 和 `ask_answered`，问题、选项、选择结果进入 `ask` 载荷并同步展开为 HTTP 字段。
- 展示端必须遍历 `tool_calls[]`；单个工具调用事件可以携带 `tool_call` 作为当前调用对象，但协议入口仍以事件类型和 `tool_call_ref` 为准。
- AgentTool 必须在工具调用事件中带上 `kind=agent`、`display_name`、`target`，展示端不得通过工具名称前缀判断成员工具。
- 运行中子智能体事件通过 `parent_tool_call_id` 表示父级 AgentTool 调用归属，终端和网页不依赖智能体名称、工具名称前缀或相邻事件猜测成员关系。
- 展示端应优先使用事件中的 `tool_name`、`member_name` 等结构化字段，`detail` 仅作为补充展示数据。

## 前端消费规则

- 实时流和历史读取返回同构的事件数组；前端不区分历史事件和实时事件。
- 服务端事件按 `sequence` 排序。提交请求后、服务端正式事件返回前的本地乐观态只能作为前端临时视图状态，不写入服务端事件对象；一旦服务端事件到达，展示顺序以服务端 `sequence` 为准。
- 消息按 `message_id` 归并，流式块按 `block_id` 归并，工具调用按 `tool_call_ref` 归并，子智能体任务按 `parent_tool_call_id` 归并。
- 子智能体任务通过 `parent_tool_call_id` 挂载到父级 AgentTool；展示端不得通过智能体名称、工具名称前缀或相邻事件猜测父子关系。

## 会话历史

会话历史使用 `sessions/<session-id>/transcript.jsonl` 作为 append-only 事实源。文件行顺序就是提交顺序，服务端读取历史时不再使用 `message_id`、`event_index` 或 `sequence` 做排序补救。

主 transcript 只记录用户、coordinator 主线和父级工具调用。子智能体本身就是 `ask_fkagent_*` 工具调用：最终结果以父级 `tool_call_end` 进入 coordinator 上下文；子智能体内部执行轨迹写入 `sessions/<session-id>/subagents/<agent-run-id>/transcript.jsonl`，归属信息写入同目录 `metadata.json`，不重复写入每一行 transcript。

每行 transcript 记录只保存持久化事实字段：

- `id`: 类型前缀加 UUID，如 `msg_<uuid>`、`tool_<uuid>`、`ask_<uuid>`。
- `at`: 记录提交时间。
- `type`: transcript 记录类型，如 `user_message`、`agent_step`、`assistant_message`、`tool_call_start`、`tool_call_end`。
- `agent`: 记录所属智能体。用户消息不写 `agent:"user"`。
- `content`、`reasoning`、`usage`、`ask`、`error`: 按记录类型出现的结构化事实字段，不再包一层通用 `payload`。
- `call_id`、`name`、`args`、`display`、`kind`、`target`: 工具调用 start 记录的扁平字段；end 记录只保留 `call_id` 和结果字段。

`agent_step` 表示模型返回的内部执行步骤。原则上所有模型返回内容都落盘：有用户可见 `content` 的记录写为 `assistant_message`，只有 `reasoning`/`usage` 的记录写为 `agent_step`。

长工具结果不直接写入 transcript，保存到 `tool-results/<result-id>.json`，transcript 只保留 `result_ref`、`summary`、`truncated` 和 `original_chars`。展示、分享和模型上下文均由 transcript 投影生成；coordinator 上下文保留自己的文本、思考和工具链，子智能体只通过父级工具结果进入 coordinator 上下文，内部思考和内部工具轨迹不注入。

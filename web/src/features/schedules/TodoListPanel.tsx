import { Check, Circle, Plus, RefreshCcw, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { createTodo, deleteTodo, listTodos, updateTodo } from "@/api/todos";
import { appActions } from "@/app/store";
import { useAppDispatch } from "@/app/hooks";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Panel, PanelBody, PanelHeader } from "@/components/ui/panel";
import { cn } from "@/lib/cn";
import type { PersonalTodo } from "@/types/todos";

export function TodoListPanel() {
  const dispatch = useAppDispatch();
  const [todos, setTodos] = useState<PersonalTodo[]>([]);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [keyword, setKeyword] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const visibleTodos = useMemo(() => {
    const query = keyword.trim().toLowerCase();
    return [...todos]
      .filter((todo) => !query || `${todo.title} ${todo.description || ""}`.toLowerCase().includes(query))
      .sort((left, right) => Number(left.completed) - Number(right.completed) || (right.updated_at || "").localeCompare(left.updated_at || ""));
  }, [todos, keyword]);

  async function load() {
    setLoading(true);
    try {
      const result = await listTodos();
      setTodos(result.todos || []);
    } catch (error) {
      dispatch(appActions.showToast(error instanceof Error ? error.message : "加载待办失败"));
    } finally {
      setLoading(false);
    }
  }

  async function addTodo() {
    const nextTitle = title.trim();
    if (!nextTitle || saving) return;
    setSaving(true);
    try {
      const result = await createTodo(nextTitle, description.trim());
      setTodos((current) => [result.todo, ...current]);
      setTitle("");
      setDescription("");
    } catch (error) {
      dispatch(appActions.showToast(error instanceof Error ? error.message : "新增待办失败"));
    } finally {
      setSaving(false);
    }
  }

  async function toggleTodo(todo: PersonalTodo) {
    try {
      const result = await updateTodo(todo.id, { completed: !todo.completed });
      if (result.todo.completed) {
        setTodos((current) => current.filter((item) => item.id !== todo.id));
        dispatch(appActions.showToast("待办已完成并清除"));
      } else {
        setTodos((current) => current.map((item) => (item.id === todo.id ? result.todo : item)));
      }
    } catch (error) {
      dispatch(appActions.showToast(error instanceof Error ? error.message : "更新待办失败"));
    }
  }

  async function removeTodo(todo: PersonalTodo) {
    try {
      await deleteTodo(todo.id);
      setTodos((current) => current.filter((item) => item.id !== todo.id));
    } catch (error) {
      dispatch(appActions.showToast(error instanceof Error ? error.message : "删除待办失败"));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const completed = todos.filter((todo) => todo.completed).length;
  return (
    <Panel>
      <PanelHeader className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div>
          <div className="flex items-center gap-3">
            <Check className="h-5 w-5 text-primary" />
            <h2 className="text-xl font-semibold">个人待办清单</h2>
          </div>
          <div className="mt-1 text-sm text-muted-foreground">随手记录要做的事，完成后直接勾选。</div>
        </div>
        <div className="flex w-full gap-2 xl:w-[420px]">
          <Input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="搜索待办" />
          <Button variant="outline" onClick={() => void load()} disabled={loading} aria-label="刷新待办">
            <RefreshCcw className="h-4 w-4" />
            刷新
          </Button>
        </div>
      </PanelHeader>
      <PanelBody className="grid gap-4 border-t border-border/70 xl:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
        <div className="space-y-2">
          {visibleTodos.length ? visibleTodos.map((todo) => (
            <div key={todo.id} className="flex items-start gap-3 rounded-lg border border-border/75 bg-card/60 px-3 py-3">
              <button type="button" className="mt-0.5 shrink-0 text-primary" onClick={() => void toggleTodo(todo)} aria-label={todo.completed ? "标记为未完成" : "标记为完成"}>
                {todo.completed ? <Check className="h-5 w-5" /> : <Circle className="h-5 w-5 text-muted-foreground" />}
              </button>
              <div className="min-w-0 flex-1">
                <div className={cn("break-words text-sm font-medium", todo.completed && "text-muted-foreground line-through")}>{todo.title}</div>
                {todo.description ? <div className="mt-1 whitespace-pre-wrap break-words text-xs text-muted-foreground">{todo.description}</div> : null}
              </div>
              <button type="button" className="shrink-0 text-muted-foreground hover:text-destructive" onClick={() => void removeTodo(todo)} aria-label="删除待办">
                <Trash2 className="h-4 w-4" />
              </button>
            </div>
          )) : <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">{loading ? "正在加载" : "还没有个人待办"}</div>}
          <div className="pt-1 text-xs text-muted-foreground">已完成 {completed} / {todos.length}</div>
        </div>
        <div className="space-y-3 rounded-xl border border-border/75 bg-muted/20 p-4">
          <div className="font-semibold">新增待办</div>
          <Input value={title} onChange={(event) => setTitle(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") void addTodo(); }} placeholder="例如：整理项目文档" />
          <Textarea value={description} onChange={(event) => setDescription(event.target.value)} placeholder="备注（可选）" className="min-h-24 text-sm" />
          <Button className="w-full" onClick={() => void addTodo()} disabled={!title.trim() || saving}>
            <Plus className="h-4 w-4" />
            {saving ? "保存中" : "添加待办"}
          </Button>
        </div>
      </PanelBody>
    </Panel>
  );
}

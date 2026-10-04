import type { PersonalTodo } from "@/types/todos";
import { del, get, patch, post } from "./client";

export function listTodos() {
  return get<{ todos: PersonalTodo[] }>("/api/fkteams/todos");
}

export function createTodo(title: string, description = "") {
  return post<{ todo: PersonalTodo }>("/api/fkteams/todos", { title, description });
}

export function updateTodo(id: string, payload: { completed?: boolean; title?: string; description?: string }) {
  return patch<{ todo: PersonalTodo }>(`/api/fkteams/todos/${encodeURIComponent(id)}`, payload);
}

export function deleteTodo(id: string) {
  return del<{ id: string }>(`/api/fkteams/todos/${encodeURIComponent(id)}`);
}

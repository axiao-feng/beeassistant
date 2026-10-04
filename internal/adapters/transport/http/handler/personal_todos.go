package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fkteams/internal/app/appdata"
	"fkteams/internal/runtime/atomicfile"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	personalTodoMaxItems = 10_000
	personalTodoMaxBytes = 8 << 20
)

// PersonalTodo 是网页端使用的全局个人待办事项，不绑定某个聊天会话。
type PersonalTodo struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type personalTodoList struct {
	Todos []PersonalTodo `json:"todos"`
}

// PersonalTodoStore 持久化全局个人待办事项。
type PersonalTodoStore struct {
	path string
	mu   sync.Mutex
}

func NewPersonalTodoStore(root string) *PersonalTodoStore {
	return &PersonalTodoStore{path: filepath.Join(root, "todos", "personal.json")}
}

func (s *PersonalTodoStore) loadLocked() (*personalTodoList, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return &personalTodoList{Todos: []PersonalTodo{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取个人待办失败: %w", err)
	}
	if len(data) > personalTodoMaxBytes {
		return nil, fmt.Errorf("个人待办数据超过大小限制")
	}
	var list personalTodoList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("解析个人待办失败: %w", err)
	}
	if len(list.Todos) > personalTodoMaxItems {
		return nil, fmt.Errorf("个人待办数量超过限制")
	}
	if list.Todos == nil {
		list.Todos = []PersonalTodo{}
	}
	return &list, nil
}

func (s *PersonalTodoStore) saveLocked(list *personalTodoList) error {
	if len(list.Todos) > personalTodoMaxItems {
		return fmt.Errorf("个人待办数量超过限制")
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化个人待办失败: %w", err)
	}
	if len(data) > personalTodoMaxBytes {
		return fmt.Errorf("个人待办数据超过大小限制")
	}
	if err := atomicfile.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("保存个人待办失败: %w", err)
	}
	return nil
}

func (s *PersonalTodoStore) List() ([]PersonalTodo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	active := list.Todos[:0]
	for _, todo := range list.Todos {
		if !todo.Completed {
			active = append(active, todo)
		}
	}
	if len(active) != len(list.Todos) {
		list.Todos = active
		if err := s.saveLocked(list); err != nil {
			return nil, err
		}
	}
	return list.Todos, nil
}

func (s *PersonalTodoStore) Add(title, description string) (PersonalTodo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.loadLocked()
	if err != nil {
		return PersonalTodo{}, err
	}
	if len(list.Todos) >= personalTodoMaxItems {
		return PersonalTodo{}, fmt.Errorf("个人待办数量已达到上限")
	}
	now := time.Now()
	todo := PersonalTodo{ID: "todo_" + uuid.NewString(), Title: title, Description: description, CreatedAt: now, UpdatedAt: now}
	list.Todos = append(list.Todos, todo)
	return todo, s.saveLocked(list)
}

func (s *PersonalTodoStore) Update(id string, completed *bool, title, description *string) (PersonalTodo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.loadLocked()
	if err != nil {
		return PersonalTodo{}, err
	}
	for i := range list.Todos {
		if list.Todos[i].ID != id {
			continue
		}
		if completed != nil {
			list.Todos[i].Completed = *completed
		}
		if title != nil {
			list.Todos[i].Title = *title
		}
		if description != nil {
			list.Todos[i].Description = *description
		}
		list.Todos[i].UpdatedAt = time.Now()
		updatedTodo := list.Todos[i]
		if completed != nil && *completed {
			// 个人清单采用“完成即清除”策略，避免已完成事项长期堆积。
			list.Todos = append(list.Todos[:i], list.Todos[i+1:]...)
		}
		if err := s.saveLocked(list); err != nil {
			return PersonalTodo{}, err
		}
		return updatedTodo, nil
	}
	return PersonalTodo{}, fmt.Errorf("未找到待办事项 %s", id)
}

func (s *PersonalTodoStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.loadLocked()
	if err != nil {
		return err
	}
	for i := range list.Todos {
		if list.Todos[i].ID != id {
			continue
		}
		list.Todos = append(list.Todos[:i], list.Todos[i+1:]...)
		return s.saveLocked(list)
	}
	return fmt.Errorf("未找到待办事项 %s", id)
}

type personalTodoRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   *bool  `json:"completed"`
}

func GetPersonalTodosHandler(store *PersonalTodoStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		todos, err := store.List()
		if err != nil {
			Fail(c, 500, err.Error())
			return
		}
		OK(c, gin.H{"todos": todos})
	}
}

func CreatePersonalTodoHandler(store *PersonalTodoStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req personalTodoRequest
		if err := c.ShouldBindJSON(&req); err != nil || len([]rune(req.Title)) == 0 {
			Fail(c, 400, "待办标题不能为空")
			return
		}
		todo, err := store.Add(req.Title, req.Description)
		if err != nil {
			Fail(c, 500, err.Error())
			return
		}
		Created(c, gin.H{"todo": todo})
	}
}

func UpdatePersonalTodoHandler(store *PersonalTodoStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req personalTodoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, 400, "请求参数无效")
			return
		}
		title := (*string)(nil)
		description := (*string)(nil)
		if req.Title != "" {
			title = &req.Title
		}
		if req.Description != "" {
			description = &req.Description
		}
		todo, err := store.Update(c.Param("id"), req.Completed, title, description)
		if err != nil {
			Fail(c, 404, err.Error())
			return
		}
		OK(c, gin.H{"todo": todo})
	}
}

func DeletePersonalTodoHandler(store *PersonalTodoStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.Delete(c.Param("id")); err != nil {
			Fail(c, 404, err.Error())
			return
		}
		OK(c, gin.H{"id": c.Param("id")})
	}
}

func defaultPersonalTodoStore() *PersonalTodoStore {
	return NewPersonalTodoStore(appdata.Dir())
}

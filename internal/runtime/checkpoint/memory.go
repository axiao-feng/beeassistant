package checkpoint

import (
	"context"
	"sync"
)

// NewMemoryStore 创建进程内 checkpoint 存储。
func NewMemoryStore() Store {
	return &MemoryStore{
		mem: map[string][]byte{},
	}
}

// MemoryStore 是线程安全的进程内 checkpoint 存储。
type MemoryStore struct {
	mu  sync.RWMutex
	mem map[string][]byte
}

func (s *MemoryStore) Set(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem[key] = append([]byte(nil), value...)
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.mem[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), v...), true, nil
}

// Delete 删除已结束运行的检查点，避免复用 Runner 时积累失效状态。
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mem, key)
	return nil
}

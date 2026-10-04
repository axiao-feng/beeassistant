package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fkteams/internal/app/appdata"
	"fkteams/internal/runtime/atomicfile"

	"github.com/gin-gonic/gin"
)

const personalProfileMaxBytes = 256 << 10

// PersonalProfile 是用户主动维护的个人资料，不会把敏感凭据写入配置文件。
type PersonalProfile struct {
	Name       string    `json:"name"`
	Occupation string    `json:"occupation"`
	Location   string    `json:"location"`
	Goals      string    `json:"goals"`
	Notes      string    `json:"notes"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PersonalProfileStore 持久化全局个人资料。
type PersonalProfileStore struct {
	path string
	mu   sync.Mutex
}

func NewPersonalProfileStore(root string) *PersonalProfileStore {
	return &PersonalProfileStore{path: filepath.Join(root, "profile", "personal.json")}
}

func (s *PersonalProfileStore) loadLocked() (PersonalProfile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return PersonalProfile{}, nil
	}
	if err != nil {
		return PersonalProfile{}, fmt.Errorf("读取个人资料失败: %w", err)
	}
	if len(data) > personalProfileMaxBytes {
		return PersonalProfile{}, fmt.Errorf("个人资料数据超过大小限制")
	}
	var profile PersonalProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return PersonalProfile{}, fmt.Errorf("解析个人资料失败: %w", err)
	}
	return profile, nil
}

func (s *PersonalProfileStore) saveLocked(profile PersonalProfile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化个人资料失败: %w", err)
	}
	if len(data) > personalProfileMaxBytes {
		return fmt.Errorf("个人资料数据超过大小限制")
	}
	if err := atomicfile.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("保存个人资料失败: %w", err)
	}
	return nil
}

func (s *PersonalProfileStore) Get() (PersonalProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *PersonalProfileStore) Save(profile PersonalProfile) (PersonalProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Occupation = strings.TrimSpace(profile.Occupation)
	profile.Location = strings.TrimSpace(profile.Location)
	profile.Goals = strings.TrimSpace(profile.Goals)
	profile.Notes = strings.TrimSpace(profile.Notes)
	profile.UpdatedAt = time.Now()
	if err := s.saveLocked(profile); err != nil {
		return PersonalProfile{}, err
	}
	return profile, nil
}

func GetPersonalProfileHandler(store *PersonalProfileStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		profile, err := store.Get()
		if err != nil {
			Fail(c, 500, err.Error())
			return
		}
		OK(c, profile)
	}
}

func SavePersonalProfileHandler(store *PersonalProfileStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var profile PersonalProfile
		if err := c.ShouldBindJSON(&profile); err != nil {
			Fail(c, 400, "个人资料参数无效")
			return
		}
		saved, err := store.Save(profile)
		if err != nil {
			Fail(c, 500, err.Error())
			return
		}
		OK(c, saved)
	}
}

func defaultPersonalProfileStore() *PersonalProfileStore {
	return NewPersonalProfileStore(appdata.Dir())
}

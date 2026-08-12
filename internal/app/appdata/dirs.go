package appdata

import (
	"os"
	"path/filepath"

	"fkteams/internal/runtime/env"
)

// Dir 返回应用数据目录，支持 FEIKONG_APP_DIR 环境变量覆盖。
func Dir() string {
	if d := env.Get(env.AppDir); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".fkteams"
	}
	return filepath.Join(home, ".fkteams")
}

// SessionsDir 返回会话历史存储目录。
func SessionsDir() string {
	return filepath.Join(Dir(), "sessions")
}

// WorkspaceDir 返回工作目录。
func WorkspaceDir() string {
	return filepath.Join(Dir(), "workspace")
}

// SchedulerDir 返回定时任务调度器数据目录。
func SchedulerDir() string {
	return filepath.Join(Dir(), "scheduler")
}

// ShareDir 返回文件分享链接持久化目录。
func ShareDir() string {
	return filepath.Join(Dir(), "share")
}

// RuntimeDir 返回运行时临时数据目录。
func RuntimeDir() string {
	return filepath.Join(Dir(), "runtime")
}

// JavaScriptDir 返回 JavaScript 扩展持久化数据目录。
func JavaScriptDir() string {
	return filepath.Join(Dir(), "javascript")
}

// SkillsDir 返回 Skills 安装目录。
func SkillsDir() string {
	return filepath.Join(Dir(), "skills")
}

// ConfigFile 返回主配置文件路径。
func ConfigFile() string {
	return filepath.Join(Dir(), "config", "config.toml")
}

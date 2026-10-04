package runtime

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const maxPipeInputBytes int64 = 32 << 20

// ReadPipeInput 检测 stdin 是否为管道并读取内容。
func ReadPipeInput() (content string, isPipe bool, err error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", false, fmt.Errorf("inspect stdin: %w", err)
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "", false, nil
	}
	data, err := readLimitedInput(os.Stdin, maxPipeInputBytes)
	if err != nil {
		return "", true, err
	}
	return strings.TrimSpace(string(data)), true, nil
}

func readLimitedInput(reader io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, fmt.Errorf("input limit must not be negative")
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("stdin exceeds %d bytes", limit)
	}
	return data, nil
}

// ExtractAgentMention 提取输入中的智能体 @ 提及。
func ExtractAgentMention(input string) (agentName string, query string) {
	input = strings.TrimSpace(input)
	re := regexp.MustCompile(`^@([\p{Han}\w]+)\s*(.*)$`)
	matches := re.FindStringSubmatch(input)
	if len(matches) == 3 {
		return matches[1], strings.TrimSpace(matches[2])
	}
	return "", input
}

// WorkMode 工作模式
type WorkMode string

const (
	ModeGeneralist WorkMode = "generalist"
	ModeTeam       WorkMode = "team"
	ModeDeep       WorkMode = "deep"
	ModeGroup      WorkMode = "group"
)

// String 返回模式字符串
func (m WorkMode) String() string {
	return string(m)
}

// GetPromptPrefix 获取提示符前缀
func (m WorkMode) GetPromptPrefix() string {
	switch m {
	case ModeGeneralist:
		return "个人助手模式> "
	case ModeTeam:
		return "团队模式> "
	case ModeDeep:
		return "深度模式> "
	case ModeGroup:
		return "多智能体讨论模式> "
	default:
		return "未知模式> "
	}
}

// ParseWorkMode 解析工作模式
func ParseWorkMode(mode string) WorkMode {
	switch mode {
	case "team":
		return ModeTeam
	case "deep":
		return ModeDeep
	case "group":
		return ModeGroup
	default:
		return ModeGeneralist
	}
}

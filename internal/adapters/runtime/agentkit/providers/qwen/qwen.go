package qwen

import (
	"context"

	qwenModel "github.com/cloudwego/eino-ext/components/model/qwen"

	"fkteams/internal/adapters/model/providers/providerkit"
	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	runtimeport "fkteams/internal/ports/runtime"
)

// New 创建阿里通义千问的聊天模型
func New(ctx context.Context, cfg *providerkit.Config) (runtimeport.ChatModel, error) {
	chatModel, err := qwenModel.NewChatModel(ctx, &qwenModel.ChatModelConfig{
		APIKey:     cfg.APIKey,
		BaseURL:    cfg.BaseURL,
		Model:      cfg.Model,
		HTTPClient: providerkit.HTTPClientWithHeaders(cfg.ExtraHeaders),
	})
	if err != nil {
		return nil, err
	}
	return kitruntime.WrapChatModel(chatModel), nil
}

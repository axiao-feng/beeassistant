package openrouter

import (
	"context"

	openrouterModel "github.com/cloudwego/eino-ext/components/model/openrouter"

	"fkteams/internal/adapters/model/providers/providerkit"
	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	runtimeport "fkteams/internal/ports/runtime"
)

// New 创建 OpenRouter 的聊天模型
func New(ctx context.Context, cfg *providerkit.Config) (runtimeport.ChatModel, error) {
	modelCfg := &openrouterModel.Config{
		APIKey:     cfg.APIKey,
		Model:      cfg.Model,
		HTTPClient: providerkit.HTTPClientWithHeaders(cfg.ExtraHeaders),
	}
	if cfg.BaseURL != "" {
		modelCfg.BaseURL = cfg.BaseURL
	}
	chatModel, err := openrouterModel.NewChatModel(ctx, modelCfg)
	if err != nil {
		return nil, err
	}
	return kitruntime.WrapChatModel(chatModel), nil
}

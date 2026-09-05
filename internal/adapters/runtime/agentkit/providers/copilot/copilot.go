package copilot

import (
	"context"
	modelcopilot "fkteams/internal/adapters/model/providers/copilot"
	"fkteams/internal/adapters/model/providers/providerkit"
	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	runtimeport "fkteams/internal/ports/runtime"

	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
)

// New 创建 Copilot 聊天模型（OpenAI 兼容）
func New(ctx context.Context, cfg *providerkit.Config) (runtimeport.ChatModel, error) {
	tm := modelcopilot.GetTokenManager()

	// 确保有有效 token
	if _, err := tm.GetToken(ctx); err != nil {
		return nil, err
	}

	modelCfg := &openaiModel.ChatModelConfig{
		BaseURL:    modelcopilot.BaseURL(),
		Model:      cfg.Model,
		HTTPClient: modelcopilot.NewHTTPClient(),
	}
	chatModel, err := openaiModel.NewChatModel(ctx, modelCfg)
	if err != nil {
		return nil, err
	}
	return kitruntime.WrapChatModel(chatModel), nil
}

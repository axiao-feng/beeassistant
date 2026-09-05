package ollama

import (
	"context"

	ollamaModel "github.com/cloudwego/eino-ext/components/model/ollama"

	"fkteams/internal/adapters/model/providers/providerkit"
	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	runtimeport "fkteams/internal/ports/runtime"
)

// New 创建 Ollama 本地模型的聊天模型
func New(ctx context.Context, cfg *providerkit.Config) (runtimeport.ChatModel, error) {
	chatModel, err := ollamaModel.NewChatModel(ctx, &ollamaModel.ChatModelConfig{
		BaseURL:    cfg.BaseURL,
		Model:      cfg.Model,
		HTTPClient: providerkit.HTTPClientWithHeaders(cfg.ExtraHeaders),
	})
	if err != nil {
		return nil, err
	}
	return kitruntime.WrapChatModel(chatModel), nil
}

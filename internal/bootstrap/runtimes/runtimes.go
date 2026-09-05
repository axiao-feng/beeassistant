package runtimes

import (
	modelproviders "fkteams/internal/adapters/model/providers"
	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	kitengine "fkteams/internal/adapters/runtime/agentkit/engine"
	kitproviders "fkteams/internal/adapters/runtime/agentkit/providers/register"
	toolmcp "fkteams/internal/adapters/tools/mcp"
	runtimeport "fkteams/internal/ports/runtime"
	modelregistry "fkteams/internal/runtime/model"
)

// Defaults 保存组合根创建的默认 runtime 依赖。
type Defaults struct {
	Runtime               runtimeport.Runtime
	Interrupt             runtimeport.InterruptRuntime
	ModelRegistry         *modelregistry.Registry
	ModelProviderRegistry *modelproviders.Registry
}

// Options 描述 runtime 组合根的显式外部依赖。
type Options struct {
	MCPProvider *toolmcp.Provider
}

// NewDefaults 显式创建默认 runtime adapter 和关联桥接能力。
func NewDefaults(options ...Options) (*Defaults, error) {
	var opt Options
	if len(options) > 0 {
		opt = options[0]
	}

	providerRegistry := modelproviders.NewRegistry()
	modelRegistry := modelregistry.NewRegistry()
	kitproviders.RegisterDefaults(providerRegistry, modelRegistry)

	engine := kitengine.NewEngine()

	if opt.MCPProvider != nil {
		opt.MCPProvider.RegisterToolProvider(engine.MCPTools)
	}

	return &Defaults{
		Runtime:               engine,
		Interrupt:             kitruntime.NewInterruptRuntime(),
		ModelRegistry:         modelRegistry,
		ModelProviderRegistry: providerRegistry,
	}, nil
}

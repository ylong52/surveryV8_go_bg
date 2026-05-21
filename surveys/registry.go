package surveys

// 业务注册表：根据配置的 survey_type 分发到对应的 handler

import (
	"fmt"

	"survey.cli/configs"
	"survey.cli/core"
	"survey.cli/core/logger"
	"survey.cli/core/storage"
	"survey.cli/surveys/macalester_2026_prospective_families"
)

// Registry 管理所有已注册的业务 handler
type Registry struct {
	handlers map[string]func(*core.SurveyCtx) core.SurveyRunner
}

// NewRegistry 构造注册表并注册所有已知业务
func NewRegistry() *Registry {
	r := &Registry{handlers: make(map[string]func(*core.SurveyCtx) core.SurveyRunner)}
	r.register()
	return r
}

// Resolve 根据 cfg.SurveyType 返回对应的 SurveyRunner
// @param cfg  全局配置（含 survey_type 字段）
// @returns    SurveyRunner 实现，或找不到时的错误
func (r *Registry) Resolve(cfg *configs.Config) (core.SurveyRunner, error) {
	factory, ok := r.handlers[cfg.SurveyType]
	if !ok {
		return nil, fmt.Errorf("unknown survey_type: %s", cfg.SurveyType)
	}
	ctx := &core.SurveyCtx{
		Config:  cfg,
		Log:     logger.Logs,
		Storage: storage.New(cfg.StoragePath),
	}
	return factory(ctx), nil
}

// register 注册所有已知业务 handler（新增业务在此追加一行）
func (r *Registry) register() {
	r.handlers["macalester_2026_prospective_families"] = func(ctx *core.SurveyCtx) core.SurveyRunner { return macalester_2026_prospective_families.New(ctx) }
}

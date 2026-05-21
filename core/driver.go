package core

// SurveyCtx 携带整次任务的共享状态；SurveyRunner 是各业务实现的统一接口

import (
	"survey.cli/configs"
	"survey.cli/core/logger"
	"survey.cli/core/storage"
)

// SurveyCtx 单次问卷任务的运行上下文
type SurveyCtx struct {
	Config   *configs.Config
	Log      *logger.Logger
	Storage  *storage.Storage
	SurveyID string
}

// SurveyRunner 每种业务类型实现此接口
type SurveyRunner interface {
	// Run 执行一次完整的问卷处理流程
	// @returns error 执行过程中的致命错误
	Run() error
}

// NewCtx 构造运行上下文（依赖注入入口）
func NewCtx(cfg *configs.Config, lg *logger.Logger) *SurveyCtx {
	return &SurveyCtx{
		Config:  cfg,
		Log:     lg,
		Storage: storage.New(cfg.StoragePath),
	}
}

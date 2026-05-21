package macalester_2026_prospective_families

// Macalester 2026 Prospective Families 的具体实现

import (
	"survey.cli/core"
)

// Handler 业务 A 的问卷处理器
type Handler struct {
	ctx *core.SurveyCtx
}

// New 构造业务 A 处理器
// @param ctx  运行时上下文
// @returns    实现了 core.SurveyRunner 的 Handler
func New(ctx *core.SurveyCtx) *Handler {
	return &Handler{ctx: ctx}
}

// Run 执行业务 A 的完整问卷处理流程
// @returns  执行过程中的致命错误
func (h *Handler) Run() error {
	h.ctx.Log.Info("macalester_2026_prospective_families: start")
	// TODO: 解析 HTML → 路由题型 → AI 推荐答案 → 存储
	h.ctx.Log.Info("macalester_2026_prospective_families: done")
	return nil
}

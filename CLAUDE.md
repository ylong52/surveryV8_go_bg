## 运行时控制 (Runtime Control)
配置项值AUTO_APPROVEtrueNON_INTERACTIVEtrueLOG_LEVELverbose
自动化策略

默认执行：Shell 命令、文件修改、环境配置，无需用户确认，直接执行。
复杂脚本：自动批准包含循环（for/while）、过滤（grep/sed）、多目录操作的 Bash 脚本。
冲突/报错：遇权限错误立即报告，不挂起等待；遇小范围报错（如缺 npm 包）直接修复（npm install）并继续。
执行前检查：执行复杂指令（大规模重构、自动化测试）前，静默检查当前 Git 分支状态和环境依赖。
输出简练：仅输出关键进度节点，隐藏冗长的中间过程。


## 编码规范 (Coding Standards)
文件与类结构

按业务拆分独立类文件，一个类只负责一件事（单一职责）。
业务逻辑与工具方法分离，互不嵌套，保持解耦。
固定配置、常量统一放在类内常量或专用配置类中，不散落在业务代码里。

## 命名规范

类、方法、变量命名直白易懂，见名知意，不用缩写或晦涩写法。

## 方法设计

方法精简，单一功能，不写超大方法。
关键方法必须加注释，标注：用途、入参、出参。

/**
 * 用途：xxx
 * @param foo  入参说明
 * @returns    出参说明
 */
代码风格

排版规整，层级清晰。
不使用晦涩写法，优先选择可读性更高的表达方式。


## 异常处理 (Error Handling)
- 严禁在循环中重复询问相同问题。
- 连续三次执行失败时，停止当前子任务，记录错误日志，并自动转入下一个可执行任务。

 
## Configuration / 配置

所有配置通过 `.env` 文件（参见 `.env` 参考）。目标：AI 配置逐步迁移至 `config/ai_config.go`。

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `DEEPSEEK_API_KEY` | — | DeepSeek API 密钥 |
| `DEEPSEEK_MODEL` | `deepseek-chat` | AI 模型 |
| `VISION_PROVIDER` | `volces` | 视觉后端 (volces/qwen_vl/gpt4v/gemini) |
| `HTTP_PORT` | `8080` | HTTP 服务器端口 |
| `SCOUT_WS_PORT` | `5001` | WebSocket 端口 |


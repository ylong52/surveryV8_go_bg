package main

// 程序启动点，依赖注入容器

import (
	"log"

	"survey.cli/configs"
	"survey.cli/core/logger"
	"survey.cli/surveys"
)

func main() {
	cfg := configs.Load()
	lg := logger.Init(cfg)

	runner, err := surveys.NewRegistry().Resolve(cfg)
	if err != nil {
		lg.Error("resolve runner failed: %v", err)
		log.Fatal(err)
	}

	if err := runner.Run(); err != nil {
		lg.Error("runner failed: %v", err)
	}
}

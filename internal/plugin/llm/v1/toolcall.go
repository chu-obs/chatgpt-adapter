package v1

import (
	"github.com/bincooo/chatgpt-adapter/v2/internal/gin.handler/response"
	"github.com/bincooo/chatgpt-adapter/v2/internal/plugin"
	"github.com/bincooo/chatgpt-adapter/v2/logger"
	"github.com/bincooo/chatgpt-adapter/v2/pkg"
	"github.com/gin-gonic/gin"
)

func completeToolCalls(ctx *gin.Context, req pkg.ChatCompletion) bool {
	logger.Info("completeTools ...")
	exec, err := plugin.CompleteToolCalls(ctx, req, func(message string) (string, error) {
		messages, _ := mergeMessages([]pkg.Keyv[interface{}]{
			{"role": "system", "content": message},
		})

		r, err := fetchGpt35(ctx, messages)
		if err != nil {
			return "", err
		}

		return waitMessage(r, plugin.ToolCallCancel)
	})

	if err != nil {
		response.Error(ctx, -1, err)
		return true
	}

	return exec
}

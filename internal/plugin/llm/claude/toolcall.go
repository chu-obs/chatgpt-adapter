package claude

import (
	"github.com/bincooo/chatgpt-adapter/v2/internal/common"
	"github.com/bincooo/chatgpt-adapter/v2/internal/gin.handler/response"
	"github.com/bincooo/chatgpt-adapter/v2/internal/plugin"
	"github.com/bincooo/chatgpt-adapter/v2/logger"
	"github.com/bincooo/chatgpt-adapter/v2/pkg"
	api "github.com/bincooo/claude-api"
	"github.com/bincooo/claude-api/types"
	"github.com/bincooo/claude-api/vars"
	"github.com/gin-gonic/gin"
	"strings"
)

func completeToolCalls(ctx *gin.Context, cookie, proxies string, completion pkg.ChatCompletion) bool {
	logger.Infof("completeTools ...")
	exec, err := plugin.CompleteToolCalls(ctx, completion, func(message string) (string, error) {
		model := vars.Model4WebClaude2
		if strings.HasPrefix(completion.Model, "claude-") {
			model = completion.Model
		}

		options := api.NewDefaultOptions(cookie, model)
		options.Proxies = proxies

		chat, err := api.New(options)
		if err != nil {
			return "", err
		}

		message = common.PadText(padMaxCount-len(message), message)
		chatResponse, err := chat.Reply(ctx.Request.Context(), "", []types.Attachment{
			{
				Content:  message,
				FileName: "paste.txt",
				FileSize: len(message),
				FileType: "text/plain",
			},
		})
		if err != nil {
			return "", err
		}

		defer chat.Delete()
		return waitMessage(chatResponse, plugin.ToolCallCancel)
	})

	if err != nil {
		response.Error(ctx, -1, err)
		return true
	}

	return exec
}

package hf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bincooo/chatgpt-adapter/v2/internal/agent"
	com "github.com/bincooo/chatgpt-adapter/v2/internal/common"
	"github.com/bincooo/chatgpt-adapter/v2/internal/gin.handler/response"
	"github.com/bincooo/chatgpt-adapter/v2/internal/plugin"
	"github.com/bincooo/chatgpt-adapter/v2/logger"
	"github.com/bincooo/chatgpt-adapter/v2/pkg"
	"github.com/bincooo/emit.io"
	"github.com/gin-gonic/gin"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var (
	Adapter  = API{}
	ginSpace = "__prodia_space__"
)

type API struct {
	plugin.BaseAdapter
}

func (API) Match(ctx *gin.Context, model string) bool {
	if model != "dall-e-3" {
		return false
	}

	token := ctx.GetString("token")
	if token == "sk-prodia-sd" {
		ctx.Set(ginSpace, "sd")
		return true
	}

	if token == "sk-prodia-xl" {
		ctx.Set(ginSpace, "xl")
		return true
	}

	if token == "sk-google-xl" {
		ctx.Set(ginSpace, "google")
		return true
	}

	return false
}

func (API) Generation(ctx *gin.Context) {
	var (
		value        = ""
		modelSlice   []string
		samplesSlice []string
		space        = ctx.GetString(ginSpace)
		generation   = com.GetGinGeneration(ctx)
	)

	message, err := completeTagsGenerator(ctx, generation.Message)
	if err != nil {
		response.Error(ctx, -1, err)
		return
	}

	model := matchModel(generation.Style, space)
	samples := matchSamples(generation.Quality, space)

	switch space {
	case "xl":
		modelSlice = xlModels
		samplesSlice = xlSamples
		value, err = xl(ctx, model, samples, message)
	case "google":
		modelSlice = googleModels
		value, err = google(ctx, model, message)
	default:
		modelSlice = sdModels
		samplesSlice = sdSamples
		value, err = sd(ctx, model, samples, message)
	}

	if err != nil {
		logger.Error(err)
		response.Error(ctx, -1, err)
		return
	}

	if (generation.Size == "HD" || strings.HasPrefix(generation.Size, "1792x")) && com.HasMfy() {
		v, e := com.Magnify(ctx, value)
		if e != nil {
			logger.Error(e)
		} else {
			value = v
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"created": time.Now().Unix(),
		"styles":  modelSlice,
		"samples": samplesSlice,
		"data": []map[string]string{
			{"url": value},
		},
		"prompt":      message + ", {{{{by famous artist}}}, beautiful, masterpiece, 4k",
		"currStyle":   model,
		"currSamples": samples,
	})
}

func matchSamples(samples, spase string) string {
	switch spase {
	case "xl":
		if com.Contains(xlSamples, samples) {
			return samples
		}
		return "Euler a"

	default:
		if com.Contains(sdSamples, samples) {
			return samples
		}
		return "Euler a"
	}
}

func matchModel(style, spase string) string {
	switch spase {
	case "xl":
		if com.Contains(xlModels, style) {
			return style
		}
		return xlModels[rand.Intn(len(xlModels))]

	case "google":
		if com.Contains(googleModels, style) {
			return style
		}
		return googleModels[rand.Intn(len(googleModels))]

	default:
		if com.Contains(sdModels, style) {
			return style
		}
		return sdModels[rand.Intn(len(sdModels))]
	}
}

func completeTagsGenerator(ctx *gin.Context, content string) (string, error) {
	var (
		proxies = ctx.GetString("proxies")
		model   = pkg.Config.GetString("llm.model")
		cookie  = pkg.Config.GetString("llm.token")
		baseUrl = pkg.Config.GetString("llm.baseUrl")
	)

	prefix := ""
	if model == "bing" {
		prefix += "<pad />"
	}

	obj := map[string]interface{}{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": strings.Replace(prefix+agent.SDWords, "{{content}}", content, -1),
			},
		},
		"temperature": .8,
		"max_tokens":  4096,
	}

	response, err := fetch(ctx.Request.Context(), proxies, baseUrl, cookie, obj)
	if err != nil {
		return "", err
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var r pkg.ChatResponse
	if err = json.Unmarshal(data, &r); err != nil {
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		if r.Error != nil {
			return "", errors.New(r.Error.Message)
		} else {
			return "", errors.New(response.Status)
		}
	}

	message := strings.TrimSpace(r.Choices[0].Message.Content)
	left := strings.Index(message, `"""`)
	right := strings.LastIndex(message, `"""`)

	if left > -1 && left < right {
		message = strings.ReplaceAll(message[left+3:right], "\"", "")
		logger.Infof("system assistant generate message[%s]: %s", model, message)
		return strings.TrimSpace(message), nil
	}

	if strings.HasSuffix(message, `"""`) { // 哎。bing 偶尔会漏掉前面的"""
		message = strings.ReplaceAll(message[:len(message)-3], "\"", "")
		logger.Infof("system assistant generate message[%s]: %s", model, message)
		return strings.TrimSpace(message), nil
	}

	left = strings.Index(message, "```")
	right = strings.LastIndex(message, "```")

	if left > -1 && left < right {
		message = strings.ReplaceAll(message[left+3:right], "\"", "")
		logger.Infof("system assistant generate message[%s]: %s", model, message)
		return strings.TrimSpace(message), nil
	}

	logger.Info("response content: ", message)
	logger.Errorf("system assistant generate message[%s] error: system assistant generate message failed", model)
	return "", errors.New("system assistant generate message failed")
}

func fetch(ctx context.Context, proxies, baseUrl, cookie string, obj interface{}) (*http.Response, error) {
	if strings.Contains(baseUrl, "127.0.0.1") || strings.Contains(baseUrl, "localhost") {
		proxies = ""
	}

	return emit.ClientBuilder().
		Context(ctx).
		Proxies(proxies).
		POST(fmt.Sprintf("%s/v1/chat/completions", baseUrl)).
		Header("Authorization", cookie).
		JHeader().
		Body(obj).
		Do()
}

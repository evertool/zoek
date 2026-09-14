package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/config"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/tts"
)

// TTSHandler proxies Tencent Cloud TextToVoice (粤语报分音频) for the mini program.
type TTSHandler struct {
	client *tts.Client
}

func NewTTSHandler(cfg *config.Config) *TTSHandler {
	return &TTSHandler{client: tts.New(cfg.TTS)}
}

// GetTTS handles GET /api/v1/tts?text=... → { audio: <base64 mp3>, codec: "mp3" }.
// 未配置密钥时返回 TTS_NOT_CONFIGURED，前端回落微信同声传译插件（普通话）。
func (h *TTSHandler) GetTTS(c *gin.Context) {
	if !h.client.Enabled() {
		c.JSON(http.StatusServiceUnavailable, errs.New("TTS_NOT_CONFIGURED", "语音合成未配置", errs.ActionReadOnly))
		return
	}
	text := strings.TrimSpace(c.Query("text"))
	if text == "" {
		c.JSON(http.StatusBadRequest, errs.New("INVALID_INPUT", "text 不能为空", errs.ActionRetry))
		return
	}
	audio, err := h.client.Synthesize(text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.New("TTS_FAILED", "语音合成失败，请稍后重试", errs.ActionRetry))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"audio": audio,
		"codec": "mp3",
	})
}

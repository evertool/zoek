package handler

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/config"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/tts"
)

// TTSHandler proxies Tencent Cloud TextToVoice (粤语报分音频) for the mini program.
type TTSHandler struct {
	client *tts.Client
	// 音频缓存：报分文案是固定短语（「收到X分」），组合有限，缓存后云调用量趋近 0。
	// 进程内存放即可——重启后重新合成一次，代价可忽略，也免了磁盘清理。
	mu    sync.RWMutex
	cache map[string]ttsCacheEntry
}

func NewTTSHandler(cfg *config.Config) *TTSHandler {
	return &TTSHandler{client: tts.New(cfg.TTS), cache: map[string]ttsCacheEntry{}}
}

type ttsCacheEntry struct {
	audio string
	used  time.Time
}

func (h *TTSHandler) cacheGet(key string) (string, bool) {
	h.mu.RLock()
	e, ok := h.cache[key]
	h.mu.RUnlock()
	if !ok {
		return "", false
	}
	h.mu.Lock()
	if e2, ok2 := h.cache[key]; ok2 {
		e2.used = time.Now()
		h.cache[key] = e2
	}
	h.mu.Unlock()
	return e.audio, true
}

// cachePut 简易 LRU：超过容量丢最久没用的一条，保住常用短语。
func (h *TTSHandler) cachePut(key, audio string) {
	const max = 512
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.cache) >= max {
		oldestKey := ""
		var oldest time.Time
		first := true
		for k, e := range h.cache {
			if first || e.used.Before(oldest) {
				oldestKey, oldest, first = k, e.used, false
			}
		}
		delete(h.cache, oldestKey)
	}
	h.cache[key] = ttsCacheEntry{audio: audio, used: time.Now()}
}

// ttsVoiceFemaleYue 是「女声·粤语」的音色 ID，也是整个播报的默认音色。
// 腾讯云 TextToVoice（整型 VoiceType）的粤语音色**只有这一个** —— 101019 智彤·粤语女声（精品音色），
// 没有粤语男声（粤语男声在另一条产品线 TRTC 对话式 TTS 里，那是字符串 VoiceId 的另一套 API）。
const ttsVoiceFemaleYue = 101019

// ttsVoices 偏好设置里的三种配音 → 腾讯云 VoiceType（都是精品音色，计费同档 0.3 元/万字符）。
// 音色表：https://cloud.tencent.com/document/product/1073/92668
var ttsVoices = map[string]int{
	"female_yue":      ttsVoiceFemaleYue, // 女声·粤语（默认）
	"female_mandarin": 101001,            // 女声·普通话（智瑜·情感女声）
	"male_mandarin":   101004,            // 男声·普通话（智云·通用男声）
}

const ttsVoiceDefault = "female_yue"

// GetTTS handles GET /api/v1/tts?text=...&voice=female_yue → { audio: <base64 mp3>, codec: "mp3" }.
// voice 可选：female_yue（默认）/ female_mandarin / male_mandarin，选错或缺省回落默认音色
// ——播报是非关键路径，别因为一个参数就报错。
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
	voiceName := c.Query("voice")
	if _, ok := ttsVoices[voiceName]; !ok {
		voiceName = ttsVoiceDefault
	}

	key := voiceName + "|" + text
	if audio, ok := h.cacheGet(key); ok {
		c.JSON(http.StatusOK, gin.H{"audio": audio, "codec": "mp3", "voice": voiceName, "cached": true})
		return
	}

	audio, err := h.client.Synthesize(text, ttsVoices[voiceName])
	if err != nil {
		c.JSON(http.StatusInternalServerError, errs.New("TTS_FAILED", "语音合成失败，请稍后重试", errs.ActionRetry))
		return
	}
	h.cachePut(key, audio)
	c.JSON(http.StatusOK, gin.H{
		"audio": audio,
		"codec": "mp3",
		"voice": voiceName,
	})
}

package tts

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/lk/zoek/backend/internal/config"
)

// 真实调用冒烟：TTS_SMOKE_SID / TTS_SMOKE_SKEY 存在时才跑，消耗少量免费字符。
// 用途：验证密钥有效、TC3 签名正确、粤语音色可用。
func TestSynthesizeSmoke(t *testing.T) {
	sid, skey := os.Getenv("TTS_SMOKE_SID"), os.Getenv("TTS_SMOKE_SKEY")
	if sid == "" || skey == "" {
		t.Skip("TTS_SMOKE_SID / TTS_SMOKE_SKEY 未设置，跳过真实调用")
	}
	cfg := config.TTSConfig{SecretID: sid, SecretKey: skey, Region: "ap-guangzhou", VoiceType: 101019, Codec: "mp3"}
	c := New(cfg)
	audio, err := c.Synthesize("阿强转给你八分，快啲落座啦")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(audio)
	if err != nil || len(raw) < 1000 {
		t.Fatalf("audio invalid: %v, len=%d", err, len(raw))
	}
	t.Logf("OK: %d bytes mp3", len(raw))
}

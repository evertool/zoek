// Package tts — 腾讯云语音合成客户端（TextToVoice，TC3-HMAC-SHA256 签名，零 SDK 依赖）。
// 用于粤语报分：VoiceType 默认 101019（智彤·粤语女声，精品音色）。
package tts

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lk/zoek/backend/internal/config"
)

const (
	ttsHost    = "tts.tencentcloudapi.com"
	ttsService = "tts"
	ttsVersion = "2019-08-23"
	ttsAction  = "TextToVoice"
)

// Client is a Tencent Cloud TTS client bound to a config.
type Client struct {
	cfg config.TTSConfig
	hc  *http.Client
}

// New creates a TTS client. Enabled() is false until SecretID/SecretKey are set.
func New(cfg config.TTSConfig) *Client {
	if cfg.Region == "" {
		cfg.Region = "ap-guangzhou"
	}
	if cfg.VoiceType == 0 {
		cfg.VoiceType = 101019
	}
	if cfg.Codec == "" {
		cfg.Codec = "mp3"
	}
	return &Client{cfg: cfg, hc: &http.Client{Timeout: 10 * time.Second}}
}

// Enabled reports whether cloud TTS is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.cfg.SecretID != "" && c.cfg.SecretKey != ""
}

// Synthesize converts text to speech, returning base64-encoded audio.
// 文本上限：中文 150 汉字（接口限制）。
func (c *Client) Synthesize(text string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("tts not configured")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty text")
	}
	if runes := len([]rune(text)); runes > 150 {
		text = string([]rune(text)[:150])
	}

	sessionID, err := randomSessionID()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]interface{}{
		"Text":      text,
		"SessionId": sessionID,
		"VoiceType": c.cfg.VoiceType,
		"Codec":     c.cfg.Codec,
		"Volume":    0,
		"Speed":     0,
	})
	if err != nil {
		return "", err
	}

	audio, err := c.post(payload)
	if err != nil {
		return "", err
	}
	return audio, nil
}

// post signs and sends the request, returning the base64 Audio field.
func (c *Client) post(payload []byte) (string, error) {
	now := time.Now().UTC()
	ts := fmt.Sprintf("%d", now.Unix())
	date := now.Format("2006-01-02")

	// ---- TC3-HMAC-SHA256 签名 ----
	hashedPayload := sha256Hex(payload)
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + ttsHost +
		"\nx-tc-action:" + strings.ToLower(ttsAction) + "\n"
	canonicalRequest := strings.Join([]string{
		"POST",
		"/",
		"",
		canonicalHeaders,
		"content-type;host;x-tc-action",
		hashedPayload,
	}, "\n")

	scope := date + "/" + ttsService + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		ts,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	kDate := hmacSHA256([]byte("TC3"+c.cfg.SecretKey), date)
	kService := hmacSHA256(kDate, ttsService)
	kSigning := hmacSHA256(kService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	auth := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host;x-tc-action, Signature=%s",
		c.cfg.SecretID, scope, signature)

	req, err := http.NewRequest(http.MethodPost, "https://"+ttsHost, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-TC-Action", ttsAction)
	req.Header.Set("X-TC-Version", ttsVersion)
	req.Header.Set("X-TC-Region", c.cfg.Region)
	req.Header.Set("X-TC-Timestamp", ts)
	req.Header.Set("Authorization", auth)

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("tts request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var out struct {
		Response struct {
			Audio     string `json:"Audio"`
			RequestID string `json:"RequestId"`
			Error     *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("tts response: %w", err)
	}
	if out.Response.Error != nil {
		return "", fmt.Errorf("tts api %s: %s", out.Response.Error.Code, out.Response.Error.Message)
	}
	if out.Response.Audio == "" {
		return "", fmt.Errorf("tts api returned empty audio")
	}
	return out.Response.Audio, nil
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func randomSessionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

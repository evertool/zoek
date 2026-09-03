package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Code2SessionFunc is the function signature for code2session.
type Code2SessionFunc func(code string) (openID, unionID string, err error)

// QRCodeFunc is the function signature for getting a mini program QR code.
type QRCodeFunc func(page string, scene string) ([]byte, error)

// Client is a WeChat Mini Program API client.
type Client struct {
	AppID     string
	AppSecret string
	HTTP      *http.Client
	// mockFunc, if set, replaces the real code2session call.
	mockFunc Code2SessionFunc
	// mockQRFunc, if set, replaces the real getwxacode call.
	mockQRFunc QRCodeFunc
}

// NewClient creates a WeChat client for production use.
func NewClient(appID, appSecret string) *Client {
	return &Client{
		AppID:     appID,
		AppSecret: appSecret,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewMockClient creates a WeChat client that uses the given mock function
// instead of calling the real WeChat API. For testing only.
func NewMockClient(mock Code2SessionFunc) *Client {
	return &Client{
		AppID:    "mock",
		mockFunc: mock,
	}
}

// Code2SessionResponse is the response from WeChat code2session API.
type Code2SessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// Code2Session exchanges a wx.login code for openid and session_key.
func (c *Client) Code2Session(code string) (openID, unionID string, err error) {
	if c.mockFunc != nil {
		return c.mockFunc(code)
	}

	if c.AppID == "" || c.AppSecret == "" {
		return "", "", fmt.Errorf("wechat appid/app_secret not configured")
	}

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		c.AppID, c.AppSecret, code,
	)

	resp, err := c.HTTP.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("call code2session: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read code2session response: %w", err)
	}

	var result Code2SessionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parse code2session response: %w", err)
	}

	if result.ErrCode != 0 {
		return "", "", fmt.Errorf("code2session error: code=%d msg=%s", result.ErrCode, result.ErrMsg)
	}

	return result.OpenID, result.UnionID, nil
}

// getAccessToken fetches the access_token from WeChat API.
// https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/access-token/auth.getAccessToken.html
func (c *Client) getAccessToken() (string, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		c.AppID, c.AppSecret,
	)

	resp, err := c.HTTP.Get(url)
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read access token response: %w", err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse access token response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("get access token error: code=%d msg=%s", result.ErrCode, result.ErrMsg)
	}

	return result.AccessToken, nil
}

// GetMiniProgramCode generates a mini program QR code (小程序码) via getwxacodeunlimit API.
// page: the page to land on (e.g. "pages/join/join")
// scene: the scene parameter (e.g. "t=<invite_token>"), max 32 chars
// Returns PNG image bytes.
// https://developers.weixin.qq.com/miniprogram/dev/api-backend/open-api/qr-code/wxacode.getUnlimited.html
func (c *Client) GetMiniProgramCode(page, scene string) ([]byte, error) {
	// Use mock if set (testing)
	if c.mockQRFunc != nil {
		return c.mockQRFunc(page, scene)
	}

	if c.AppID == "" || c.AppSecret == "" {
		return nil, fmt.Errorf("wechat appid/app_secret not configured")
	}

	accessToken, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("get access token for qrcode: %w", err)
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/wxa/getwxacodeunlimit?access_token=%s", accessToken)

	payload, _ := json.Marshal(map[string]interface{}{
		"scene":     scene,
		"page":      page,
		"width":     430,
		"check_path": false,
	})

	resp, err := c.HTTP.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("call getwxacodeunlimit: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read qrcode response: %w", err)
	}

	// Check if the response is an error JSON (WeChat returns JSON on error, PNG on success)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "application/json" || (len(body) > 0 && body[0] == '{') {
		var result struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if err := json.Unmarshal(body, &result); err == nil && result.ErrCode != 0 {
			return nil, fmt.Errorf("getwxacodeunlimit error: code=%d msg=%s", result.ErrCode, result.ErrMsg)
		}
	}

	return body, nil
}

// NewMockQRClient creates a WeChat client with mock functions for both
// code2session and getMiniProgramCode. For testing only.
func NewMockQRClient(mock Code2SessionFunc, mockQR QRCodeFunc) *Client {
	return &Client{
		AppID:     "mock",
		mockFunc:  mock,
		mockQRFunc: mockQR,
	}
}

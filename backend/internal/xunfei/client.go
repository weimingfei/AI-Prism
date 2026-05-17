package xunfei

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ai-prism/backend/internal/asr"
	"github.com/ai-prism/backend/internal/config"
	"github.com/gorilla/websocket"
)

type Client struct {
	cfg config.XunfeiConfig
}

type TTSRequest struct {
	Text     string
	Voice    string
	Speed    int
	Volume   int
	Pitch    int
	Encoding string
}

func NewClient(cfg config.XunfeiConfig) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) ConfiguredForASR() bool {
	return c.cfg.ASREnabled && c.credentialsReady() && strings.TrimSpace(c.cfg.ASRWSURL) != ""
}

func (c *Client) ConfiguredForTTS() bool {
	return c.cfg.TTSEnabled && c.credentialsReady() && strings.TrimSpace(c.cfg.TTSWSURL) != ""
}

func (c *Client) DialASR() (*websocket.Conn, error) {
	if !c.ConfiguredForASR() {
		return nil, errors.New("讯飞语音听写未配置")
	}
	return c.dial(c.cfg.ASRWSURL)
}

func (c *Client) Synthesize(req TTSRequest) ([]byte, error) {
	if !c.ConfiguredForTTS() {
		return nil, errors.New("讯飞语音合成未配置")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, errors.New("TTS 文本为空")
	}

	conn, err := c.dial(c.cfg.TTSWSURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		voice = "xiaoyan"
	}
	encoding := strings.TrimSpace(req.Encoding)
	if encoding == "" {
		encoding = "lame"
	}
	speed := clampVoiceNumber(req.Speed, 50)
	volume := clampVoiceNumber(req.Volume, 50)
	pitch := clampVoiceNumber(req.Pitch, 50)

	payload := map[string]any{
		"common": map[string]any{
			"app_id": c.cfg.AppID,
		},
		"business": map[string]any{
			"aue":    encoding,
			"sfl":    1,
			"auf":    "audio/L16;rate=16000",
			"vcn":    voice,
			"tte":    "UTF8",
			"speed":  speed,
			"volume": volume,
			"pitch":  pitch,
		},
		"data": map[string]any{
			"status": 2,
			"text":   base64.StdEncoding.EncodeToString([]byte(text)),
		},
	}
	if err := conn.WriteJSON(payload); err != nil {
		return nil, err
	}

	var audio bytes.Buffer
	for {
		var resp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Audio  string `json:"audio"`
				Status int    `json:"status"`
			} `json:"data"`
		}
		if err := conn.ReadJSON(&resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("讯飞 TTS 失败：%s", resp.Message)
		}
		if resp.Data.Audio != "" {
			chunk, err := base64.StdEncoding.DecodeString(resp.Data.Audio)
			if err != nil {
				return nil, err
			}
			audio.Write(chunk)
		}
		if resp.Data.Status == 2 {
			break
		}
	}
	return audio.Bytes(), nil
}

func (c *Client) NewASRStartFrame(audio []byte) map[string]any {
	return map[string]any{
		"common": map[string]any{
			"app_id": c.cfg.AppID,
		},
		"business": map[string]any{
			"language": "zh_cn",
			"domain":   "iat",
			"accent":   "mandarin",
			"vad_eos":  3000,
			"dwa":      "wpgs",
		},
		"data": map[string]any{
			"status":   0,
			"format":   "audio/L16;rate=16000",
			"encoding": "raw",
			"audio":    base64.StdEncoding.EncodeToString(audio),
		},
	}
}

func NewASRContinueFrame(audio []byte) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"status":   1,
			"format":   "audio/L16;rate=16000",
			"encoding": "raw",
			"audio":    base64.StdEncoding.EncodeToString(audio),
		},
	}
}

func NewASREndFrame() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"status":   2,
			"format":   "audio/L16;rate=16000",
			"encoding": "raw",
			"audio":    "",
		},
	}
}

func ExtractASRText(payload []byte) (string, int, error) {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status int `json:"status"`
			Result struct {
				WS []struct {
					CW []struct {
						W string `json:"w"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return "", 0, err
	}
	if resp.Code != 0 {
		return "", resp.Data.Status, fmt.Errorf("讯飞 ASR 失败：%s", resp.Message)
	}
	var builder strings.Builder
	for _, ws := range resp.Data.Result.WS {
		if len(ws.CW) > 0 {
			builder.WriteString(ws.CW[0].W)
		}
	}
	return builder.String(), resp.Data.Status, nil
}

type ASRUpdate struct {
	Text        string
	Status      int
	SegmentID   int
	PGS         string
	RG          []int
	BG          *int
	ED          *int
	FinalPacket bool
}

func ExtractASRUpdate(payload []byte) (ASRUpdate, error) {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status int `json:"status"`
			Result struct {
				SN  int    `json:"sn"`
				PGS string `json:"pgs"`
				RG  []int  `json:"rg"`
				BG  *int   `json:"bg"`
				ED  *int   `json:"ed"`
				LS  bool   `json:"ls"`
				WS  []struct {
					CW []struct {
						W string `json:"w"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return ASRUpdate{}, err
	}
	if resp.Code != 0 {
		return ASRUpdate{Status: resp.Data.Status}, fmt.Errorf("讯飞 ASR 失败：%s", resp.Message)
	}
	var builder strings.Builder
	for _, ws := range resp.Data.Result.WS {
		if len(ws.CW) > 0 {
			builder.WriteString(ws.CW[0].W)
		}
	}
	return ASRUpdate{
		Text:        builder.String(),
		Status:      resp.Data.Status,
		SegmentID:   resp.Data.Result.SN,
		PGS:         resp.Data.Result.PGS,
		RG:          normalizeRG(resp.Data.Result.RG),
		BG:          resp.Data.Result.BG,
		ED:          resp.Data.Result.ED,
		FinalPacket: resp.Data.Status == 2 || resp.Data.Result.LS,
	}, nil
}

func ASRPacketFromUpdate(update ASRUpdate) asr.Packet {
	return asr.Packet{
		SegmentID:   update.SegmentID,
		Text:        update.Text,
		PGS:         update.PGS,
		RG:          update.RG,
		BG:          update.BG,
		ED:          update.ED,
		FinalPacket: update.FinalPacket,
	}
}

func normalizeRG(values []int) []int {
	if len(values) < 2 {
		return nil
	}
	start, end := values[0], values[1]
	if start > end {
		start, end = end, start
	}
	return []int{start, end}
}

func (c *Client) dial(rawURL string) (*websocket.Conn, error) {
	signed, err := c.signedURL(rawURL)
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.Dial(signed, nil)
	return conn, err
}

func (c *Client) signedURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	host := parsed.Host
	date := time.Now().UTC().Format(http.TimeFormat)
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET %s HTTP/1.1", host, date, path)
	mac := hmac.New(sha256.New, []byte(c.cfg.APISecret))
	_, _ = mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authorizationOrigin := fmt.Sprintf(
		`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		c.cfg.APIKey,
		signature,
	)

	query := parsed.Query()
	query.Set("authorization", base64.StdEncoding.EncodeToString([]byte(authorizationOrigin)))
	query.Set("date", date)
	query.Set("host", host)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) credentialsReady() bool {
	return strings.TrimSpace(c.cfg.AppID) != "" &&
		strings.TrimSpace(c.cfg.APIKey) != "" &&
		strings.TrimSpace(c.cfg.APISecret) != ""
}

func clampVoiceNumber(value int, fallback int) int {
	if value <= 0 {
		value = fallback
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Milky  MilkyConfig  `json:"milky"`
	OneBot OneBotConfig `json:"onebot"`
	Bridge BridgeConfig `json:"bridge"`
}

type MilkyConfig struct {
	WSGateway   string `json:"ws_gateway"`
	RestGateway string `json:"rest_gateway"`
	Token       string `json:"token"`
}

type OneBotConfig struct {
	Host              string              `json:"host"`
	Port              int                 `json:"port"`
	AccessToken       string              `json:"access_token"`
	EnableHTTPAPI     bool                `json:"enable_http_api"`
	EnableWSAPI       bool                `json:"enable_ws_api"`
	EnableWSEvent     bool                `json:"enable_ws_event"`
	EnableWSUniversal bool                `json:"enable_ws_universal"`
	Reverse           OneBotReverseConfig `json:"reverse"`
}

type OneBotReverseConfig struct {
	Enable              bool   `json:"enable"`
	URL                 string `json:"url"`
	APIURL              string `json:"api_url"`
	EventURL            string `json:"event_url"`
	UseUniversalClient  bool   `json:"use_universal_client"`
	ReconnectIntervalMS int    `json:"reconnect_interval_ms"`
}

type BridgeConfig struct {
	SelfID              int64  `json:"self_id"`
	MessageFormat       string `json:"message_format"`
	HeartbeatIntervalMS int    `json:"heartbeat_interval_ms"`
	LogLevel            string `json:"log_level"`
	CacheSize           int    `json:"cache_size"`
}

func Default() Config {
	return Config{
		OneBot: OneBotConfig{
			Host:              "0.0.0.0",
			Port:              6700,
			EnableHTTPAPI:     false,
			EnableWSAPI:       true,
			EnableWSEvent:     true,
			EnableWSUniversal: true,
			Reverse: OneBotReverseConfig{
				ReconnectIntervalMS: 3000,
			},
		},
		Bridge: BridgeConfig{
			MessageFormat:       "array",
			HeartbeatIntervalMS: 15000,
			LogLevel:            "info",
			CacheSize:           2048,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Milky.WSGateway) == "" {
		return errors.New("milky.ws_gateway is required")
	}
	if strings.TrimSpace(c.Milky.RestGateway) == "" {
		return errors.New("milky.rest_gateway is required")
	}
	if c.OneBot.Port <= 0 || c.OneBot.Port > 65535 {
		return errors.New("onebot.port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.OneBot.Host) == "" {
		return errors.New("onebot.host is required")
	}
	if c.Bridge.MessageFormat != "array" && c.Bridge.MessageFormat != "string" {
		return errors.New("bridge.message_format must be array or string")
	}
	if c.Bridge.HeartbeatIntervalMS <= 0 {
		return errors.New("bridge.heartbeat_interval_ms must be positive")
	}
	if c.Bridge.CacheSize <= 0 {
		return errors.New("bridge.cache_size must be positive")
	}
	if c.OneBot.Reverse.Enable && c.OneBot.Reverse.ReconnectIntervalMS <= 0 {
		return errors.New("onebot.reverse.reconnect_interval_ms must be positive")
	}
	if c.OneBot.Reverse.Enable {
		if c.OneBot.Reverse.UseUniversalClient {
			if strings.TrimSpace(c.OneBot.Reverse.URL) == "" &&
				strings.TrimSpace(c.OneBot.Reverse.APIURL) == "" &&
				strings.TrimSpace(c.OneBot.Reverse.EventURL) == "" {
				return errors.New("onebot.reverse requires url when universal reverse ws is enabled")
			}
		} else {
			if strings.TrimSpace(c.OneBot.Reverse.URL) == "" &&
				strings.TrimSpace(c.OneBot.Reverse.APIURL) == "" &&
				strings.TrimSpace(c.OneBot.Reverse.EventURL) == "" {
				return errors.New("onebot.reverse requires url/api_url/event_url when reverse ws is enabled")
			}
		}
	}
	return nil
}

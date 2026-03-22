package onebot

import (
	"encoding/json"
	"strings"
)

type APIRequest struct {
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
	Echo   any             `json:"echo,omitempty"`
}

type APIResponse struct {
	Status  string `json:"status"`
	RetCode int    `json:"retcode"`
	Data    any    `json:"data"`
	Msg     string `json:"msg,omitempty"`
	Wording string `json:"wording,omitempty"`
	Echo    any    `json:"echo,omitempty"`
}

func NormalizeAction(action string) string {
	action = strings.TrimSpace(action)
	action = strings.TrimSuffix(action, "_async")
	action = strings.TrimSuffix(action, "_rate_limited")
	return action
}

func Success(data any, echo any) APIResponse {
	return APIResponse{
		Status:  "ok",
		RetCode: 0,
		Data:    data,
		Echo:    echo,
	}
}

func Failure(retCode int, message string, echo any) APIResponse {
	return APIResponse{
		Status:  "failed",
		RetCode: retCode,
		Data:    nil,
		Msg:     message,
		Wording: message,
		Echo:    echo,
	}
}

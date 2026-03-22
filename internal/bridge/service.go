package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"milky-onebot11-bridge/internal/config"
	"milky-onebot11-bridge/internal/milky"
	"milky-onebot11-bridge/internal/onebot"
	"milky-onebot11-bridge/internal/state"
	"milky-onebot11-bridge/internal/types"
)

type Service struct {
	cfg      config.Config
	logger   *slog.Logger
	upstream *milky.Client
	server   *onebot.Server

	runtime  *state.Runtime
	messages *state.MessageMap
	requests *state.RequestMap
}

func NewService(cfg config.Config, logger *slog.Logger) (*Service, error) {
	svc := &Service{
		cfg:      cfg,
		logger:   logger,
		upstream: milky.NewClient(cfg.Milky, logger),
		runtime:  state.NewRuntime(),
		messages: state.NewMessageMap(),
		requests: state.NewRequestMap(),
	}
	svc.server = onebot.NewServer(cfg.OneBot, logger, svc)
	return svc, nil
}

func (s *Service) Run(ctx context.Context) error {
	if err := s.upstream.Connect(ctx); err != nil {
		return err
	}
	s.runtime.SetUpstreamConnected(true)
	login := s.upstream.Login()
	if s.cfg.Bridge.SelfID != 0 {
		login.SelfID = s.cfg.Bridge.SelfID
	}
	s.runtime.SetLogin(login)

	errCh := s.server.Start(ctx)

	ticker := time.NewTicker(time.Duration(s.cfg.Bridge.HeartbeatIntervalMS) * time.Millisecond)
	defer ticker.Stop()
	defer s.runtime.SetUpstreamConnected(false)
	defer s.upstream.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			return err
		case event := <-s.upstream.Events():
			payload, ok := s.toOneBotEvent(event)
			if ok {
				s.server.Broadcast(payload)
			}
		case <-ticker.C:
			status := s.runtime.Status()
			s.server.Broadcast(onebot.HeartbeatEvent(
				s.selfID(),
				map[string]any{
					"online": status.Online,
					"good":   status.Good,
				},
				s.cfg.Bridge.HeartbeatIntervalMS,
			))
		}
	}
}

func (s *Service) OnWSConnect(_ context.Context, wsType string) []any {
	if wsType == "api" || wsType == "reverse-api" {
		return nil
	}
	return []any{onebot.LifecycleEvent(s.selfID(), "connect")}
}

func (s *Service) CurrentSelfID() int64 {
	return s.selfID()
}

func (s *Service) HandleAPI(_ context.Context, req onebot.APIRequest) onebot.APIResponse {
	action := onebot.NormalizeAction(req.Action)
	switch action {
	case "send_private_msg":
		var params struct {
			UserID     int64           `json:"user_id"`
			Message    json.RawMessage `json:"message"`
			AutoEscape bool            `json:"auto_escape"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		segments, err := parseOneBotMessage(params.Message, params.AutoEscape)
		if err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		messageID, err := s.upstream.SendPrivateMessage(params.UserID, segments)
		if err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		s.messages.Put(types.MessageRef{OneBotID: messageID, MilkySeq: messageID, MessageType: "private", UserID: params.UserID})
		return onebot.Success(map[string]any{"message_id": messageID}, req.Echo)
	case "send_group_msg":
		var params struct {
			GroupID    int64           `json:"group_id"`
			Message    json.RawMessage `json:"message"`
			AutoEscape bool            `json:"auto_escape"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		segments, err := parseOneBotMessage(params.Message, params.AutoEscape)
		if err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		messageID, err := s.upstream.SendGroupMessage(params.GroupID, segments)
		if err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		s.messages.Put(types.MessageRef{OneBotID: messageID, MilkySeq: messageID, MessageType: "group", GroupID: params.GroupID})
		return onebot.Success(map[string]any{"message_id": messageID}, req.Echo)
	case "send_msg":
		var params struct {
			MessageType string          `json:"message_type"`
			UserID      int64           `json:"user_id"`
			GroupID     int64           `json:"group_id"`
			Message     json.RawMessage `json:"message"`
			AutoEscape  bool            `json:"auto_escape"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		segments, err := parseOneBotMessage(params.Message, params.AutoEscape)
		if err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		switch params.MessageType {
		case "", "private":
			if params.UserID != 0 {
				messageID, err := s.upstream.SendPrivateMessage(params.UserID, segments)
				if err != nil {
					return onebot.Failure(1500, err.Error(), req.Echo)
				}
				s.messages.Put(types.MessageRef{OneBotID: messageID, MilkySeq: messageID, MessageType: "private", UserID: params.UserID})
				return onebot.Success(map[string]any{"message_id": messageID}, req.Echo)
			}
		case "group":
			if params.GroupID != 0 {
				messageID, err := s.upstream.SendGroupMessage(params.GroupID, segments)
				if err != nil {
					return onebot.Failure(1500, err.Error(), req.Echo)
				}
				s.messages.Put(types.MessageRef{OneBotID: messageID, MilkySeq: messageID, MessageType: "group", GroupID: params.GroupID})
				return onebot.Success(map[string]any{"message_id": messageID}, req.Echo)
			}
		}
		return onebot.Failure(1400, "send_msg requires a valid message target", req.Echo)
	case "get_login_info":
		login := s.runtime.Login()
		return onebot.Success(map[string]any{
			"user_id":  login.SelfID,
			"nickname": login.Nickname,
		}, req.Echo)
	case "get_status":
		status := s.runtime.Status()
		return onebot.Success(map[string]any{
			"online": status.Online,
			"good":   status.Good,
		}, req.Echo)
	case "get_version_info":
		return onebot.Success(map[string]any{
			"app_name":         "milky-ob11-bridge",
			"app_version":      "0.1.0",
			"protocol_version": "v11",
		}, req.Echo)
	case "can_send_image":
		return onebot.Success(map[string]any{"yes": true}, req.Echo)
	case "can_send_record":
		return onebot.Success(map[string]any{"yes": true}, req.Echo)
	case "get_group_info":
		var params struct {
			GroupID int64 `json:"group_id"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		info, err := s.upstream.GetGroupInfo(params.GroupID)
		if err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		return onebot.Success(info, req.Echo)
	case "get_group_list":
		info, err := s.upstream.GetGroupList()
		if err != nil {
			code, msg := translateError(err)
			return onebot.Failure(code, msg, req.Echo)
		}
		return onebot.Success(info, req.Echo)
	case "get_group_member_info":
		var params struct {
			GroupID int64 `json:"group_id"`
			UserID  int64 `json:"user_id"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		info, err := s.upstream.GetGroupMemberInfo(params.GroupID, params.UserID)
		if err != nil {
			code, msg := translateError(err)
			return onebot.Failure(code, msg, req.Echo)
		}
		return onebot.Success(info, req.Echo)
	case "get_group_member_list":
		var params struct {
			GroupID int64 `json:"group_id"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		info, err := s.upstream.GetGroupMemberList(params.GroupID)
		if err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		return onebot.Success(info, req.Echo)
	case "delete_msg":
		var params struct {
			MessageID int64 `json:"message_id"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		ref, ok := s.messages.Get(params.MessageID)
		if !ok {
			return onebot.Failure(1502, "message_id not found", req.Echo)
		}
		if err := s.upstream.DeleteMessage(ref); err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		return onebot.Success(nil, req.Echo)
	case "get_msg":
		var params struct {
			MessageID int64 `json:"message_id"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		ref, ok := s.messages.Get(params.MessageID)
		if !ok {
			return onebot.Failure(1502, "message_id not found", req.Echo)
		}
		event, err := s.upstream.GetMessage(ref)
		if err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		message, raw := buildOneBotMessage(s.cfg.Bridge.MessageFormat, event.Segments)
		data := map[string]any{
			"time":         event.Time,
			"message_type": ref.MessageType,
			"message_id":   ref.OneBotID,
			"real_id":      ref.MilkySeq,
			"message":      message,
			"raw_message":  raw,
			"sender":       buildSenderObject(event.Sender, ref.MessageType),
		}
		if ref.MessageType == "group" {
			data["group_id"] = ref.GroupID
		} else {
			data["user_id"] = ref.UserID
		}
		return onebot.Success(data, req.Echo)
	case "set_friend_add_request":
		var params struct {
			Flag    string `json:"flag"`
			Approve bool   `json:"approve"`
			Remark  string `json:"remark"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		ref, ok := s.requests.Get(params.Flag)
		if !ok {
			return onebot.Failure(1502, "request flag not found", req.Echo)
		}
		if ref.Kind != "friend" {
			return onebot.Failure(1400, "flag does not point to a friend request", req.Echo)
		}
		if err := s.upstream.HandleFriendRequest(ref, params.Approve, params.Remark); err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		return onebot.Success(nil, req.Echo)
	case "set_group_add_request":
		var params struct {
			Flag    string `json:"flag"`
			Approve bool   `json:"approve"`
			Reason  string `json:"reason"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return onebot.Failure(1400, err.Error(), req.Echo)
		}
		ref, ok := s.requests.Get(params.Flag)
		if !ok {
			return onebot.Failure(1502, "request flag not found", req.Echo)
		}
		if ref.Kind != "group" {
			return onebot.Failure(1400, "flag does not point to a group request", req.Echo)
		}
		if err := s.upstream.HandleGroupRequest(ref, params.Approve); err != nil {
			return onebot.Failure(1500, err.Error(), req.Echo)
		}
		return onebot.Success(nil, req.Echo)
	default:
		return onebot.Failure(1503, unsupportedAction(action), req.Echo)
	}
}

func (s *Service) selfID() int64 {
	login := s.runtime.Login()
	if login.SelfID != 0 {
		return login.SelfID
	}
	return s.cfg.Bridge.SelfID
}

func translateError(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	if errors.Is(err, context.Canceled) {
		return 1500, err.Error()
	}
	return 1500, err.Error()
}

func buildSenderObject(sender types.Sender, messageType string) map[string]any {
	if messageType == "group" {
		return map[string]any{
			"user_id":  sender.UserID,
			"nickname": sender.Nickname,
			"card":     sender.Card,
			"sex":      sender.Sex,
			"age":      sender.Age,
			"area":     sender.Area,
			"level":    sender.Level,
			"role":     sender.Role,
			"title":    sender.Title,
		}
	}
	return map[string]any{
		"user_id":  sender.UserID,
		"nickname": sender.Nickname,
		"sex":      sender.Sex,
		"age":      sender.Age,
	}
}

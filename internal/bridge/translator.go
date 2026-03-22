package bridge

import (
	"fmt"
	"time"

	"milky-onebot11-bridge/internal/types"
)

func (s *Service) toOneBotEvent(event types.InboundEvent) (map[string]any, bool) {
	selfID := s.selfID()
	switch event.Kind {
	case types.EventMessagePrivate:
		s.messages.Put(types.MessageRef{
			OneBotID:    event.MessageID,
			MilkySeq:    event.MessageID,
			MessageType: "private",
			UserID:      event.UserID,
		})
		message, raw := buildOneBotMessage(s.cfg.Bridge.MessageFormat, event.Segments)
		return map[string]any{
			"time":         chooseEventTime(event.Time),
			"self_id":      selfID,
			"post_type":    "message",
			"message_type": "private",
			"sub_type":     "friend",
			"message_id":   event.MessageID,
			"user_id":      event.UserID,
			"message":      message,
			"raw_message":  raw,
			"font":         0,
			"sender": map[string]any{
				"user_id":  event.Sender.UserID,
				"nickname": event.Sender.Nickname,
				"sex":      emptyToUnknown(event.Sender.Sex),
				"age":      event.Sender.Age,
			},
		}, true
	case types.EventMessageGroup:
		s.messages.Put(types.MessageRef{
			OneBotID:    event.MessageID,
			MilkySeq:    event.MessageID,
			MessageType: "group",
			GroupID:     event.GroupID,
			UserID:      event.UserID,
		})
		message, raw := buildOneBotMessage(s.cfg.Bridge.MessageFormat, event.Segments)
		return map[string]any{
			"time":         chooseEventTime(event.Time),
			"self_id":      selfID,
			"post_type":    "message",
			"message_type": "group",
			"sub_type":     "normal",
			"message_id":   event.MessageID,
			"group_id":     event.GroupID,
			"user_id":      event.UserID,
			"message":      message,
			"raw_message":  raw,
			"font":         0,
			"sender": map[string]any{
				"user_id":  event.Sender.UserID,
				"nickname": event.Sender.Nickname,
				"card":     event.Sender.Card,
				"sex":      emptyToUnknown(event.Sender.Sex),
				"age":      event.Sender.Age,
				"area":     event.Sender.Area,
				"level":    event.Sender.Level,
				"role":     emptyToMember(event.Sender.Role),
				"title":    event.Sender.Title,
			},
		}, true
	case types.EventPokePrivate:
		return map[string]any{
			"time":        chooseEventTime(event.Time),
			"self_id":     selfID,
			"post_type":   "notice",
			"notice_type": "notify",
			"sub_type":    "poke",
			"user_id":     event.UserID,
			"target_id":   event.TargetID,
		}, true
	case types.EventPokeGroup:
		return map[string]any{
			"time":        chooseEventTime(event.Time),
			"self_id":     selfID,
			"post_type":   "notice",
			"notice_type": "notify",
			"sub_type":    "poke",
			"group_id":    event.GroupID,
			"user_id":     event.UserID,
			"target_id":   event.TargetID,
		}, true
	case types.EventFriendRequest:
		flag := s.requests.Put(*event.Request)
		return map[string]any{
			"time":         chooseEventTime(event.Time),
			"self_id":      selfID,
			"post_type":    "request",
			"request_type": "friend",
			"user_id":      event.UserID,
			"comment":      event.Comment,
			"flag":         flag,
		}, true
	case types.EventGroupInvite:
		flag := s.requests.Put(*event.Request)
		return map[string]any{
			"time":         chooseEventTime(event.Time),
			"self_id":      selfID,
			"post_type":    "request",
			"request_type": "group",
			"sub_type":     "invite",
			"group_id":     event.GroupID,
			"user_id":      event.UserID,
			"comment":      event.Comment,
			"flag":         flag,
		}, true
	case types.EventRecallPrivate:
		return map[string]any{
			"time":        chooseEventTime(event.Time),
			"self_id":     selfID,
			"post_type":   "notice",
			"notice_type": "friend_recall",
			"user_id":     event.UserID,
			"message_id":  event.MessageID,
		}, true
	case types.EventRecallGroup:
		return map[string]any{
			"time":        chooseEventTime(event.Time),
			"self_id":     selfID,
			"post_type":   "notice",
			"notice_type": "group_recall",
			"group_id":    event.GroupID,
			"user_id":     event.UserID,
			"operator_id": event.UserID,
			"message_id":  event.MessageID,
		}, true
	default:
		return nil, false
	}
}

func chooseEventTime(ts int64) int64 {
	if ts > 0 {
		return ts
	}
	return time.Now().Unix()
}

func emptyToUnknown(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

func emptyToMember(v string) string {
	if v == "" {
		return "member"
	}
	return v
}

func unsupportedAction(action string) string {
	return fmt.Sprintf("action %s is not supported in v1", action)
}

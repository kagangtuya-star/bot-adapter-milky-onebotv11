package onebot

import "time"

func LifecycleEvent(selfID int64, subType string) map[string]any {
	return map[string]any{
		"time":            time.Now().Unix(),
		"self_id":         selfID,
		"post_type":       "meta_event",
		"meta_event_type": "lifecycle",
		"sub_type":        subType,
	}
}

func HeartbeatEvent(selfID int64, status map[string]any, intervalMS int) map[string]any {
	return map[string]any{
		"time":            time.Now().Unix(),
		"self_id":         selfID,
		"post_type":       "meta_event",
		"meta_event_type": "heartbeat",
		"status":          status,
		"interval":        intervalMS,
	}
}

package bridge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"milky-onebot11-bridge/internal/types"
)

type oneBotSegment struct {
	Type string            `json:"type"`
	Data map[string]string `json:"data"`
}

func parseOneBotMessage(raw json.RawMessage, autoEscape bool) ([]types.Segment, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("message is required")
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}

	switch value := decoded.(type) {
	case string:
		if autoEscape {
			return []types.Segment{{Type: types.SegmentText, Data: map[string]string{"text": value}}}, nil
		}
		return parseCQMessage(value), nil
	case map[string]any:
		return parseSegmentObject(value), nil
	case []any:
		segments := make([]types.Segment, 0, len(value))
		for _, item := range value {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("message segment must be object")
			}
			segments = append(segments, parseSegmentObject(obj)...)
		}
		return segments, nil
	default:
		return nil, fmt.Errorf("unsupported message type %T", decoded)
	}
}

func parseSegmentObject(obj map[string]any) []types.Segment {
	segmentType, _ := obj["type"].(string)
	dataMap := map[string]string{}
	if rawData, ok := obj["data"].(map[string]any); ok {
		for k, v := range rawData {
			dataMap[k] = fmt.Sprint(v)
		}
	}

	switch segmentType {
	case "text":
		return []types.Segment{{Type: types.SegmentText, Data: map[string]string{"text": dataMap["text"]}}}
	case "image":
		return []types.Segment{{Type: types.SegmentImage, Data: normalizeMediaData(dataMap)}}
	case "record":
		return []types.Segment{{Type: types.SegmentRecord, Data: normalizeMediaData(dataMap)}}
	case "at":
		return []types.Segment{{Type: types.SegmentAt, Data: map[string]string{"qq": dataMap["qq"]}}}
	case "reply":
		return []types.Segment{{Type: types.SegmentReply, Data: map[string]string{"id": dataMap["id"]}}}
	case "poke":
		return []types.Segment{{Type: types.SegmentPoke, Data: map[string]string{"qq": dataMap["qq"]}}}
	default:
		text := fmt.Sprintf("[CQ:%s", segmentType)
		if len(dataMap) > 0 {
			parts := make([]string, 0, len(dataMap))
			for k, v := range dataMap {
				parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			}
			text += "," + strings.Join(parts, ",")
		}
		text += "]"
		return []types.Segment{{Type: types.SegmentText, Data: map[string]string{"text": text}}}
	}
}

func parseCQMessage(input string) []types.Segment {
	segments := make([]types.Segment, 0)
	for len(input) > 0 {
		start := strings.Index(input, "[CQ:")
		if start < 0 {
			if input != "" {
				segments = append(segments, types.Segment{Type: types.SegmentText, Data: map[string]string{"text": unescapeCQText(input)}})
			}
			break
		}
		if start > 0 {
			segments = append(segments, types.Segment{Type: types.SegmentText, Data: map[string]string{"text": unescapeCQText(input[:start])}})
		}
		end := strings.Index(input[start:], "]")
		if end < 0 {
			segments = append(segments, types.Segment{Type: types.SegmentText, Data: map[string]string{"text": unescapeCQText(input[start:])}})
			break
		}
		token := input[start+4 : start+end]
		segments = append(segments, parseCQToken(token)...)
		input = input[start+end+1:]
	}
	return segments
}

func parseCQToken(token string) []types.Segment {
	parts := strings.Split(token, ",")
	if len(parts) == 0 {
		return nil
	}
	name := parts[0]
	data := map[string]string{}
	for _, item := range parts[1:] {
		pair := strings.SplitN(item, "=", 2)
		if len(pair) != 2 {
			continue
		}
		data[pair[0]] = unescapeCQText(pair[1])
	}
	switch name {
	case "image":
		return []types.Segment{{Type: types.SegmentImage, Data: normalizeMediaData(data)}}
	case "record":
		return []types.Segment{{Type: types.SegmentRecord, Data: normalizeMediaData(data)}}
	case "at":
		return []types.Segment{{Type: types.SegmentAt, Data: map[string]string{"qq": data["qq"]}}}
	case "reply":
		return []types.Segment{{Type: types.SegmentReply, Data: map[string]string{"id": data["id"]}}}
	case "poke":
		return []types.Segment{{Type: types.SegmentPoke, Data: map[string]string{"qq": data["qq"]}}}
	default:
		return []types.Segment{{Type: types.SegmentText, Data: map[string]string{"text": "[CQ:" + token + "]"}}}
	}
}

func normalizeMediaData(data map[string]string) map[string]string {
	file := data["file"]
	url := data["url"]
	if url == "" {
		url = file
	}
	return map[string]string{
		"file": file,
		"url":  url,
	}
}

func buildOneBotMessage(format string, segments []types.Segment) (any, string) {
	if format == "string" {
		raw := buildCString(segments)
		return raw, raw
	}
	array := make([]oneBotSegment, 0, len(segments))
	for _, segment := range segments {
		switch segment.Type {
		case types.SegmentText:
			array = append(array, oneBotSegment{Type: "text", Data: map[string]string{"text": segment.Data["text"]}})
		case types.SegmentImage:
			array = append(array, oneBotSegment{Type: "image", Data: normalizeMediaData(segment.Data)})
		case types.SegmentRecord:
			array = append(array, oneBotSegment{Type: "record", Data: normalizeMediaData(segment.Data)})
		case types.SegmentAt:
			array = append(array, oneBotSegment{Type: "at", Data: map[string]string{"qq": segment.Data["qq"]}})
		case types.SegmentReply:
			array = append(array, oneBotSegment{Type: "reply", Data: map[string]string{"id": segment.Data["id"]}})
		case types.SegmentPoke:
			array = append(array, oneBotSegment{Type: "poke", Data: map[string]string{"qq": segment.Data["qq"]}})
		default:
			array = append(array, oneBotSegment{Type: "text", Data: map[string]string{"text": segment.Data["text"]}})
		}
	}
	return array, buildCString(segments)
}

func buildCString(segments []types.Segment) string {
	var builder strings.Builder
	for _, segment := range segments {
		switch segment.Type {
		case types.SegmentText:
			builder.WriteString(escapeCQText(segment.Data["text"]))
		case types.SegmentImage:
			file := segment.Data["file"]
			if file == "" {
				file = segment.Data["url"]
			}
			builder.WriteString("[CQ:image,file=" + escapeCQValue(file) + "]")
		case types.SegmentRecord:
			file := segment.Data["file"]
			if file == "" {
				file = segment.Data["url"]
			}
			builder.WriteString("[CQ:record,file=" + escapeCQValue(file) + "]")
		case types.SegmentAt:
			builder.WriteString("[CQ:at,qq=" + escapeCQValue(segment.Data["qq"]) + "]")
		case types.SegmentReply:
			builder.WriteString("[CQ:reply,id=" + escapeCQValue(segment.Data["id"]) + "]")
		case types.SegmentPoke:
			builder.WriteString("[CQ:poke,qq=" + escapeCQValue(segment.Data["qq"]) + "]")
		default:
			builder.WriteString(escapeCQText(segment.Data["text"]))
		}
	}
	return builder.String()
}

func escapeCQText(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "[", "&#91;")
	text = strings.ReplaceAll(text, "]", "&#93;")
	return text
}

func escapeCQValue(text string) string {
	text = escapeCQText(text)
	text = strings.ReplaceAll(text, ",", "&#44;")
	return text
}

func unescapeCQText(text string) string {
	replacer := strings.NewReplacer(
		"&#44;", ",",
		"&#91;", "[",
		"&#93;", "]",
		"&amp;", "&",
	)
	return replacer.Replace(text)
}

func parseInt64FromString(v string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
}

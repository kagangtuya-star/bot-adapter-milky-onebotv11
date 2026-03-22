package milky

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	milky "github.com/Szzrain/Milky-go-sdk"

	"milky-onebot11-bridge/internal/config"
	"milky-onebot11-bridge/internal/types"
)

type Client struct {
	cfg    config.MilkyConfig
	logger *slog.Logger

	mu      sync.RWMutex
	session *milky.Session
	login   types.LoginInfo
	events  chan types.InboundEvent
}

func NewClient(cfg config.MilkyConfig, logger *slog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger,
		events: make(chan types.InboundEvent, 128),
	}
}

func (c *Client) Events() <-chan types.InboundEvent {
	return c.events
}

func (c *Client) Connect(_ context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("milky sdk panic recovered during connect", "panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("milky sdk panic during connect: %v", r)
		}
	}()
	ws := strings.TrimRight(c.cfg.WSGateway, "/")
	rest := strings.TrimRight(c.cfg.RestGateway, "/")
	session, err := milky.New(ws, rest, c.cfg.Token, newSDKLogger(c.logger))
	if err != nil {
		return fmt.Errorf("create milky session: %w", err)
	}

	c.registerHandlers(session)
	if err := session.Open(); err != nil {
		return fmt.Errorf("open milky session: %w", err)
	}

	info, err := session.GetLoginInfo()
	if err != nil {
		_ = session.Close()
		return fmt.Errorf("get login info: %w", err)
	}

	c.mu.Lock()
	c.session = session
	c.login = types.LoginInfo{
		SelfID:   info.UIN,
		Nickname: info.Nickname,
	}
	c.mu.Unlock()

	c.logger.Info("milky connected", "self_id", info.UIN, "nickname", info.Nickname)
	return nil
}

type sdkLogger struct {
	logger *slog.Logger
}

func newSDKLogger(logger *slog.Logger) *sdkLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &sdkLogger{logger: logger.With("component", "milky-sdk")}
}

func (l *sdkLogger) Infof(format string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, args...))
}
func (l *sdkLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(format, args...))
}
func (l *sdkLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}
func (l *sdkLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}
func (l *sdkLogger) Info(args ...interface{})  { l.logger.Info(fmt.Sprint(args...)) }
func (l *sdkLogger) Error(args ...interface{}) { l.logger.Error(fmt.Sprint(args...)) }
func (l *sdkLogger) Debug(args ...interface{}) { l.logger.Debug(fmt.Sprint(args...)) }
func (l *sdkLogger) Warn(args ...interface{})  { l.logger.Warn(fmt.Sprint(args...)) }

func (c *Client) Close() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.session == nil {
		return nil
	}
	return c.session.Close()
}

func (c *Client) Login() types.LoginInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.login
}

func (c *Client) SendPrivateMessage(userID int64, segments []types.Segment) (int64, error) {
	elements, err := toMilkySegments(segments)
	if err != nil {
		return 0, err
	}
	session, err := c.currentSession()
	if err != nil {
		return 0, err
	}
	ret, err := session.SendPrivateMessage(userID, &elements)
	if err != nil {
		return 0, err
	}
	return ret.MessageSeq, nil
}

func (c *Client) SendGroupMessage(groupID int64, segments []types.Segment) (int64, error) {
	elements, err := toMilkySegments(segments)
	if err != nil {
		return 0, err
	}
	session, err := c.currentSession()
	if err != nil {
		return 0, err
	}
	ret, err := session.SendGroupMessage(groupID, &elements)
	if err != nil {
		return 0, err
	}
	return ret.MessageSeq, nil
}

func (c *Client) GetGroupInfo(groupID int64) (types.GroupInfo, error) {
	session, err := c.currentSession()
	if err != nil {
		return types.GroupInfo{}, err
	}
	info, err := session.GetGroupInfo(groupID, true)
	if err != nil {
		return types.GroupInfo{}, err
	}
	if info == nil {
		return types.GroupInfo{}, errors.New("nil group info")
	}
	return parseGroupInfo(info), nil
}

func (c *Client) GetGroupList() ([]types.GroupInfo, error) {
	session, err := c.currentSession()
	if err != nil {
		return nil, err
	}
	list, err := session.GetGroupList(true)
	if err != nil {
		return nil, err
	}
	result := make([]types.GroupInfo, 0, len(list))
	for i := range list {
		result = append(result, parseGroupInfo(&list[i]))
	}
	return result, nil
}

func (c *Client) GetGroupMemberInfo(groupID, userID int64) (types.GroupMemberInfo, error) {
	session, err := c.currentSession()
	if err != nil {
		return types.GroupMemberInfo{}, err
	}
	info, err := session.GetGroupMemberInfo(groupID, userID, true)
	if err != nil {
		return types.GroupMemberInfo{}, err
	}
	if info == nil {
		return types.GroupMemberInfo{}, errors.New("nil group member info")
	}
	return parseGroupMemberInfo(info), nil
}

func (c *Client) GetGroupMemberList(groupID int64) ([]types.GroupMemberInfo, error) {
	session, err := c.currentSession()
	if err != nil {
		return nil, err
	}
	list, err := session.GetGroupMemberList(groupID, true)
	if err != nil {
		return nil, err
	}
	result := make([]types.GroupMemberInfo, 0, len(list))
	for i := range list {
		result = append(result, parseGroupMemberInfo(&list[i]))
	}
	return result, nil
}

func (c *Client) GetMessage(ref types.MessageRef) (*types.InboundEvent, error) {
	session, err := c.currentSession()
	if err != nil {
		return nil, err
	}
	scene := ref.MessageType
	peerID := ref.UserID
	if scene == "group" {
		peerID = ref.GroupID
	}
	msg, err := session.GetMessage(scene, peerID, ref.MilkySeq)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, errors.New("nil message")
	}
	event, ok := fromReceiveMessage(msg)
	if !ok {
		return nil, errors.New("unsupported message scene")
	}
	return &event, nil
}

func (c *Client) DeleteMessage(ref types.MessageRef) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	switch ref.MessageType {
	case "private":
		return session.RecallPrivateMessage(ref.UserID, ref.MilkySeq)
	case "group":
		return session.RecallGroupMessage(ref.GroupID, ref.MilkySeq)
	default:
		return fmt.Errorf("unsupported message type for recall: %s", ref.MessageType)
	}
}

func (c *Client) HandleFriendRequest(ref types.RequestRef, approve bool, reason string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	if approve {
		return session.AcceptFriendRequest(ref.InitiatorUID, false)
	}
	return session.RejectFriendRequest(ref.InitiatorUID, false, reason)
}

func (c *Client) HandleGroupRequest(ref types.RequestRef, approve bool) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	if approve {
		return session.AcceptGroupInvitation(ref.GroupID, ref.InvitationSeq)
	}
	return session.RejectGroupInvitation(ref.GroupID, ref.InvitationSeq)
}

func (c *Client) currentSession() (*milky.Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.session == nil {
		return nil, errors.New("milky session is not connected")
	}
	return c.session, nil
}

func (c *Client) registerHandlers(session *milky.Session) {
	session.AddHandler(func(_ *milky.Session, m *milky.ReceiveMessage) {
		if m == nil {
			return
		}
		event, ok := fromReceiveMessage(m)
		if !ok {
			return
		}
		c.emit(event)
	})

	session.AddHandler(func(_ *milky.Session, m *milky.GroupNudge) {
		if m == nil {
			return
		}
		c.emit(types.InboundEvent{
			Kind:     types.EventPokeGroup,
			Time:     0,
			GroupID:  m.GroupID,
			UserID:   m.SenderID,
			TargetID: m.ReceiverID,
		})
	})

	session.AddHandler(func(_ *milky.Session, m *milky.FriendNudge) {
		if m == nil {
			return
		}
		targetID := m.UserID
		if m.IsSelfReceive {
			targetID = c.Login().SelfID
		}
		c.emit(types.InboundEvent{
			Kind:     types.EventPokePrivate,
			Time:     0,
			UserID:   m.UserID,
			TargetID: targetID,
		})
	})

	session.AddHandler(func(_ *milky.Session, m *milky.FriendRequest) {
		if m == nil {
			return
		}
		c.emit(types.InboundEvent{
			Kind:    types.EventFriendRequest,
			Time:    0,
			UserID:  m.InitiatorID,
			Comment: m.Comment,
			Request: &types.RequestRef{
				Kind:         "friend",
				InitiatorUID: m.InitiatorUID,
			},
		})
	})

	session.AddHandler(func(_ *milky.Session, m *milky.GroupInvitation) {
		if m == nil {
			return
		}
		c.emit(types.InboundEvent{
			Kind:    types.EventGroupInvite,
			Time:    0,
			GroupID: m.GroupID,
			UserID:  m.InitiatorID,
			Request: &types.RequestRef{
				Kind:          "group",
				GroupID:       m.GroupID,
				InvitationSeq: m.InvitationSeq,
			},
		})
	})

	session.AddHandler(func(_ *milky.Session, m *milky.MessageRecall) {
		if m == nil {
			return
		}
		event := types.InboundEvent{
			Time:      0,
			MessageID: m.MessageSeq,
			UserID:    m.SenderID,
		}
		switch m.MessageScene {
		case "group":
			event.Kind = types.EventRecallGroup
			event.GroupID = m.PeerID
		case "friend":
			event.Kind = types.EventRecallPrivate
		default:
			return
		}
		c.emit(event)
	})
}

func (c *Client) emit(event types.InboundEvent) {
	select {
	case c.events <- event:
	default:
		c.logger.Warn("drop milky event because buffer is full", "kind", event.Kind)
	}
}

func toMilkySegments(segments []types.Segment) ([]milky.IMessageElement, error) {
	elements := make([]milky.IMessageElement, 0, len(segments))
	for _, segment := range segments {
		switch segment.Type {
		case types.SegmentText:
			elements = append(elements, &milky.TextElement{Text: segment.Data["text"]})
		case types.SegmentImage:
			uri := segment.Data["url"]
			if uri == "" {
				uri = segment.Data["file"]
			}
			if uri == "" {
				return nil, errors.New("image segment requires url or file")
			}
			elements = append(elements, &milky.ImageElement{
				URI:     uri,
				Summary: filepath.Base(uri),
				SubType: "normal",
			})
		case types.SegmentAt:
			userID, err := strconv.ParseInt(segment.Data["qq"], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid at qq: %w", err)
			}
			elements = append(elements, &milky.AtElement{UserID: userID})
		case types.SegmentReply:
			id, err := strconv.ParseInt(segment.Data["id"], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid reply id: %w", err)
			}
			elements = append(elements, &milky.ReplyElement{MessageSeq: id})
		case types.SegmentRecord:
			uri := segment.Data["url"]
			if uri == "" {
				uri = segment.Data["file"]
			}
			if uri == "" {
				return nil, errors.New("record segment requires url or file")
			}
			elements = append(elements, &milky.RecordElement{URI: uri})
		case types.SegmentPoke:
			continue
		default:
			text := segment.Data["text"]
			if text != "" {
				elements = append(elements, &milky.TextElement{Text: text})
			}
		}
	}
	return elements, nil
}

func fromMilkySegments(raw []milky.IMessageElement) []types.Segment {
	segments := make([]types.Segment, 0, len(raw))
	for _, segment := range raw {
		switch seg := segment.(type) {
		case *milky.TextElement:
			segments = append(segments, types.Segment{Type: types.SegmentText, Data: map[string]string{"text": seg.Text}})
		case *milky.ImageElement:
			url := seg.TempURL
			if url == "" {
				url = seg.URI
			}
			segments = append(segments, types.Segment{Type: types.SegmentImage, Data: map[string]string{"url": url}})
		case *milky.AtElement:
			segments = append(segments, types.Segment{Type: types.SegmentAt, Data: map[string]string{"qq": strconv.FormatInt(seg.UserID, 10)}})
		case *milky.ReplyElement:
			segments = append(segments, types.Segment{Type: types.SegmentReply, Data: map[string]string{"id": strconv.FormatInt(seg.MessageSeq, 10)}})
		case *milky.RecordElement:
			url := seg.TempURL
			if url == "" {
				url = seg.URI
			}
			segments = append(segments, types.Segment{Type: types.SegmentRecord, Data: map[string]string{"url": url}})
		default:
			segments = append(segments, types.Segment{Type: types.SegmentUnknown, Data: map[string]string{"text": fmt.Sprintf("[unsupported:%T]", segment)}})
		}
	}
	return segments
}

func parseGroupInfo(info *milky.GroupInfo) types.GroupInfo {
	if info == nil {
		return types.GroupInfo{}
	}
	return types.GroupInfo{
		GroupID:        info.GroupId,
		GroupName:      info.Name,
		MemberCount:    info.MemberCount,
		MaxMemberCount: info.MaxMemberCount,
	}
}

func parseGroupMemberInfo(info *milky.GroupMemberInfo) types.GroupMemberInfo {
	if info == nil {
		return types.GroupMemberInfo{}
	}
	return types.GroupMemberInfo{
		GroupID:  info.GroupId,
		UserID:   info.UserId,
		Nickname: info.Nickname,
		Card:     info.Card,
		Sex:      info.Sex,
		Level:    strconv.Itoa(int(info.Level)),
		Role:     info.Role,
		Title:    info.Title,
	}
}

func fromReceiveMessage(m *milky.ReceiveMessage) (types.InboundEvent, bool) {
	if m == nil {
		return types.InboundEvent{}, false
	}
	event := types.InboundEvent{
		Time:      m.Time,
		MessageID: m.MessageSeq,
		UserID:    m.SenderId,
		Sender: types.Sender{
			UserID: m.SenderId,
		},
		Segments: fromMilkySegments(m.Segments),
	}
	switch m.MessageScene {
	case "group":
		event.Kind = types.EventMessageGroup
		event.GroupID = m.PeerId
		if m.Group != nil {
			event.GroupID = m.Group.GroupId
		}
		if m.GroupMember != nil {
			member := parseGroupMemberInfo(m.GroupMember)
			event.Sender = types.Sender{
				UserID:   member.UserID,
				Nickname: member.Nickname,
				Card:     member.Card,
				Sex:      member.Sex,
				Level:    member.Level,
				Role:     member.Role,
				Title:    member.Title,
			}
		}
		return event, true
	case "friend":
		event.Kind = types.EventMessagePrivate
		if m.Friend != nil {
			event.Sender.Nickname = m.Friend.Nickname
			event.Sender.Sex = m.Friend.Sex
		}
		return event, true
	default:
		return types.InboundEvent{}, false
	}
}

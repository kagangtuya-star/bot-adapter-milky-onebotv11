package types

type SegmentType string

const (
	SegmentText    SegmentType = "text"
	SegmentImage   SegmentType = "image"
	SegmentAt      SegmentType = "at"
	SegmentReply   SegmentType = "reply"
	SegmentRecord  SegmentType = "record"
	SegmentFile    SegmentType = "file"
	SegmentPoke    SegmentType = "poke"
	SegmentUnknown SegmentType = "unknown"
)

type Segment struct {
	Type SegmentType       `json:"type"`
	Data map[string]string `json:"data,omitempty"`
	Raw  map[string]any    `json:"raw,omitempty"`
}

type EventKind string

const (
	EventMessagePrivate EventKind = "message_private"
	EventMessageGroup   EventKind = "message_group"
	EventPokePrivate    EventKind = "poke_private"
	EventPokeGroup      EventKind = "poke_group"
	EventFriendRequest  EventKind = "friend_request"
	EventGroupInvite    EventKind = "group_invite"
	EventRecallPrivate  EventKind = "recall_private"
	EventRecallGroup    EventKind = "recall_group"
)

type Sender struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int32  `json:"age,omitempty"`
	Area     string `json:"area,omitempty"`
	Level    string `json:"level,omitempty"`
	Role     string `json:"role,omitempty"`
	Title    string `json:"title,omitempty"`
}

type RequestRef struct {
	Kind          string `json:"kind"`
	InitiatorUID  string `json:"initiator_uid,omitempty"`
	GroupID       int64  `json:"group_id,omitempty"`
	InvitationSeq int64  `json:"invitation_seq,omitempty"`
}

type InboundEvent struct {
	Kind      EventKind   `json:"kind"`
	Time      int64       `json:"time"`
	MessageID int64       `json:"message_id,omitempty"`
	GroupID   int64       `json:"group_id,omitempty"`
	UserID    int64       `json:"user_id,omitempty"`
	TargetID  int64       `json:"target_id,omitempty"`
	Segments  []Segment   `json:"segments,omitempty"`
	Sender    Sender      `json:"sender,omitempty"`
	Comment   string      `json:"comment,omitempty"`
	Request   *RequestRef `json:"request,omitempty"`
}

type LoginInfo struct {
	SelfID   int64  `json:"self_id"`
	Nickname string `json:"nickname"`
}

type GroupInfo struct {
	GroupID        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	MemberCount    int32  `json:"member_count,omitempty"`
	MaxMemberCount int32  `json:"max_member_count,omitempty"`
}

type GroupMemberInfo struct {
	GroupID  int64  `json:"group_id"`
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int32  `json:"age,omitempty"`
	Area     string `json:"area,omitempty"`
	Level    string `json:"level,omitempty"`
	Role     string `json:"role,omitempty"`
	Title    string `json:"title,omitempty"`
}

type Status struct {
	Online bool `json:"online"`
	Good   bool `json:"good"`
}

type MessageRef struct {
	OneBotID    int64  `json:"onebot_id"`
	MilkySeq    int64  `json:"milky_seq"`
	MessageType string `json:"message_type"`
	GroupID     int64  `json:"group_id,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
}

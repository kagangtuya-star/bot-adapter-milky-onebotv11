package state

import (
	"sync"

	"milky-onebot11-bridge/internal/types"
)

type MessageMap struct {
	mu    sync.RWMutex
	items map[int64]types.MessageRef
}

func NewMessageMap() *MessageMap {
	return &MessageMap{items: make(map[int64]types.MessageRef)}
}

func (m *MessageMap) Put(ref types.MessageRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[ref.OneBotID] = ref
}

func (m *MessageMap) Get(oneBotID int64) (types.MessageRef, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ref, ok := m.items[oneBotID]
	return ref, ok
}

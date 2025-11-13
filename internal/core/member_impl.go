package core

import (
	"sync"

	"github.com/dkeye/Voice/internal/domain"
	"github.com/rs/zerolog/log"
)

// memberSessionImpl implements MemberSession by pairing meta + transport.
type memberSessionImpl struct {
	meta *domain.Member

	smu    sync.RWMutex
	signal SignalConnection

	mmu   sync.RWMutex
	media MediaConnection
}

func NewMemberSession(meta *domain.Member) MemberSession {
	return &memberSessionImpl{meta: meta}
}

func (m *memberSessionImpl) Meta() *domain.Member { return m.meta }
func (m *memberSessionImpl) Signal() SignalConnection {
	m.smu.RLock()
	defer m.smu.RUnlock()
	return m.signal
}

func (m *memberSessionImpl) Media() MediaConnection {
	m.mmu.RLock()
	defer m.mmu.RUnlock()
	return m.media
}

func (m *memberSessionImpl) UpdateSignal(signalConn SignalConnection) MemberSession {
	m.smu.Lock()
	defer m.smu.Unlock()
	m.signal = signalConn
	log.Info().Str("module", "core.member").Str("user", string(m.meta.User.ID)).Msg("signal updated")
	return m
}

func (m *memberSessionImpl) UpdateMedia(mediaConn MediaConnection) MemberSession {
	m.mmu.Lock()
	defer m.mmu.Unlock()
	m.media = mediaConn
	log.Info().Str("module", "core.member").Str("user", string(m.meta.User.ID)).Msg("media updated")
	return m
}

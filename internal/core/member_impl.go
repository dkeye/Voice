package core

import "github.com/dkeye/Voice/internal/domain"

// memberSession implements MemberSession by pairing meta + transport.
type memberSession struct {
	meta *domain.Member
	signal SignalConnection
}

func NewMemberSession(meta *domain.Member, conn SignalConnection) MemberSession {
	return &memberSession{meta: meta, signal: conn}
}

func (m *memberSession) Meta() *domain.Member     { return m.meta }
func (m *memberSession) Signal() SignalConnection { return m.signal }

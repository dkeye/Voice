package core

import "github.com/dkeye/Voice/internal/domain"

// memberSession implements MemberSession by pairing meta + transport.
type memberSession struct {
	meta *domain.Member
	conn MemberConnection
}

func NewMemberSession(meta *domain.Member, conn MemberConnection) MemberSession {
	return &memberSession{meta: meta, conn: conn}
}

func (m *memberSession) Meta() *domain.Member   { return m.meta }
func (m *memberSession) Conn() MemberConnection { return m.conn }

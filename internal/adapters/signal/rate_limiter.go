package signal

import (
	"sync"
	"time"

	"github.com/dkeye/Voice/internal/domain"
)

type RoomRateLimiter struct {
	mu       sync.Mutex
	history  map[domain.UserID][]time.Time
	limit    int
	interval time.Duration
}

func NewRoomRateLimiter(limit int, interval time.Duration) *RoomRateLimiter {
	return &RoomRateLimiter{
		history:  make(map[domain.UserID][]time.Time),
		limit:    limit,
		interval: interval,
	}
}

func (rl *RoomRateLimiter) Allow(uid domain.UserID) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.interval)

	// 1. Берем историю пользователя
	attempts := rl.history[uid]

	// 2. Убираем старые попытки
	fresh := make([]time.Time, 0, len(attempts))
	for _, t := range attempts {
		if t.After(windowStart) {
			fresh = append(fresh, t)
		}
	}

	// 3. Если свежих попыток >= лимита → блок
	if len(fresh) >= rl.limit {
		return false
	}

	// 4. Иначе добавить текущую попытку
	fresh = append(fresh, now)
	rl.history[uid] = fresh

	return true
}

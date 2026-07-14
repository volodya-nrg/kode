package other

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type Storage struct {
	m  map[uuid.UUID]ScheduleRecord
	mu sync.RWMutex
}

func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k := range s.m {
		delete(s.m, k)
	}
	s.m = nil

	return nil
}

func (s *Storage) Create(item ScheduleRecord) (uuid.UUID, error) {
	newScheduleUUID, err := uuid.NewUUID()
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create a new UUID: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item.ScheduleID = newScheduleUUID
	s.m[newScheduleUUID] = item

	return newScheduleUUID, nil
}

func (s *Storage) One(scheduleID uuid.UUID) (*ScheduleRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if item, ok := s.m[scheduleID]; ok {
		return &item, nil
	}

	return nil, fmt.Errorf("schedule not found")
}

func (s *Storage) ByUserID(userID uuid.UUID) ([]ScheduleRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]ScheduleRecord, 0)
	for _, v := range s.m {
		if v.UserID == userID {
			items = append(items, v)
		}
	}

	return items, nil
}

func NewStorage() *Storage {
	return &Storage{
		m: make(map[uuid.UUID]ScheduleRecord),
	}
}

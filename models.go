package other

import (
	"time"

	"github.com/google/uuid"
)

type ScheduleRecord struct {
	ScheduleID uuid.UUID     // id расписания
	UserID     uuid.UUID     // id пользователя
	DrugName   string        // наименование лекарства
	Period     time.Duration // периодичность приёмов, например в день или в неделю
	TTL        time.Time     // крайнее время когда необходимо принимать лекарство
}

type ScheduleItem struct {
	ScheduleID uuid.UUID
	UserID     uuid.UUID
	DrugName   string
	Period     time.Duration
	TTL        time.Time
}

type NearestDrugItem struct {
	DrugName string
	Time     time.Time
}

type ResponseUUID struct {
	UUID uuid.UUID `json:"uuid"`
}

type ResponseUUIDs struct {
	UUIDs []uuid.UUID `json:"uuids"`
}

type ResponseScheduleByDay struct {
	Times []string `json:"times"`
}

type ResponseNearestDrug struct {
	DrugName string    `json:"drug_name"`
	Time     time.Time `json:"time"`
}

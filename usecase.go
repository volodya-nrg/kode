package other

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	dayHourStart = 8
	dayHourEnd   = 22
)

type storage interface {
	Create(item ScheduleRecord) (uuid.UUID, error)
	One(scheduleID uuid.UUID) (*ScheduleRecord, error)
	ByUserID(userID uuid.UUID) ([]ScheduleRecord, error)
}

type nowFunc func() time.Time

type UseCase struct {
	multiplicityMinutes int
	nearestPeriod       time.Duration
	storage             storage
	nowFunc             nowFunc
}

func (u *UseCase) CreateSchedule(
	item ScheduleItem,
) (uuid.UUID, error) {
	scheduleUUID, err := u.storage.Create(ScheduleToDB(item))
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create a new schedule: %w", err)
	}

	return scheduleUUID, nil
}

// GetScheduleByDay расчет времени приёма на сегодняшний день (8:00-22:00), интервал кратный 15 (в минутах).
// Во внимание берется только диапазон 8:00-22:00. Какое время суток на сегодня, не важно. Показываем просто расчет.
func (u *UseCase) GetScheduleByDay(scheduleID uuid.UUID) ([]time.Time, error) {
	scheduleDB, err := u.storage.One(scheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	var (
		schedule = ScheduleToItem(*scheduleDB)
		times    = make([]time.Time, 0)
		now      = u.nowFunc()
		diffTime = now.Sub(schedule.TTL)
	)

	if diffTime < 0 { // "< 0" - время в запасе. Сразу проверим, чтоб лишнее не вычислять.
		// начало дня должно начинаться с 8 утра, чтоб итерация была сведена к минимуму
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), dayHourStart, 0, 0, 0, now.Location())
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), dayHourEnd, 0, 0, 0, now.Location())

		for _, v := range u.calc(startOfDay, endOfDay, schedule) {
			times = append(times, v.Time)
		}
	}

	return times, nil
}

func (u *UseCase) GetScheduleIDs(userID uuid.UUID) ([]uuid.UUID, error) {
	schedules, err := u.storage.ByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}

	ids := make([]uuid.UUID, len(schedules))
	for k, v := range schedules {
		ids[k] = v.ScheduleID
	}

	return ids, nil
}

// GetNearestDrugs получение название таблеток и их время приёма (кратное), в ближайшее время
func (u *UseCase) GetNearestDrugs(userID uuid.UUID) ([]NearestDrugItem, error) {
	// получим все расписания для данного пользователя
	schedulesDb, err := u.storage.ByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}

	schedules := make([]ScheduleItem, len(schedulesDb))
	for k, v := range schedulesDb {
		schedules[k] = ScheduleToItem(v)
	}

	// возьмем нужное расписание приема таблеток в определенное ближайшее время
	timeStart := u.nowFunc()
	nearestDrugItems := make([]NearestDrugItem, 0)
	for _, schedule := range schedules {
		timeEnd := timeStart.Add(u.nearestPeriod)
		if schedule.TTL.Before(timeEnd) { // тут обращаем внимание на время жизни данного расписания
			timeEnd = schedule.TTL
		}

		nearestDrugItems = append(nearestDrugItems, u.calc(timeStart, timeEnd, schedule)...)
	}

	// результат надо отсортировать по времени, от раннего к позднему
	sort.Slice(nearestDrugItems, func(i, j int) bool {
		return nearestDrugItems[i].Time.Before(nearestDrugItems[j].Time)
	})

	return nearestDrugItems, nil
}

// calc основной алгоритм расчета
func (u *UseCase) calc(start, end time.Time, schedule ScheduleItem) []NearestDrugItem {
	result := make([]NearestDrugItem, 0)
	tmpTime := start.Add(schedule.Period)

	// В каждом цикле обязательно добавляем время. Все что попадает в диапазон, возьмем.
	for {
		if tmpTime.After(end) {
			break
		}

		// только с 8 утра и до 22 ночи (не включительно)
		if hour := tmpTime.Hour(); hour >= dayHourStart && hour < dayHourEnd { // 22:01 уже не входит в диапазон
			// если есть остаток минут от кратного числа, то округлим до него в большую сторону
			if ost := tmpTime.Minute() % u.multiplicityMinutes; ost > 0 {
				tmpTime = tmpTime.Add(time.Duration(u.multiplicityMinutes-ost) * time.Minute)
			}

			result = append(result, NearestDrugItem{
				DrugName: schedule.DrugName,
				Time:     tmpTime,
			})
		}

		tmpTime = tmpTime.Add(schedule.Period)
	}

	return result
}

func NewUseCase(
	multiplicityMinutes int,
	nearestPeriod time.Duration,
	storage storage,
	nowFunc nowFunc,
) *UseCase {
	return &UseCase{
		multiplicityMinutes: multiplicityMinutes,
		nearestPeriod:       nearestPeriod,
		storage:             storage,
		nowFunc:             nowFunc,
	}
}

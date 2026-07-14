package other

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type usecase interface {
	GetScheduleByDay(scheduleID uuid.UUID) ([]time.Time, error)
	CreateSchedule(item ScheduleItem) (uuid.UUID, error)
	GetScheduleIDs(userID uuid.UUID) ([]uuid.UUID, error)
	GetNearestDrugs(userID uuid.UUID) ([]NearestDrugItem, error)
}

// Server упрощенный http-сервер
type Server struct {
	srv *http.Server
}

func (s *Server) Close() error {
	if err := s.srv.Close(); err != nil {
		return fmt.Errorf("failed to close http-server: %w", err)
	}
	return nil
}

func (s *Server) Run() error {
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to listen and serve: %w", err)
	}
	return nil
}

// NewServer фабрика. Для простоты хендлеры указанны внутри.
func NewServer(addr string, usecase usecase) *Server {
	mux := http.NewServeMux()

	// Get: возвращает данные о выбранном расписании с рассчитанным графиком приёмов на день
	// Post: создание расписания приемов на день и возвращение id расписания
	//  	user_id - id пользователя
	// 		drug_name - наименование лекарства
	//		period - периодичность приёмов за день
	//  	ttl - продолжительность лечения, например до определенной даты
	mux.HandleFunc("/schedule", func(w http.ResponseWriter, r *http.Request) {
		// если создают расписание
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, fmt.Sprintf("failed to parse form: %s", err), http.StatusInternalServerError)
				return
			}

			drugName := strings.TrimSpace(r.FormValue(RequestFieldDrugName))
			if drugName == "" {
				http.Error(w, "drug_name is empty", http.StatusBadRequest)
				return
			}

			userID, err := uuid.Parse(r.FormValue(RequestFieldUserID))
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to parse user_id: %s", err), http.StatusBadRequest)
				return
			}
			if userID == uuid.Nil {
				http.Error(w, "user_id is empty", http.StatusBadRequest)
				return
			}

			period, err := time.ParseDuration(r.FormValue(RequestFieldPeriod))
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to parse period: %s", err), http.StatusBadRequest)
				return
			}

			ttl, err := time.Parse(time.RFC3339, r.FormValue(RequestFieldTTL))
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to parse ttl: %s", err), http.StatusBadRequest)
				return
			}

			scheduleItem := ScheduleItem{
				DrugName: drugName,
				UserID:   userID,
				Period:   period,
				TTL:      ttl,
			}

			scheduleID, err := usecase.CreateSchedule(scheduleItem)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to create schedule: %s", err), http.StatusInternalServerError)
				return
			}

			resp := ResponseUUID{UUID: scheduleID}

			w.Header().Set("Content-Type", ContentTypeJSON)

			if err = json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, fmt.Sprintf("failed to json encode: %s", err), http.StatusInternalServerError)
				return
			}
		} else if r.Method == http.MethodGet { // получение расписания
			// TODO userID тут не нужен, при наличии scheduleID

			scheduleID, err := uuid.Parse(r.URL.Query().Get(RequestFieldScheduleID))
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to parse schedule_id: %s", err), http.StatusBadRequest)
				return
			}

			// получаем расписание приёма на текущий день определенного лекарства
			times, err := usecase.GetScheduleByDay(scheduleID)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to get schedule by day: %s", err), http.StatusInternalServerError)
				return
			}

			resp := ResponseScheduleByDay{
				Times: make([]string, len(times)),
			}
			for k, v := range times {
				resp.Times[k] = v.Format(time.TimeOnly) // возьмем только время сегодняшнего дня
			}

			w.Header().Set("Content-Type", ContentTypeJSON)

			if err = json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, fmt.Sprintf("failed to json encode: %s", err), http.StatusInternalServerError)
				return
			}
		} else { // остальное все игнорится
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// возвращает список идентификаторов существующих расписаний для указанного пользователя
	mux.HandleFunc("/schedules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}

		userID, err := uuid.Parse(r.URL.Query().Get(RequestFieldUserID))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to parse user_id: %s", err), http.StatusBadRequest)
			return
		}

		scheduleIDs, err := usecase.GetScheduleIDs(userID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get schedule ids: %s", err), http.StatusInternalServerError)
			return
		}

		resp := ResponseUUIDs{
			UUIDs: scheduleIDs,
		}

		w.Header().Set("Content-Type", ContentTypeJSON)

		if err = json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("failed to json encode: %s", err), http.StatusInternalServerError)
			return
		}
	})

	// возвращает данные о таблетках, которые необходимо принять в ближайшее время
	mux.HandleFunc("/next_takings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}

		userID, err := uuid.Parse(r.URL.Query().Get(RequestFieldUserID))
		if err != nil {
			http.Error(w, "failed to parse userID", http.StatusBadRequest)
			return
		}

		nearestDrugs, err := usecase.GetNearestDrugs(userID)
		if err != nil {
			http.Error(w, "failed to get nearest drugs", http.StatusInternalServerError)
			return
		}

		resp := NearestDrugsToTransport(nearestDrugs)

		w.Header().Set("Content-Type", ContentTypeJSON)

		if err = json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("failed to json encode: %s", err), http.StatusInternalServerError)
			return
		}
	})

	return &Server{
		srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

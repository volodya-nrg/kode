package other

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKodeRu(t *testing.T) {
	t.Parallel()

	port, err := freeport.GetFreePort()
	require.NoError(t, err)

	var (
		fixedTime      = time.Date(2026, 7, 14, 16, 22, 0, 0, time.UTC) // явно полдень
		addr           = fmt.Sprintf("localhost:%d", port)
		fullAddr       = "http://" + addr
		storageService = NewStorage()
		useCase        = NewUseCase(15, 48*time.Hour, storageService, func() time.Time { return fixedTime })
		server         = NewServer(addr, useCase)
		cl             = http.Client{Timeout: 3 * time.Second}
	)

	go func() {
		assert.NoError(t, server.Run())
	}()
	t.Cleanup(func() {
		assert.NoError(t, server.Close())
		assert.NoError(t, storageService.Close())
		cl.CloseIdleConnections()
	})

	t.Run("check GET /schedule on error", func(t *testing.T) {
		// error: не хватает schedule_id
		resp, err := cl.Get(fmt.Sprintf("%s/schedule", fullAddr))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// error: не валидный schedule_id
		resp, err = cl.Get(fmt.Sprintf("%s/schedule?%s=abc", fullAddr, RequestFieldScheduleID))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// error: не известный schedule_id
		resp, err = cl.Get(fmt.Sprintf("%s/schedule?%s=%s", fullAddr, RequestFieldScheduleID, uuid.NewString()))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusInternalServerError)
	})
	t.Run("check POST /schedule", func(t *testing.T) {
		req := url.Values{}
		checkOnErrors := [][2]string{
			{"", ""},
			{RequestFieldDrugName, ""},
			{RequestFieldDrugName, "a"},
			{RequestFieldUserID, ""},
			{RequestFieldUserID, "a"},
			{RequestFieldUserID, uuid.NewString()},
			{RequestFieldPeriod, ""},
			{RequestFieldPeriod, "a"},
			{RequestFieldPeriod, "1m"},
			{RequestFieldTTL, ""},
			{RequestFieldTTL, "a"},
		}
		for _, v := range checkOnErrors {
			req.Set(v[0], v[1])
			resp, err := cl.PostForm(fmt.Sprintf("%s/schedule", fullAddr), req)
			require.NoError(t, err)
			handleClientResponse(t, resp, http.StatusBadRequest)
		}

		req = url.Values{}
		req.Add(RequestFieldDrugName, "a")
		req.Add(RequestFieldUserID, uuid.NewString())
		req.Add(RequestFieldPeriod, "61m")
		req.Add(RequestFieldTTL, fixedTime.Add(1*time.Minute).Format(time.RFC3339)) // актуальное время
		resp, err := cl.PostForm(fmt.Sprintf("%s/schedule", fullAddr), req)
		require.NoError(t, err)
		respUUID := ResponseUUID{}
		require.NoError(t, json.Unmarshal(handleClientResponse(t, resp, http.StatusOK), &respUUID))
		require.NotEqual(t, uuid.Nil, respUUID.UUID)

		resp, err = cl.Get(fmt.Sprintf("%s/schedule?%s=%s", fullAddr, RequestFieldScheduleID, respUUID.UUID))
		require.NoError(t, err)
		respScheduleByDay := ResponseScheduleByDay{}
		require.NoError(t, json.Unmarshal(handleClientResponse(t, resp, http.StatusOK), &respScheduleByDay))
		require.Len(t, respScheduleByDay.Times, 11)
	})
	t.Run("check GET /schedules", func(t *testing.T) {
		// error: не хватает user_id
		resp, err := cl.Get(fmt.Sprintf("%s/schedules", fullAddr))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// error: пустой user_id
		resp, err = cl.Get(fmt.Sprintf("%s/schedules?%s=", fullAddr, RequestFieldUserID))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// error: не корректный user_id
		resp, err = cl.Get(fmt.Sprintf("%s/schedules?%s=abc", fullAddr, RequestFieldUserID))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// ok: левый корректный user_id
		resp, err = cl.Get(fmt.Sprintf("%s/schedules?%s=%s", fullAddr, RequestFieldUserID, uuid.NewString()))
		require.NoError(t, err)
		var respUUIDs ResponseUUIDs
		require.NoError(t, json.Unmarshal(handleClientResponse(t, resp, http.StatusOK), &respUUIDs))
		require.Empty(t, respUUIDs.UUIDs)

		// создадим несколько расписаний и получим их id
		const (
			amountAll    = 5
			amountTarget = amountAll - 2
		)
		userID1 := uuid.NewString()
		userID2 := uuid.NewString()

		for i := 1; i <= amountAll; i++ {
			userIDLoc := userID1
			if i > amountTarget {
				userIDLoc = userID2
			}

			req := url.Values{}
			req.Add(RequestFieldDrugName, "a")
			req.Add(RequestFieldUserID, userIDLoc)
			req.Add(RequestFieldPeriod, "1h")
			req.Add(RequestFieldTTL, fixedTime.Add(1*time.Minute).Format(time.RFC3339))
			resp, err = cl.PostForm(fmt.Sprintf("%s/schedule", fullAddr), req)
			require.NoError(t, err)
			handleClientResponse(t, resp, http.StatusOK)
		}

		resp, err = cl.Get(fmt.Sprintf("%s/schedules?%s=%s", fullAddr, RequestFieldUserID, userID1))
		require.NoError(t, err)
		respUUIDs = ResponseUUIDs{}
		require.NoError(t, json.Unmarshal(handleClientResponse(t, resp, http.StatusOK), &respUUIDs))
		require.Len(t, respUUIDs.UUIDs, amountTarget)

		resp, err = cl.Get(fmt.Sprintf("%s/schedules?%s=%s", fullAddr, RequestFieldUserID, userID2))
		require.NoError(t, err)
		respUUIDs = ResponseUUIDs{}
		require.NoError(t, json.Unmarshal(handleClientResponse(t, resp, http.StatusOK), &respUUIDs))
		require.Len(t, respUUIDs.UUIDs, amountAll-amountTarget)
	})
	t.Run("check GET /next_takings", func(t *testing.T) {
		// error: не хватает user_id
		resp, err := cl.Get(fmt.Sprintf("%s/next_takings", fullAddr))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// error: пустой user_id
		resp, err = cl.Get(fmt.Sprintf("%s/next_takings?%s=", fullAddr, RequestFieldUserID))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// error: не корректный user_id
		resp, err = cl.Get(fmt.Sprintf("%s/next_takings?%s=abc", fullAddr, RequestFieldUserID))
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusBadRequest)

		// ok: левый корректный user_id
		resp, err = cl.Get(fmt.Sprintf("%s/next_takings?%s=%s", fullAddr, RequestFieldUserID, uuid.NewString()))
		require.NoError(t, err)
		var respNearestDrugs []ResponseNearestDrug
		require.NoError(t, json.Unmarshal(handleClientResponse(t, resp, http.StatusOK), &respNearestDrugs))
		require.Empty(t, respNearestDrugs)

		// создадим несколько расписаний приёма таблеток
		const (
			drugName1 = "a"
			drugName2 = "b"
		)

		userID := uuid.NewString()
		req := url.Values{}
		req.Add(RequestFieldDrugName, drugName1)
		req.Add(RequestFieldUserID, userID)
		req.Add(RequestFieldPeriod, "47m")
		req.Add(RequestFieldTTL, fixedTime.Add(3*time.Hour).Format(time.RFC3339))
		resp, err = cl.PostForm(fmt.Sprintf("%s/schedule", fullAddr), req)
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusOK)

		req = url.Values{}
		req.Add(RequestFieldDrugName, drugName2)
		req.Add(RequestFieldUserID, userID)
		req.Add(RequestFieldPeriod, "131m")
		req.Add(RequestFieldTTL, fixedTime.Add(24*time.Hour).Format(time.RFC3339))
		resp, err = cl.PostForm(fmt.Sprintf("%s/schedule", fullAddr), req)
		require.NoError(t, err)
		handleClientResponse(t, resp, http.StatusOK)

		resp, err = cl.Get(fmt.Sprintf("%s/next_takings?%s=%s", fullAddr, RequestFieldUserID, userID))
		require.NoError(t, err)
		respNearestDrugs = make([]ResponseNearestDrug, 0)
		require.NoError(t, json.Unmarshal(handleClientResponse(t, resp, http.StatusOK), &respNearestDrugs))

		amountRowsForDrug1 := 0
		amountRowsForDrug2 := 0
		for _, v := range respNearestDrugs {
			switch v.DrugName {
			case drugName1:
				amountRowsForDrug1++
			case drugName2:
				amountRowsForDrug2++
			}
		}

		require.EqualValues(t, 3, amountRowsForDrug1)
		require.EqualValues(t, 5, amountRowsForDrug2)
	})
}

func handleClientResponse(t *testing.T, resp *http.Response, expectStatus int) []byte {
	t.Helper()

	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NoError(t, resp.Body.Close())

	assert.Equal(t, expectStatus, resp.StatusCode)

	return bodyBytes
}

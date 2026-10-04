package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/services"
)

func (e *Env) PATCH(path, token string, body interface{}) *Response {
	return e.do(http.MethodPatch, path, token, body)
}

// growthChild gives the account's child a known sex and a birth date that
// makes them 3 years 2 months old today, and returns its id.
func growthChild(t *testing.T, env *Env, a *Account, gender string) (uuid.UUID, time.Time) {
	t.Helper()
	var raw string
	require.NoError(t, env.DB.Raw("SELECT id::text FROM children WHERE parent_id = ? LIMIT 1", a.ID).Scan(&raw).Error)
	id := uuid.MustParse(raw)
	birth := today().AddDate(-3, -2, 0)
	require.NoError(t, env.DB.Model(&models.Children{}).Where("id = ?", id).
		Updates(map[string]interface{}{"gender": gender, "date_of_birth": birth}).Error)
	return id, birth
}

func today() time.Time {
	n := time.Now().In(time.FixedZone("WIB", 7*3600))
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func day(t time.Time) string { return t.Format("2006-01-02") }

func growthPath(child uuid.UUID, rest string) string {
	return fmt.Sprintf("/children/%s/growth%s", child, rest)
}

func TestGrowth_CreateClassifiesOnTheServer(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")

	res := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
		"client_id": uuid.NewString(), "measured_on": day(today()),
		"height_cm": 94.8, "weight_kg": 13.9, "position": "standing",
		// A category sent by the client is ignored.
		"hfa_category": "tall",
	})
	require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	d := res.Data()
	assert.Equal(t, "3 tahun 2 bulan", d["age_label"])
	assert.Equal(t, "normal", d["hfa"].(map[string]interface{})["category"])
	assert.Equal(t, "normal", d["wfa"].(map[string]interface{})["category"])
	assert.Equal(t, false, d["flagged"])
}

func TestGrowth_Validation(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, birth := growthChild(t, env, a, "Perempuan")
	path := growthPath(child, "/measurements")

	cases := map[string]struct {
		body map[string]interface{}
		code string
	}{
		"future date":   {map[string]interface{}{"measured_on": day(today().AddDate(0, 0, 1)), "weight_kg": 13, "position": "standing"}, "DATE_OUT_OF_RANGE"},
		"before birth":  {map[string]interface{}{"measured_on": day(birth.AddDate(0, 0, -1)), "weight_kg": 3, "position": "recumbent"}, "DATE_OUT_OF_RANGE"},
		"no values":     {map[string]interface{}{"measured_on": day(today()), "position": "standing"}, "VALUE_REQUIRED"},
		"bad position":  {map[string]interface{}{"measured_on": day(today()), "weight_kg": 13, "position": "sitting"}, "INVALID_POSITION"},
		"outlier":       {map[string]interface{}{"measured_on": day(today()), "height_cm": 9.5, "position": "standing"}, "OUTLIER_NEEDS_CONFIRM"},
		"negative":      {map[string]interface{}{"measured_on": day(today()), "weight_kg": -1, "position": "standing"}, "INVALID_VALUE"},
		"too large":     {map[string]interface{}{"measured_on": day(today()), "weight_kg": 1000, "position": "standing"}, "INVALID_VALUE"},
		"malformed day": {map[string]interface{}{"measured_on": "12/09/2026", "weight_kg": 13, "position": "standing"}, "DATE_OUT_OF_RANGE"},
	}
	for name, c := range cases {
		res := env.POST(path, a.Token, c.body)
		assert.Equal(t, http.StatusUnprocessableEntity, res.Code, name)
		assert.Equal(t, c.code, res.JSON()["code"], name)
	}

	var count int64
	env.DB.Model(&models.GrowthMeasurement{}).Where("child_id = ?", child).Count(&count)
	assert.Zero(t, count, "nothing invalid may be stored")
}

func TestGrowth_OutlierNeedsConfirmation(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")
	body := map[string]interface{}{"measured_on": day(today()), "height_cm": 9.5, "position": "standing"}

	res := env.POST(growthPath(child, "/measurements"), a.Token, body)
	require.Equal(t, http.StatusUnprocessableEntity, res.Code)
	assert.Equal(t, []interface{}{"hfa"}, res.JSON()["indicators"])

	body["confirm_outlier"] = true
	res = env.POST(growthPath(child, "/measurements"), a.Token, body)
	require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	assert.Equal(t, true, res.Data()["flagged"])
}

func TestGrowth_RetriedCreateIsIdempotent(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")
	body := map[string]interface{}{"client_id": uuid.NewString(), "measured_on": day(today()), "weight_kg": 13.9, "position": "standing"}

	first := env.POST(growthPath(child, "/measurements"), a.Token, body)
	require.Equal(t, http.StatusCreated, first.Code, string(first.Body))
	second := env.POST(growthPath(child, "/measurements"), a.Token, body)
	require.Equal(t, http.StatusOK, second.Code, string(second.Body))
	assert.Equal(t, first.Data()["id"], second.Data()["id"])

	// Two different measurements on the same date are both kept.
	body["client_id"] = uuid.NewString()
	require.Equal(t, http.StatusCreated, env.POST(growthPath(child, "/measurements"), a.Token, body).Code)

	var count int64
	env.DB.Model(&models.GrowthMeasurement{}).Where("child_id = ?", child).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestGrowth_SummaryLatestValuesAndDeltas(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")
	post := func(on time.Time, h, w interface{}) {
		res := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
			"measured_on": day(on), "height_cm": h, "weight_kg": w, "position": "standing",
		})
		require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	}

	post(today().AddDate(0, -3, 0), 92.6, 13.3)
	one := env.GET(growthPath(child, ""), a.Token)
	require.Equal(t, http.StatusOK, one.Code, string(one.Body))
	assert.NotContains(t, one.Data()["latest_height"].(map[string]interface{}), "delta_from_previous",
		"a single measurement has no delta")

	post(today(), 94.8, nil) // newest has no weight: latest weight is the older one
	res := env.GET(growthPath(child, ""), a.Token)
	require.Equal(t, http.StatusOK, res.Code)
	d := res.Data()
	assert.Equal(t, true, d["profile_complete"])
	child0 := d["child"].(map[string]interface{})
	assert.Equal(t, "male", child0["sex"])
	assert.Equal(t, "3 tahun 2 bulan", child0["age_label"])
	h := d["latest_height"].(map[string]interface{})
	assert.Equal(t, 94.8, h["value"])
	assert.InDelta(t, 2.2, h["delta_from_previous"], 1e-9)
	w := d["latest_weight"].(map[string]interface{})
	assert.Equal(t, 13.3, w["value"])
	ms := d["measurements"].([]interface{})
	require.Len(t, ms, 2)
	assert.Equal(t, day(today()), ms[0].(map[string]interface{})["measured_on"], "newest first")
}

func TestGrowth_UnrecognizedGenderIsAnIncompleteProfile(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "")

	res := env.GET(growthPath(child, ""), a.Token)
	require.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, false, res.Data()["profile_complete"])

	create := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
		"measured_on": day(today()), "weight_kg": 13, "position": "standing",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, create.Code)
	assert.Equal(t, "PROFILE_INCOMPLETE", create.JSON()["code"])
}

func TestGrowth_EditRecomputes(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")
	created := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
		"measured_on": day(today()), "height_cm": 9.5, "position": "standing", "confirm_outlier": true,
	})
	require.Equal(t, http.StatusCreated, created.Code)
	id := created.Data()["id"].(string)

	res := env.PATCH(growthPath(child, "/measurements/"+id), a.Token, map[string]interface{}{"height_cm": 95.0})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	assert.Equal(t, false, res.Data()["flagged"])
	assert.Equal(t, "normal", res.Data()["hfa"].(map[string]interface{})["category"])

	// Clearing the only value is refused.
	clear := env.PATCH(growthPath(child, "/measurements/"+id), a.Token, map[string]interface{}{"height_cm": nil})
	assert.Equal(t, http.StatusUnprocessableEntity, clear.Code)
	assert.Equal(t, "VALUE_REQUIRED", clear.JSON()["code"])

	// Adding a weight and then clearing the height works.
	both := env.PATCH(growthPath(child, "/measurements/"+id), a.Token, map[string]interface{}{"weight_kg": 13.9, "height_cm": nil})
	require.Equal(t, http.StatusOK, both.Code, string(both.Body))
	assert.Nil(t, both.Data()["height_cm"])
	assert.Nil(t, both.Data()["hfa"])
}

func TestGrowth_DeleteRestoreAndPurge(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")
	created := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
		"measured_on": day(today()), "weight_kg": 13.9, "position": "standing",
	})
	require.Equal(t, http.StatusCreated, created.Code)
	id := created.Data()["id"].(string)

	require.Equal(t, http.StatusNoContent, env.DELETE(growthPath(child, "/measurements/"+id), a.Token).Code)
	assert.Empty(t, env.GET(growthPath(child, ""), a.Token).Data()["measurements"])
	assert.Equal(t, http.StatusNotFound, env.PATCH(growthPath(child, "/measurements/"+id), a.Token, map[string]interface{}{"weight_kg": 14}).Code,
		"a deleted measurement cannot be edited")

	restored := env.POST(growthPath(child, "/measurements/"+id+"/restore"), a.Token, nil)
	require.Equal(t, http.StatusOK, restored.Code, string(restored.Body))
	assert.Len(t, env.GET(growthPath(child, ""), a.Token).Data()["measurements"], 1)

	// Purge removes rows soft-deleted more than 30 days ago, and only those.
	require.Equal(t, http.StatusNoContent, env.DELETE(growthPath(child, "/measurements/"+id), a.Token).Code)
	recent := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
		"measured_on": day(today()), "weight_kg": 14, "position": "standing",
	})
	recentID := recent.Data()["id"].(string)
	require.Equal(t, http.StatusNoContent, env.DELETE(growthPath(child, "/measurements/"+recentID), a.Token).Code)
	require.NoError(t, env.DB.Model(&models.GrowthMeasurement{}).Where("id = ?", id).
		Update("deleted_at", time.Now().Add(-31*24*time.Hour)).Error)

	n, err := services.NewGrowthService(env.DB).PurgeDeleted()
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	assert.Equal(t, http.StatusNotFound, env.POST(growthPath(child, "/measurements/"+id+"/restore"), a.Token, nil).Code)
	assert.Equal(t, http.StatusOK, env.POST(growthPath(child, "/measurements/"+recentID+"/restore"), a.Token, nil).Code)
}

func TestGrowth_ProfileChangeRecomputesEveryMeasurement(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, birth := growthChild(t, env, a, "Laki-Laki")
	created := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
		"measured_on": day(today()), "height_cm": 94.8, "weight_kg": 13.9, "position": "standing",
	})
	require.Equal(t, http.StatusCreated, created.Code)
	ageBefore := created.Data()["age_days"].(float64)
	wfaBefore := created.Data()["wfa"].(map[string]interface{})["z"].(float64)

	var parent models.Parent
	require.NoError(t, env.DB.First(&parent, "id = ?", a.ID).Error)
	newBirth := birth.AddDate(0, -6, 0)
	res := env.PUT("/user", a.Token, map[string]interface{}{
		"id": a.ID, "name": parent.Name, "phone_number": parent.PhoneNumber, "email_address": parent.EmailAddress,
		"address": parent.Address, "city": parent.City,
		"child": []map[string]string{{
			"id": child.String(), "name": "Budi", "gender": "Perempuan",
			"date_of_birth": newBirth.Format(time.RFC3339),
		}},
	})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))

	var m models.GrowthMeasurement
	require.NoError(t, env.DB.First(&m, "child_id = ?", child).Error)
	assert.Equal(t, int(ageBefore)+int(birth.Sub(newBirth).Hours()/24), m.AgeDays)
	require.NotNil(t, m.WfaZ)
	assert.NotEqual(t, wfaBefore, *m.WfaZ, "reclassified against the older age and the girls' table")
}

func TestGrowth_AccountDeletionRemovesMeasurements(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")
	require.Equal(t, http.StatusCreated, env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
		"measured_on": day(today()), "weight_kg": 13.9, "position": "standing",
	}).Code)

	require.Equal(t, http.StatusOK, env.DELETE("/user/me", a.Token).Code)

	var count int64
	env.DB.Model(&models.GrowthMeasurement{}).Where("child_id = ?", child).Count(&count)
	assert.Zero(t, count)
}

func TestGrowth_OldRoutesAreGone(t *testing.T) {
	env := NewAPIEnv(t)
	a := env.Register(t)
	assert.Equal(t, http.StatusNotFound, env.GET("/growth?child_id="+uuid.NewString(), a.Token).Code)
}

func TestGrowthMetrics_AreAggregateOnly(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)

	empty := env.GET("/admin/analytics/growth?days=30", admin)
	require.Equal(t, http.StatusOK, empty.Code, string(empty.Body))
	assert.Equal(t, float64(0), empty.Data()["created_measurements"])

	a := env.Register(t)
	child, _ := growthChild(t, env, a, "Laki-Laki")
	post := func(h float64, confirm bool) string {
		res := env.POST(growthPath(child, "/measurements"), a.Token, map[string]interface{}{
			"measured_on": day(today()), "height_cm": h, "position": "standing", "confirm_outlier": confirm,
		})
		require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
		return res.Data()["id"].(string)
	}
	post(94.8, false)
	post(88.0, false) // stunted; newest, so it is the child's latest category
	flagged := post(9.5, true)
	require.Equal(t, http.StatusNoContent, env.DELETE(growthPath(child, "/measurements/"+flagged), a.Token).Code)

	env.Redis.FlushAll() // the endpoint caches for 10 minutes
	res := env.GET("/admin/analytics/growth?days=30", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	d := res.Data()
	assert.Equal(t, float64(1), d["activated_parents"])
	assert.Equal(t, float64(3), d["created_measurements"])
	assert.InDelta(t, 100.0/3, d["corrections_per_100"], 0.01)
	assert.InDelta(t, 1.0/3, d["outlier_confirmed_share"], 0.001)
	assert.Equal(t, map[string]interface{}{"stunted": float64(1)}, d["category_distribution"].(map[string]interface{})["hfa"])
	assert.NotContains(t, string(res.Body), child.String())
	assert.NotContains(t, string(res.Body), "Budi")

	assert.Equal(t, http.StatusForbidden, env.GET("/admin/analytics/growth", a.Token).Code)
}

package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/angkagen"
	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func uploaded(t *testing.T, res *Response) string {
	t.Helper()
	require.Contains(t, []int{http.StatusCreated, http.StatusOK}, res.Code, string(res.Body))
	return res.Data()["id"].(string)
}

// angkaObject creates, fills and publishes an object; returns its id.
func angkaObject(t *testing.T, env *Env, admin, name string) string {
	t.Helper()
	id := newAngkaObject(t, env, admin, name)
	img := uploaded(t, env.upload(admin, "image", name+".png", assetFixture(t, "transparent.png")))
	audio := uploaded(t, env.upload(admin, "audio", name+".mp3", mp3(60)))
	res := saveObject(env, admin, id, map[string]interface{}{
		"name": name, "question_text": "Ada berapa " + name + "?",
		"image_asset_id": img, "question_audio_id": audio,
	})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	pub := env.POST("/admin/angka/objects/"+id+"/publish", admin, nil)
	require.Equal(t, http.StatusNoContent, pub.Code, string(pub.Body))
	return id
}

func newAngkaObject(t *testing.T, env *Env, admin, name string) string {
	t.Helper()
	res := env.POST("/admin/angka/objects", admin, map[string]interface{}{"name": name})
	require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	return res.Data()["id"].(string)
}

func objectRev(t *testing.T, env *Env, admin, id string) int {
	t.Helper()
	res := env.GET("/admin/angka/objects/"+id+"/draft", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	return int(res.Data()["draft_rev"].(float64))
}

func saveObject(env *Env, admin, id string, draft map[string]interface{}) *Response {
	return env.PUT("/admin/angka/objects/"+id+"/draft", admin,
		map[string]interface{}{"draft": draft, "draft_rev": objectRev(env.T, env, admin, id)})
}

func numberRow(t *testing.T, env *Env, admin string, value int) map[string]interface{} {
	t.Helper()
	res := env.GET("/admin/angka/numbers", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	for _, r := range res.JSON()["data"].([]interface{}) {
		row := r.(map[string]interface{})
		if int(row["value"].(float64)) == value {
			return row
		}
	}
	t.Fatalf("number %d not found", value)
	return nil
}

// publishNumbers gives numbers 1..max an audio and object and publishes them.
func publishNumbers(t *testing.T, env *Env, admin string, max int, object string) {
	t.Helper()
	audio := uploaded(t, env.upload(admin, "audio", "angka.mp3", mp3(30)))
	for v := 1; v <= max; v++ {
		row := numberRow(t, env, admin, v)
		draft := row["draft"].(map[string]interface{})
		draft["audio_asset_id"] = audio
		draft["object_id"] = object
		res := env.PUT(fmt.Sprintf("/admin/angka/numbers/%d/draft", v), admin,
			map[string]interface{}{"draft": draft, "draft_rev": row["draft_rev"]})
		require.Equal(t, http.StatusOK, res.Code, string(res.Body))
		pub := env.POST(fmt.Sprintf("/admin/angka/numbers/%d/publish", v), admin, nil)
		require.Equal(t, http.StatusNoContent, pub.Code, string(pub.Body))
	}
}

func levelDraft(t *testing.T, env *Env, admin, id string) (map[string]interface{}, int) {
	t.Helper()
	res := env.GET("/admin/angka/levels/"+id+"/draft", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	d := res.Data()
	return d["draft"].(map[string]interface{}), int(d["draft_rev"].(float64))
}

// editLevel loads a level's draft, applies edit and saves it.
func editLevel(t *testing.T, env *Env, admin, id string, edit func(d map[string]interface{})) {
	t.Helper()
	d, rev := levelDraft(t, env, admin, id)
	edit(d)
	res := env.PUT("/admin/angka/levels/"+id+"/draft", admin, map[string]interface{}{"draft": d, "draft_rev": rev})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
}

// angkaLevel creates a level (requiring the previous one), sets its range
// and objects, and publishes it unless publish is false.
func angkaLevel(t *testing.T, env *Env, admin, name string, min, max int, objects []string, publish bool) string {
	t.Helper()
	res := env.POST("/admin/angka/levels", admin, nil)
	require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	id := res.Data()["id"].(string)
	editLevel(t, env, admin, id, func(d map[string]interface{}) {
		d["name"] = name
		d["range"] = map[string]interface{}{"min": min, "max": max}
		d["object_ids"] = objects
	})
	if publish {
		pub := env.POST("/admin/angka/levels/"+id+"/publish", admin, nil)
		require.Equal(t, http.StatusOK, pub.Code, string(pub.Body))
	}
	return id
}

// angkaWorld publishes three objects, numbers 1–10, a free Level 1 (1–5)
// and Level 2 (1–10) requiring it.
type angkaWorld struct {
	admin            string
	apel, bola, ikan string
	l1, l2           string
}

func newAngkaWorld(t *testing.T, env *Env) angkaWorld {
	t.Helper()
	_, admin := adminSession(t, env)
	w := angkaWorld{admin: admin}
	w.apel = angkaObject(t, env, admin, "apel")
	w.bola = angkaObject(t, env, admin, "bola")
	w.ikan = angkaObject(t, env, admin, "ikan")
	publishNumbers(t, env, admin, 10, w.apel)
	w.l1 = angkaLevel(t, env, admin, "Hitung 1 sampai 5", 1, 5, []string{w.apel, w.bola}, true)
	require.Equal(t, http.StatusNoContent, env.POST("/admin/angka/levels/"+w.l1+"/set-free", admin, nil).Code)
	w.l2 = angkaLevel(t, env, admin, "Hitung 1 sampai 10", 1, 10, []string{w.apel, w.bola, w.ikan}, true)
	return w
}

func fieldCodes(res *Response) map[string]string {
	out := map[string]string{}
	fields, _ := res.JSON()["fields"].([]interface{})
	for _, f := range fields {
		m := f.(map[string]interface{})
		out[m["path"].(string)] = m["code"].(string)
	}
	return out
}

// session is a started session as the app sees it.
type session struct {
	ID        string
	Questions []angkagen.Question
	Answers   []interface{}
}

func startSession(t *testing.T, env *Env, token, child, level string, restart bool) (*Response, *session) {
	t.Helper()
	res := env.POST("/children/"+child+"/angka/sessions", token, map[string]interface{}{"level_id": level, "restart": restart})
	if res.Code != http.StatusOK {
		return res, nil
	}
	d := res.Data()
	c := d["content"].(map[string]interface{})
	rng := c["range"].(map[string]interface{})
	var objects []string
	for _, o := range d["object_ids"].([]interface{}) {
		objects = append(objects, o.(string))
	}
	qs := angkagen.Generate(angkagen.Params{
		Min: int(rng["min"].(float64)), Max: int(rng["max"].(float64)),
		Count: int(c["question_count"].(float64)), ObjectIDs: objects,
		Layout: c["layout"].(string), Seed: uint32(d["seed"].(float64)),
	})
	return res, &session{ID: d["session_id"].(string), Questions: qs, Answers: d["answers"].([]interface{})}
}

func try(env *Env, token, child, sess string, q, n, answer int) *Response {
	return env.PATCH("/children/"+child+"/angka/sessions/"+sess, token,
		map[string]interface{}{"q": q, "try": n, "answer": answer})
}

// playAll answers every question: correct on the first try for the first
// firstRight questions, otherwise wrong then right.
func playAll(t *testing.T, env *Env, token, child string, s *session, firstRight int) {
	t.Helper()
	for i, q := range s.Questions {
		if i >= firstRight {
			require.Equal(t, http.StatusOK, try(env, token, child, s.ID, i+1, 1, q.Count+1).Code)
			res := try(env, token, child, s.ID, i+1, 2, q.Count)
			require.Equal(t, http.StatusOK, res.Code, string(res.Body))
			continue
		}
		res := try(env, token, child, s.ID, i+1, 1, q.Count)
		require.Equal(t, http.StatusOK, res.Code, string(res.Body))
		assert.Equal(t, true, res.Data()["correct"])
	}
}

// ─── content admin ───────────────────────────────────────────────────────────

func TestAngka_SeededNumbersAndFlag(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	res := env.GET("/admin/angka/numbers", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	rows := res.JSON()["data"].([]interface{})
	require.Len(t, rows, 20)
	for _, r := range rows {
		row := r.(map[string]interface{})
		v := int(row["value"].(float64))
		assert.Equal(t, v <= 5, row["is_free"], "number %d", v)
		assert.Equal(t, "draft", row["status"])
	}
	assert.Equal(t, "tujuh", numberRow(t, env, admin, 7)["name"])
	assert.Equal(t, "dua puluh", numberRow(t, env, admin, 20)["name"])

	flags := env.GET("/app/feature-flags", "")
	raw, _ := json.Marshal(flags.JSON())
	assert.Contains(t, string(raw), `"belajar_angka":false`)
}

func TestAngka_ObjectPublishValidation(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	id := newAngkaObject(t, env, admin, "")

	res := env.POST("/admin/angka/objects/"+id+"/publish", admin, nil)
	require.Equal(t, http.StatusUnprocessableEntity, res.Code, string(res.Body))
	codes := fieldCodes(res)
	assert.Equal(t, "REQUIRED", codes["name"])
	assert.Equal(t, "REQUIRED", codes["image_asset_id"])
	assert.Equal(t, "REQUIRED", codes["question_audio_id"])

	opaque := uploaded(t, env.upload(admin, "image", "apel.png", assetFixture(t, "square.png")))
	long := uploaded(t, env.upload(admin, "audio", "long.mp3", mp3(250))) // ≈ 6.5 s
	require.Equal(t, http.StatusOK, saveObject(env, admin, id, map[string]interface{}{
		"name": "apel", "question_text": "Ada berapa apel?", "image_asset_id": opaque, "question_audio_id": long,
	}).Code)
	res = env.POST("/admin/angka/objects/"+id+"/publish", admin, nil)
	require.Equal(t, http.StatusUnprocessableEntity, res.Code)
	codes = fieldCodes(res)
	assert.Equal(t, "NOT_TRANSPARENT", codes["image_asset_id"])
	assert.Equal(t, "TOO_LONG", codes["question_audio_id"])

	// External URLs can't be measured and pass, but must be https.
	require.Equal(t, http.StatusOK, saveObject(env, admin, id, map[string]interface{}{
		"name": "apel", "question_text": "Ada berapa apel?",
		"image_url": "http://media.haloarunika.com/apel.png", "question_audio_url": "https://media.haloarunika.com/apel.mp3",
	}).Code)
	res = env.POST("/admin/angka/objects/"+id+"/publish", admin, nil)
	assert.Equal(t, "INVALID_URL", fieldCodes(res)["image_url"])
	require.Equal(t, http.StatusOK, saveObject(env, admin, id, map[string]interface{}{
		"name": "apel", "question_text": "Ada berapa apel?",
		"image_url": "https://media.haloarunika.com/apel.png", "question_audio_url": "https://media.haloarunika.com/apel.mp3",
	}).Code)
	assert.Equal(t, http.StatusNoContent, env.POST("/admin/angka/objects/"+id+"/publish", admin, nil).Code)

	list := env.GET("/admin/angka/objects", admin)
	row := list.JSON()["data"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "published", row["status"])
	assert.Equal(t, false, row["has_unpublished_changes"])
	assert.Equal(t, "https://media.haloarunika.com/apel.png", row["image_url"])
}

func TestAngka_EditorRolesAndConflict(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	obj := angkaObject(t, env, admin, "apel")
	_, editor := editorSession(t, env)

	// Editors draft and create; publishing actions need the publisher role.
	created := env.POST("/admin/angka/levels", editor, nil)
	require.Equal(t, http.StatusCreated, created.Code, string(created.Body))
	level := created.Data()["id"].(string)
	for _, path := range []string{
		"/admin/angka/objects/" + obj + "/publish", "/admin/angka/objects/" + obj + "/hide",
		"/admin/angka/numbers/1/publish", "/admin/angka/numbers/1/set-free",
		"/admin/angka/levels/" + level + "/publish", "/admin/angka/levels/" + level + "/set-free",
		"/admin/angka/levels/" + level + "/hide",
	} {
		assert.Equal(t, http.StatusForbidden, env.POST(path, editor, nil).Code, path)
	}
	assert.Equal(t, http.StatusForbidden, env.DELETE("/admin/angka/levels/"+level, editor).Code)
	assert.Equal(t, http.StatusForbidden, env.PUT("/admin/angka/levels/order", editor, map[string]interface{}{"ids": []string{level}}).Code)

	d, rev := levelDraft(t, env, editor, level)
	first := env.PUT("/admin/angka/levels/"+level+"/draft", editor, map[string]interface{}{"draft": d, "draft_rev": rev})
	require.Equal(t, http.StatusOK, first.Code)
	second := env.PUT("/admin/angka/levels/"+level+"/draft", admin, map[string]interface{}{"draft": d, "draft_rev": rev})
	assert.Equal(t, http.StatusConflict, second.Code)
	assert.Equal(t, "DRAFT_CONFLICT", second.JSON()["code"])

	var n int64
	env.DB.Model(&models.AdminAuditLog{}).Where("entity_type = ? AND entity_id = ?", "angka_level", level).Count(&n)
	assert.NotZero(t, n, "level writes are audited")
}

func TestAngka_LevelValidation(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	apel := angkaObject(t, env, admin, "apel")
	publishNumbers(t, env, admin, 10, apel)
	l1 := angkaLevel(t, env, admin, "Hitung 1 sampai 5", 1, 5, []string{apel}, true)
	l2 := angkaLevel(t, env, admin, "Hitung 1 sampai 10", 1, 10, []string{apel}, false)

	publish := func(id string) map[string]string {
		res := env.POST("/admin/angka/levels/"+id+"/publish", admin, nil)
		require.Equal(t, http.StatusUnprocessableEntity, res.Code, string(res.Body))
		return fieldCodes(res)
	}

	editLevel(t, env, admin, l2, func(d map[string]interface{}) {
		d["range"] = map[string]interface{}{"min": 5, "max": 25}
		d["question_count"] = 4
		d["stars"] = map[string]interface{}{"three": 7, "two": 9}
		d["object_ids"] = []string{}
		d["feedback"] = map[string]interface{}{"success": "", "retry": "Hampir benar!", "hint": "x"}
	})
	codes := publish(l2)
	assert.Equal(t, "OUT_OF_RANGE", codes["range.max"])
	assert.Equal(t, "OUT_OF_RANGE", codes["question_count"])
	assert.Equal(t, "GREATER_THAN_THREE", codes["stars.two"])
	assert.Equal(t, "REQUIRED", codes["object_ids"])
	assert.Equal(t, "REQUIRED", codes["feedback.success"])

	// Range 11–20 needs count audio up to 20; numbers 11–20 aren't published.
	editLevel(t, env, admin, l2, func(d map[string]interface{}) {
		d["range"] = map[string]interface{}{"min": 11, "max": 20}
		d["question_count"] = 10
		d["stars"] = map[string]interface{}{"three": 9, "two": 7}
		d["object_ids"] = []string{apel}
		d["feedback"] = map[string]interface{}{"success": "Hebat!", "retry": "Hampir!", "hint": "Hitung {benda}"}
	})
	missing := publish(l2)
	assert.Equal(t, "NUMBER_AUDIO_MISSING", missing["range.max"])
	assert.Equal(t, "NUMBER_AUDIO_MISSING", missing["numbers.11"])
	assert.Equal(t, "NUMBER_AUDIO_MISSING", missing["numbers.20"])
	assert.NotContains(t, missing, "numbers.10")

	// A prerequisite cycle: L1 requires L2 while L2 requires L1.
	editLevel(t, env, admin, l2, func(d map[string]interface{}) {
		d["range"] = map[string]interface{}{"min": 1, "max": 10}
	})
	require.Equal(t, http.StatusOK, env.POST("/admin/angka/levels/"+l2+"/publish", admin, nil).Code)
	editLevel(t, env, admin, l1, func(d map[string]interface{}) { d["prerequisite_id"] = l2 })
	assert.Equal(t, "CYCLE", publish(l1)["prerequisite_id"])
	editLevel(t, env, admin, l1, func(d map[string]interface{}) { d["prerequisite_id"] = l1 })
	assert.Equal(t, "NOT_PUBLISHED", publish(l1)["prerequisite_id"])
}

func TestAngka_LevelVersionsFreeOrderDelete(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	admin := w.admin

	// Versions and rollback.
	editLevel(t, env, admin, w.l2, func(d map[string]interface{}) { d["name"] = "Hitung sampai 10" })
	require.Equal(t, http.StatusOK, env.POST("/admin/angka/levels/"+w.l2+"/publish", admin, nil).Code)
	rb := env.POST("/admin/angka/levels/"+w.l2+"/rollback/1", admin, nil)
	require.Equal(t, http.StatusOK, rb.Code, string(rb.Body))
	assert.EqualValues(t, 3, rb.Data()["version"])
	d, _ := levelDraft(t, env, admin, w.l2)
	assert.Equal(t, "Hitung 1 sampai 10", d["name"])
	vs := env.GET("/admin/angka/levels/"+w.l2+"/versions", admin)
	require.Len(t, vs.JSON()["data"], 3)

	// One free level at a time.
	require.Equal(t, http.StatusNoContent, env.POST("/admin/angka/levels/"+w.l2+"/set-free", admin, nil).Code)
	rows := env.GET("/admin/angka/levels", admin).JSON()["data"].([]interface{})
	free := map[string]bool{}
	for _, r := range rows {
		row := r.(map[string]interface{})
		free[row["id"].(string)] = row["is_free"].(bool)
		if row["id"] == w.l2 {
			assert.EqualValues(t, 1, row["prerequisite_sort_order"])
			assert.Equal(t, []interface{}{"apel", "bola", "ikan"}, row["object_names"])
		}
	}
	assert.Equal(t, map[string]bool{w.l1: false, w.l2: true}, free)
	require.Equal(t, http.StatusNoContent, env.POST("/admin/angka/levels/"+w.l2+"/unset-free", admin, nil).Code)

	// Reorder needs every level once.
	assert.Equal(t, http.StatusUnprocessableEntity, env.PUT("/admin/angka/levels/order", admin,
		map[string]interface{}{"ids": []string{w.l2}}).Code)
	require.Equal(t, http.StatusNoContent, env.PUT("/admin/angka/levels/order", admin,
		map[string]interface{}{"ids": []string{w.l2, w.l1}}).Code)
	first := env.GET("/admin/angka/levels", admin).JSON()["data"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, w.l2, first["id"])

	// A published level, or one another level requires, can't be deleted.
	del := env.DELETE("/admin/angka/levels/"+w.l1, admin)
	assert.Equal(t, http.StatusConflict, del.Code)
	assert.Equal(t, "LEVEL_IN_USE", del.JSON()["code"])
	draft := env.POST("/admin/angka/levels", admin, nil).Data()["id"].(string)
	assert.Equal(t, http.StatusNoContent, env.DELETE("/admin/angka/levels/"+draft, admin).Code)
}

func TestAngka_ObjectHideAndDelete(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	apel := angkaObject(t, env, admin, "apel")
	bola := angkaObject(t, env, admin, "bola")
	spare := angkaObject(t, env, admin, "spare")
	publishNumbers(t, env, admin, 5, apel)
	angkaLevel(t, env, admin, "Hitung 1 sampai 5", 1, 5, []string{apel, bola}, true)

	require.Equal(t, http.StatusNoContent, env.POST("/admin/angka/objects/"+bola+"/hide", admin, nil).Code)
	last := env.POST("/admin/angka/objects/"+apel+"/hide", admin, nil)
	assert.Equal(t, http.StatusConflict, last.Code)
	assert.Equal(t, "OBJECT_LAST_IN_LEVEL", last.JSON()["code"])

	inUse := env.DELETE("/admin/angka/objects/"+bola, admin)
	assert.Equal(t, http.StatusConflict, inUse.Code)
	assert.Equal(t, "OBJECT_IN_USE", inUse.JSON()["code"])
	assert.Equal(t, http.StatusNoContent, env.DELETE("/admin/angka/objects/"+spare, admin).Code)

	rows := env.GET("/admin/angka/objects", admin).JSON()["data"].([]interface{})
	require.Len(t, rows, 2)
	for _, r := range rows {
		row := r.(map[string]interface{})
		assert.EqualValues(t, 1, row["used_in"], row["name"])
	}
}

func TestAngka_Preview(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	d, _ := levelDraft(t, env, w.admin, w.l2)
	preview := func(seed int) []interface{} {
		res := env.POST("/admin/angka/levels/"+w.l2+"/preview", w.admin, map[string]interface{}{"draft": d, "seed": seed})
		require.Equal(t, http.StatusOK, res.Code, string(res.Body))
		return res.Data()["questions"].([]interface{})
	}
	a, b := preview(7), preview(7)
	assert.Equal(t, a, b, "same seed, same questions")
	assert.NotEqual(t, a, preview(8))
	require.Len(t, a, 10)
	q := a[0].(map[string]interface{})
	assert.Contains(t, q["object"].(map[string]interface{})["image_url"], "/media/huruf/")

	d["range"] = map[string]interface{}{"min": 1, "max": 30}
	bad := env.POST("/admin/angka/levels/"+w.l2+"/preview", w.admin, map[string]interface{}{"draft": d, "seed": 1})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Code)
}

// ─── app API ──────────────────────────────────────────────────────────────────

func TestAngka_ManifestAndAccess(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	require.Equal(t, http.StatusNoContent, env.POST("/admin/angka/numbers/9/hide", w.admin, nil).Code)
	angkaLevel(t, env, w.admin, "Draft only", 1, 5, []string{w.apel}, false)
	user := env.Register(t)

	res := env.GET("/learn/angka/manifest", user.Token)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	m := res.Data()
	assert.Equal(t, false, m["premium"])
	nums := m["numbers"].([]interface{})
	require.Len(t, nums, 9, "1–10 without the hidden 9")
	for _, n := range nums {
		row := n.(map[string]interface{})
		v := int(row["value"].(float64))
		assert.NotEqual(t, 9, v)
		assert.Equal(t, v > 5, row["locked"], "number %d", v)
	}
	levels := m["levels"].([]interface{})
	require.Len(t, levels, 2, "the draft-only level is excluded")
	assert.Equal(t, false, levels[0].(map[string]interface{})["locked"])
	assert.Equal(t, true, levels[1].(map[string]interface{})["locked"])
	assert.Equal(t, w.l1, levels[1].(map[string]interface{})["prerequisite_id"])

	tag := headerOf(env, "/learn/angka/manifest", user.Token)
	assert.Equal(t, http.StatusNotModified, env.getWithHeader("/learn/angka/manifest", user.Token, "If-None-Match", tag).Code)

	three := env.GET("/learn/angka/numbers/3", user.Token)
	require.Equal(t, http.StatusOK, three.Code, string(three.Body))
	assert.Equal(t, "tiga", three.Data()["name"])
	assert.Len(t, three.Data()["count_audio"], 3)
	assert.Equal(t, "apel", three.Data()["object"].(map[string]interface{})["name"])
	assert.Equal(t, http.StatusPaymentRequired, env.GET("/learn/angka/numbers/7", user.Token).Code)
	assert.Equal(t, http.StatusNotFound, env.GET("/learn/angka/numbers/9", user.Token).Code)

	l1 := env.GET("/learn/angka/levels/"+w.l1, user.Token)
	require.Equal(t, http.StatusOK, l1.Code, string(l1.Body))
	assert.Len(t, l1.Data()["objects"], 2)
	assert.Len(t, l1.Data()["count_audio"], 5)
	locked := env.GET("/learn/angka/levels/"+w.l2, user.Token)
	assert.Equal(t, http.StatusPaymentRequired, locked.Code)
	assert.Equal(t, "PREMIUM_REQUIRED", locked.JSON()["code"])

	expired := env.Register(t)
	fixtures.NewSubscription(t, env.DB, parentOf(t, env, expired), fixtures.ExpiringAt(time.Now().Add(-24*time.Hour)))
	assert.Equal(t, http.StatusPaymentRequired, env.GET("/learn/angka/levels/"+w.l2, expired.Token).Code)

	packOwner := env.Register(t)
	fixtures.NewEntitlement(t, env.DB, parentOf(t, env, packOwner), fixtures.NewProduct(t, env.DB))
	assert.Equal(t, http.StatusPaymentRequired, env.GET("/learn/angka/numbers/7", packOwner.Token).Code)

	fixtures.NewSubscription(t, env.DB, parentOf(t, env, user))
	sub := env.getWithHeader("/learn/angka/manifest", user.Token, "If-None-Match", tag)
	require.Equal(t, http.StatusOK, sub.Code, "subscribing changes the ETag")
	assert.Equal(t, true, sub.Data()["premium"])
	assert.Equal(t, http.StatusOK, env.GET("/learn/angka/numbers/7", user.Token).Code)
	assert.Equal(t, http.StatusOK, env.GET("/learn/angka/levels/"+w.l2, user.Token).Code)
}

// Every premium-guarded route answers 402 for a locked item and lets the
// free one through.
func TestAngka_PremiumGuardOnEveryRoute(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	// Level 2 without a prerequisite, so the guard is the only lock.
	editLevel(t, env, w.admin, w.l2, func(d map[string]interface{}) { d["prerequisite_id"] = nil })
	require.Equal(t, http.StatusOK, env.POST("/admin/angka/levels/"+w.l2+"/publish", w.admin, nil).Code)
	user := env.Register(t)
	child := firstChild(t, env, user)

	routes := []struct {
		name         string
		free, locked func() *Response
	}{
		{"GET number",
			func() *Response { return env.GET("/learn/angka/numbers/2", user.Token) },
			func() *Response { return env.GET("/learn/angka/numbers/8", user.Token) }},
		{"GET level",
			func() *Response { return env.GET("/learn/angka/levels/"+w.l1, user.Token) },
			func() *Response { return env.GET("/learn/angka/levels/"+w.l2, user.Token) }},
		{"POST session",
			func() *Response {
				return env.POST("/children/"+child+"/angka/sessions", user.Token, map[string]interface{}{"level_id": w.l1})
			},
			func() *Response {
				return env.POST("/children/"+child+"/angka/sessions", user.Token, map[string]interface{}{"level_id": w.l2})
			}},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusPaymentRequired, r.locked().Code, r.name+" locked")
		assert.Equal(t, http.StatusOK, r.free().Code, r.name+" free")
	}
}

func TestAngka_SessionPlayAndScoring(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	user := env.Register(t)
	fixtures.NewSubscription(t, env.DB, parentOf(t, env, user))
	child := firstChild(t, env, user)

	// Level 2 needs Level 1 first.
	res, _ := startSession(t, env, user.Token, child, w.l2, false)
	assert.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "LEVEL_LOCKED", res.JSON()["code"])

	res, s := startSession(t, env, user.Token, child, w.l1, false)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	require.Len(t, s.Questions, 10)
	assert.Len(t, res.Data()["objects"], 2)
	assert.Len(t, res.Data()["count_audio"], 5)

	q1 := s.Questions[0].Count
	wrong := try(env, user.Token, child, s.ID, 1, 1, q1+1)
	require.Equal(t, http.StatusOK, wrong.Code, string(wrong.Body))
	assert.Equal(t, false, wrong.Data()["correct"])
	assert.Equal(t, false, wrong.Data()["finished"])
	// Retries of a recorded try are no-ops; anything that doesn't fit is refused.
	assert.Equal(t, http.StatusOK, try(env, user.Token, child, s.ID, 1, 1, q1+1).Code)
	for _, bad := range []struct{ q, n, a int }{
		{1, 1, q1},  // a different answer for a recorded try
		{1, 3, q1},  // a skipped try
		{11, 1, 1},  // no such question
		{2, 1, 100}, // three digits
	} {
		r := try(env, user.Token, child, s.ID, bad.q, bad.n, bad.a)
		assert.Equal(t, http.StatusUnprocessableEntity, r.Code, "%+v", bad)
		assert.Equal(t, "INVALID_TRY", r.JSON()["code"])
	}
	// There's no try limit: the question stays open until it's right.
	for n := 2; n <= 4; n++ {
		r := try(env, user.Token, child, s.ID, 1, n, q1+n)
		require.Equal(t, http.StatusOK, r.Code, string(r.Body))
		assert.Equal(t, false, r.Data()["finished"])
	}
	right := try(env, user.Token, child, s.ID, 1, 5, q1)
	assert.Equal(t, true, right.Data()["finished"])
	assert.Equal(t, http.StatusUnprocessableEntity, try(env, user.Token, child, s.ID, 1, 6, q1).Code, "after a right answer")

	early := env.POST("/children/"+child+"/angka/sessions/"+s.ID+"/complete", user.Token, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, early.Code)
	assert.Equal(t, "SESSION_INCOMPLETE", early.JSON()["code"])

	// Questions 2–10 right first time: 9 first-try correct = 3 stars.
	for i, q := range s.Questions[1:] {
		require.Equal(t, http.StatusOK, try(env, user.Token, child, s.ID, i+2, 1, q.Count).Code)
	}
	done := env.POST("/children/"+child+"/angka/sessions/"+s.ID+"/complete", user.Token, nil)
	require.Equal(t, http.StatusOK, done.Code, string(done.Body))
	assert.EqualValues(t, 3, done.Data()["stars"])
	assert.EqualValues(t, 9, done.Data()["first_correct"])
	assert.Equal(t, []interface{}{w.l2}, done.Data()["unlocked_level_ids"])
	again := env.POST("/children/"+child+"/angka/sessions/"+s.ID+"/complete", user.Token, nil)
	assert.Equal(t, done.Data(), again.Data(), "complete is idempotent")

	// A replay with fewer stars keeps the best, and unlocks nothing new.
	_, replay := startSession(t, env, user.Token, child, w.l1, false)
	require.NotEqual(t, s.ID, replay.ID)
	playAll(t, env, user.Token, child, replay, 0)
	worse := env.POST("/children/"+child+"/angka/sessions/"+replay.ID+"/complete", user.Token, nil)
	assert.EqualValues(t, 1, worse.Data()["stars"])
	assert.EqualValues(t, 3, worse.Data()["best_stars"])
	assert.Empty(t, worse.Data()["unlocked_level_ids"])

	// Level 2 opens now; thresholds 9 / 7 give 2 stars for 7.
	res, s2 := startSession(t, env, user.Token, child, w.l2, false)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	playAll(t, env, user.Token, child, s2, 7)
	l2 := env.POST("/children/"+child+"/angka/sessions/"+s2.ID+"/complete", user.Token, nil)
	assert.EqualValues(t, 2, l2.Data()["stars"])

	prog := env.GET("/children/"+child+"/angka/progress", user.Token)
	require.Equal(t, http.StatusOK, prog.Code)
	best := map[string]float64{}
	for _, r := range prog.JSON()["data"].([]interface{}) {
		row := r.(map[string]interface{})
		best[row["level_id"].(string)] = row["best_stars"].(float64)
		assert.Equal(t, true, row["completed"])
		assert.Nil(t, row["current_session"])
	}
	assert.Equal(t, map[string]float64{w.l1: 3, w.l2: 2}, best)
}

func TestAngka_SessionResumeRestartAndPinning(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	user := env.Register(t)
	child := firstChild(t, env, user)

	_, s := startSession(t, env, user.Token, child, w.l1, false)
	for i, q := range s.Questions[:3] {
		require.Equal(t, http.StatusOK, try(env, user.Token, child, s.ID, i+1, 1, q.Count).Code)
	}
	prog := env.GET("/children/"+child+"/angka/progress", user.Token).JSON()["data"].([]interface{})
	cur := prog[0].(map[string]interface{})["current_session"].(map[string]interface{})
	assert.EqualValues(t, 3, cur["answered"])
	assert.EqualValues(t, 10, cur["question_count"])

	// Lanjut resumes the same session.
	_, again := startSession(t, env, user.Token, child, w.l1, false)
	assert.Equal(t, s.ID, again.ID)
	assert.Len(t, again.Answers, 3)
	assert.Equal(t, s.Questions, again.Questions)

	// Editing and republishing the level doesn't move a session in progress.
	editLevel(t, env, w.admin, w.l1, func(d map[string]interface{}) {
		d["range"] = map[string]interface{}{"min": 2, "max": 3}
	})
	require.Equal(t, http.StatusOK, env.POST("/admin/angka/levels/"+w.l1+"/publish", w.admin, nil).Code)
	res, pinned := startSession(t, env, user.Token, child, w.l1, false)
	assert.EqualValues(t, 1, res.Data()["level_version"])
	assert.Equal(t, s.Questions, pinned.Questions)

	// Hidden objects aren't used by new sessions.
	require.Equal(t, http.StatusNoContent, env.POST("/admin/angka/objects/"+w.bola+"/hide", w.admin, nil).Code)
	res, fresh := startSession(t, env, user.Token, child, w.l1, true)
	require.NotEqual(t, s.ID, fresh.ID)
	assert.EqualValues(t, 2, res.Data()["level_version"])
	assert.Equal(t, []interface{}{w.apel}, res.Data()["object_ids"])
	for _, q := range fresh.Questions {
		assert.True(t, q.Count >= 2 && q.Count <= 3)
	}
	var old models.AngkaSession
	require.NoError(t, env.DB.Where("id = ?", s.ID).First(&old).Error)
	assert.Equal(t, "abandoned", old.Status)

	// A session untouched for 7 days is no longer resumed.
	require.NoError(t, env.DB.Model(&models.AngkaSession{}).Where("id = ?", fresh.ID).
		Update("updated_at", time.Now().Add(-8*24*time.Hour)).Error)
	_, later := startSession(t, env, user.Token, child, w.l1, false)
	assert.NotEqual(t, fresh.ID, later.ID)
}

func TestAngka_ExpiryMidLevel(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	user := env.Register(t)
	fixtures.NewSubscription(t, env.DB, parentOf(t, env, user))
	child := firstChild(t, env, user)
	_, s1 := startSession(t, env, user.Token, child, w.l1, false)
	playAll(t, env, user.Token, child, s1, 10)
	require.Equal(t, http.StatusOK, env.POST("/children/"+child+"/angka/sessions/"+s1.ID+"/complete", user.Token, nil).Code)

	_, s2 := startSession(t, env, user.Token, child, w.l2, false)
	require.NoError(t, env.DB.Exec("UPDATE user_subscriptions SET expires_at = ? WHERE user_id = ?",
		time.Now().Add(-time.Hour), user.ID).Error)

	playAll(t, env, user.Token, child, s2, 10)
	done := env.POST("/children/"+child+"/angka/sessions/"+s2.ID+"/complete", user.Token, nil)
	require.Equal(t, http.StatusOK, done.Code, "the level started before expiry can be finished")
	res, _ := startSession(t, env, user.Token, child, w.l2, true)
	assert.Equal(t, http.StatusPaymentRequired, res.Code)
}

func TestAngka_CrossParentAndDeletion(t *testing.T) {
	env := NewAPIEnv(t)
	w := newAngkaWorld(t, env)
	user := env.Register(t)
	child := firstChild(t, env, user)
	_, s := startSession(t, env, user.Token, child, w.l1, false)
	require.Equal(t, http.StatusOK, try(env, user.Token, child, s.ID, 1, 1, s.Questions[0].Count).Code)

	other := env.Register(t)
	otherChild := firstChild(t, env, other)
	assert.Equal(t, http.StatusNotFound, env.GET("/children/"+child+"/angka/progress", other.Token).Code)
	r, _ := startSession(t, env, other.Token, child, w.l1, false)
	assert.Equal(t, http.StatusNotFound, r.Code)
	assert.Equal(t, http.StatusNotFound, try(env, other.Token, child, s.ID, 2, 1, 1).Code)
	// Their own child can't reach this session either.
	assert.Equal(t, http.StatusNotFound, try(env, other.Token, otherChild, s.ID, 2, 1, 1).Code)
	assert.Equal(t, http.StatusNotFound,
		env.POST("/children/"+otherChild+"/angka/sessions/"+s.ID+"/complete", other.Token, nil).Code)

	res := env.DELETE("/user/me", user.Token)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, res.Code, string(res.Body))
	var sessions, progress int64
	env.DB.Model(&models.AngkaSession{}).Where("child_id = ?", child).Count(&sessions)
	env.DB.Model(&models.AngkaLevelProgress{}).Where("child_id = ?", child).Count(&progress)
	assert.Zero(t, sessions)
	assert.Zero(t, progress)
}

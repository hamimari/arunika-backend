package api_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func editorSession(t *testing.T, env *Env) (uuid.UUID, string) {
	t.Helper()
	id, token := adminSession(t, env)
	require.NoError(t, env.DB.Model(&models.AdminUser{}).Where("id = ?", id).Update("role", "editor").Error)
	return id, token
}

// upload posts a multipart asset.
func (e *Env) upload(token, kind, name string, data []byte) *Response {
	e.T.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	require.NoError(e.T, w.WriteField("kind", kind))
	part, err := w.CreateFormFile("file", name)
	require.NoError(e.T, err)
	_, _ = part.Write(data)
	require.NoError(e.T, w.Close())
	req := httptest.NewRequest(http.MethodPost, "/admin/assets", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	return &Response{t: e.T, Code: rec.Code, Body: rec.Body.Bytes()}
}

func (e *Env) getWithHeader(path, token, header, value string) *Response {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(header, value)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	return &Response{t: e.T, Code: rec.Code, Body: rec.Body.Bytes()}
}

func assetFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../services/testdata/assets/" + name)
	require.NoError(t, err)
	return b
}

// mp3 is n MPEG-1 layer III frames (128 kbps, 44.1 kHz ≈ 26 ms each).
func mp3(n int) []byte {
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		f := make([]byte, 417)
		copy(f, []byte{0xFF, 0xFB, 0x90, 0x00})
		buf.Write(f)
	}
	return buf.Bytes()
}

func letterID(t *testing.T, env *Env, admin, upper string) string {
	t.Helper()
	res := env.GET("/admin/huruf/letters", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	for _, row := range res.JSON()["data"].([]interface{}) {
		r := row.(map[string]interface{})
		if r["upper"] == upper {
			return r["id"].(string)
		}
	}
	t.Fatalf("letter %s not found", upper)
	return ""
}

func draftOf(t *testing.T, env *Env, admin, id string) (map[string]interface{}, int) {
	t.Helper()
	res := env.GET("/admin/huruf/letters/"+id+"/draft", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	d := res.Data()
	return d["draft"].(map[string]interface{}), int(d["draft_rev"].(float64))
}

// completeDraft is a publishable draft for upper with the given assets.
func completeDraft(upper, word, imageID, letterAudio, wordAudio string) map[string]interface{} {
	return map[string]interface{}{
		"kenali": map[string]interface{}{"word": word, "highlight": []int{0}, "image_asset_id": imageID},
		"dengar": map[string]interface{}{"letter_audio_id": letterAudio, "word_audio_id": wordAudio},
		"tebalkan": map[string]interface{}{
			"lower_required": false,
			"upper": []map[string]interface{}{
				{"order": 1, "label": "Garis miring kiri", "path": "M150 30 L70 260"},
				{"order": 2, "label": "Garis miring kanan", "path": "M150 30 L230 260"},
				{"order": 3, "label": "Garis tengah", "path": "M99 170 L201 170"},
			},
			"lower": []interface{}{},
		},
		"feedback": map[string]interface{}{
			"success": "Keren! Huruf " + upper + " rapi!", "retry": "Belum pas, ayo lagi!",
			"hint": "Mulai dari angka 1, lalu ikuti titik-titik sampai ujung.",
		},
	}
}

type assetIDs struct{ image, letterAudio, wordAudio string }

func uploadAssets(t *testing.T, env *Env, admin string) assetIDs {
	t.Helper()
	img := env.upload(admin, "image", "apel.webp", assetFixture(t, "square.webp"))
	require.Contains(t, []int{http.StatusCreated, http.StatusOK}, img.Code, string(img.Body))
	a1 := env.upload(admin, "audio", "a.mp3", mp3(40))
	require.Contains(t, []int{http.StatusCreated, http.StatusOK}, a1.Code, string(a1.Body))
	a2 := env.upload(admin, "audio", "apel.mp3", mp3(70))
	require.Contains(t, []int{http.StatusCreated, http.StatusOK}, a2.Code, string(a2.Body))
	return assetIDs{img.Data()["id"].(string), a1.Data()["id"].(string), a2.Data()["id"].(string)}
}

func saveDraft(env *Env, admin, id string, draft map[string]interface{}, rev int) *Response {
	return env.PUT("/admin/huruf/letters/"+id+"/draft", admin, map[string]interface{}{"draft": draft, "draft_rev": rev})
}

// publishLetter makes upper publishable and publishes it; returns its id.
func publishLetter(t *testing.T, env *Env, admin, upper, word string) string {
	t.Helper()
	id := letterID(t, env, admin, upper)
	a := uploadAssets(t, env, admin)
	_, rev := draftOf(t, env, admin, id)
	res := saveDraft(env, admin, id, completeDraft(upper, word, a.image, a.letterAudio, a.wordAudio), rev)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	pub := env.POST("/admin/huruf/letters/"+id+"/publish", admin, nil)
	require.Equal(t, http.StatusOK, pub.Code, string(pub.Body))
	return id
}

func parentOf(t *testing.T, env *Env, a *Account) *models.Parent {
	t.Helper()
	var p models.Parent
	require.NoError(t, env.DB.Where("id = ?", a.ID).First(&p).Error)
	return &p
}

func firstChild(t *testing.T, env *Env, a *Account) string {
	t.Helper()
	var id string
	require.NoError(t, env.DB.Raw("SELECT id::text FROM children WHERE parent_id = ? LIMIT 1", a.ID).Scan(&id).Error)
	return id
}

func manifestLetters(t *testing.T, res *Response) map[string]map[string]interface{} {
	t.Helper()
	out := map[string]map[string]interface{}{}
	for _, l := range res.Data()["letters"].([]interface{}) {
		m := l.(map[string]interface{})
		out[m["upper"].(string)] = m
	}
	return out
}

// ─── roles and audit ──────────────────────────────────────────────────────────

func TestHuruf_SeededLettersAndRoles(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)

	res := env.GET("/admin/huruf/letters", admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	rows := res.JSON()["data"].([]interface{})
	require.Len(t, rows, 26)
	free := 0
	for _, r := range rows {
		row := r.(map[string]interface{})
		assert.Equal(t, "draft", row["status"])
		assert.Equal(t, true, row["has_unpublished_changes"])
		if row["is_free"] == true {
			free++
			assert.Equal(t, "A", row["upper"])
		}
	}
	assert.Equal(t, 1, free)

	// A new admin row defaults to publisher, like every pre-existing admin.
	var role string
	require.NoError(t, env.DB.Raw("SELECT role FROM admin_users LIMIT 1").Scan(&role).Error)
	assert.Equal(t, "publisher", role)
}

func TestHuruf_EditorCannotPublishAndDemotionIsImmediate(t *testing.T) {
	env := NewAPIEnv(t)
	pubID, publisher := adminSession(t, env)
	_, editor := editorSession(t, env)
	id := letterID(t, env, editor, "B")

	// Editors can read and save drafts…
	d, rev := draftOf(t, env, editor, id)
	d["kenali"].(map[string]interface{})["word"] = "Bola"
	assert.Equal(t, http.StatusOK, saveDraft(env, editor, id, d, rev).Code)
	// …but not publish or manage roles.
	for _, path := range []string{"/publish", "/hide", "/set-free", "/unset-free", "/rollback/1"} {
		res := env.POST("/admin/huruf/letters/"+id+path, editor, nil)
		assert.Equal(t, http.StatusForbidden, res.Code, path)
	}
	// Editors may see who has which role, but not change it.
	assert.Equal(t, http.StatusOK, env.GET("/admin/admins", editor).Code)

	// A second publisher is demoted while holding a valid token.
	otherID, other := adminSession(t, env)
	assert.Equal(t, http.StatusNoContent, env.POST("/admin/huruf/letters/"+id+"/hide", other, nil).Code)
	res := env.PATCH("/admin/admins/"+otherID.String()+"/role", publisher, map[string]string{"role": "editor"})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	assert.Equal(t, http.StatusForbidden, env.POST("/admin/huruf/letters/"+id+"/unhide", other, nil).Code)

	// The role change is audited.
	var n int64
	env.DB.Model(&models.AdminAuditLog{}).Where("action = 'role.change' AND entity_id = ? AND admin_id = ?", otherID.String(), pubID).Count(&n)
	assert.EqualValues(t, 1, n)
}

func TestHuruf_LastPublisherIsProtected(t *testing.T) {
	env := NewAPIEnv(t)
	// The template DB has no admins; this is the only publisher.
	id, admin := adminSession(t, env)
	res := env.PATCH("/admin/admins/"+id.String()+"/role", admin, map[string]string{"role": "editor"})
	assert.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "LAST_PUBLISHER", res.JSON()["code"])

	list := env.GET("/admin/admins", admin)
	require.Equal(t, http.StatusOK, list.Code)
	assert.Len(t, list.JSON()["data"], 1)

	bad := env.PATCH("/admin/admins/"+id.String()+"/role", admin, map[string]string{"role": "owner"})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Code)
}

func TestHuruf_AutosaveAuditIsCoalesced(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	id := letterID(t, env, admin, "C")
	for i, word := range []string{"C", "Ci", "Cic", "Cica", "Cicak"} {
		d, rev := draftOf(t, env, admin, id)
		d["kenali"].(map[string]interface{})["word"] = word
		require.Equal(t, http.StatusOK, saveDraft(env, admin, id, d, rev).Code, "save %d", i)
	}
	res := env.GET("/admin/audit-log?entity_type=letter&entity_id="+id, admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	entries := res.JSON()["data"].([]interface{})
	require.Len(t, entries, 1)
	e := entries[0].(map[string]interface{})
	assert.Equal(t, "draft.save", e["action"])
	assert.Equal(t, "Cicak", e["after"].(map[string]interface{})["kenali"].(map[string]interface{})["word"])
	assert.NotEmpty(t, e["admin_email"])
}

// ─── assets ───────────────────────────────────────────────────────────────────

func TestHuruf_AssetUpload(t *testing.T) {
	env := NewAPIEnv(t)
	_, editor := editorSession(t, env)

	res := env.upload(editor, "image", "apel.webp", assetFixture(t, "square.webp"))
	require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	d := res.Data()
	assert.EqualValues(t, 512, d["width"])
	assert.Equal(t, "image/webp", d["mime"])
	assert.Contains(t, d["url"], "/media/huruf/")

	again := env.upload(editor, "image", "copy.webp", assetFixture(t, "square.webp"))
	require.Equal(t, http.StatusOK, again.Code)
	assert.Equal(t, d["id"], again.Data()["id"], "same bytes, same asset")

	cases := map[string]struct {
		kind, name string
		data       []byte
		reason     string
	}{
		"renamed jpeg": {"image", "apel.png", assetFixture(t, "photo.jpg"), "UNSUPPORTED_TYPE"},
		"not square":   {"image", "wide.png", assetFixture(t, "wide.png"), "NOT_SQUARE"},
		"too long":     {"audio", "long.mp3", mp3(460), "TOO_LONG"},
		"too large":    {"image", "big.png", append(assetFixture(t, "square.png"), make([]byte, 600*1024)...), "TOO_LARGE"},
	}
	for name, c := range cases {
		res := env.upload(editor, c.kind, c.name, c.data)
		assert.Equal(t, http.StatusUnprocessableEntity, res.Code, name)
		assert.Equal(t, c.reason, res.JSON()["reason"], name)
	}

	audio := env.upload(editor, "audio", "a.mp3", mp3(77))
	require.Equal(t, http.StatusCreated, audio.Code, string(audio.Body))
	assert.InDelta(t, 2011, audio.Data()["duration_ms"], 3)
}

// ─── publishing ───────────────────────────────────────────────────────────────

func TestHuruf_PublishValidation(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	id := letterID(t, env, admin, "D")
	a := uploadAssets(t, env, admin)

	d := completeDraft("D", "Domba", a.image, a.letterAudio, "")
	delete(d["dengar"].(map[string]interface{}), "word_audio_id")
	d["tebalkan"].(map[string]interface{})["upper"] = []map[string]interface{}{
		{"order": 1, "label": "x", "path": "M10 10 A5 5 0 0 1 20 20"},
		{"order": 3, "label": "y", "path": "M10 10 L20 20"},
	}
	d["kenali"].(map[string]interface{})["highlight"] = []int{1}
	d["kenali"].(map[string]interface{})["image_asset_id"] = a.letterAudio // wrong kind
	_, rev := draftOf(t, env, admin, id)
	require.Equal(t, http.StatusOK, saveDraft(env, admin, id, d, rev).Code)

	res := env.POST("/admin/huruf/letters/"+id+"/publish", admin, nil)
	require.Equal(t, http.StatusUnprocessableEntity, res.Code, string(res.Body))
	got := map[string]string{}
	for _, f := range res.JSON()["fields"].([]interface{}) {
		m := f.(map[string]interface{})
		got[m["path"].(string)] = m["code"].(string)
	}
	assert.Equal(t, map[string]string{
		"dengar.word_audio_id":   "REQUIRED",
		"kenali.highlight":       "INVALID",
		"kenali.image_asset_id":  "WRONG_KIND",
		"tebalkan.upper[0].path": "INVALID_PATH",
		"tebalkan.upper":         "ORDER_INVALID",
	}, got)

	var versions int64
	env.DB.Model(&models.LetterVersion{}).Where("letter_id = ?", id).Count(&versions)
	assert.Zero(t, versions)
}

func TestHuruf_DraftConflict(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	id := letterID(t, env, admin, "E")
	d, rev := draftOf(t, env, admin, id)
	require.Equal(t, http.StatusOK, saveDraft(env, admin, id, d, rev).Code)
	res := saveDraft(env, admin, id, d, rev)
	assert.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "DRAFT_CONFLICT", res.JSON()["code"])
}

func TestHuruf_PublishVersionsAndRollback(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	user := env.Register(t)
	id := publishLetter(t, env, admin, "A", "Apel")

	m := manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))
	assert.Equal(t, "Apel", m["A"]["word"])
	assert.EqualValues(t, 1, m["A"]["version"])

	// Saving a draft doesn't change what the app sees.
	d, rev := draftOf(t, env, admin, id)
	d["kenali"].(map[string]interface{})["word"] = "Anggur"
	require.Equal(t, http.StatusOK, saveDraft(env, admin, id, d, rev).Code)
	assert.Equal(t, "Apel", manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))["A"]["word"])
	draft := env.GET("/admin/huruf/letters/"+id+"/draft", admin).Data()
	assert.Equal(t, true, draft["has_unpublished_changes"])

	require.Equal(t, http.StatusOK, env.POST("/admin/huruf/letters/"+id+"/publish", admin, nil).Code)
	assert.Equal(t, "Anggur", manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))["A"]["word"])

	res := env.POST("/admin/huruf/letters/"+id+"/rollback/1", admin, nil)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	assert.EqualValues(t, 3, res.Data()["version"])
	a := manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))["A"]
	assert.Equal(t, "Apel", a["word"])
	assert.EqualValues(t, 3, a["version"])

	vs := env.GET("/admin/huruf/letters/"+id+"/versions", admin).JSON()["data"].([]interface{})
	require.Len(t, vs, 3)
	assert.EqualValues(t, 3, vs[0].(map[string]interface{})["version"])
	assert.Equal(t, true, vs[0].(map[string]interface{})["is_current"])
	assert.Equal(t, "Apel", draftWord(t, env, admin, id), "rollback also resets the draft")

	assert.Equal(t, http.StatusNotFound, env.POST("/admin/huruf/letters/"+id+"/rollback/9", admin, nil).Code)
}

func draftWord(t *testing.T, env *Env, admin, id string) string {
	d, _ := draftOf(t, env, admin, id)
	return d["kenali"].(map[string]interface{})["word"].(string)
}

func TestHuruf_FreeLetterHideAndOrder(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	user := env.Register(t)
	a := publishLetter(t, env, admin, "A", "Apel")
	b := publishLetter(t, env, admin, "B", "Bola")

	m := manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))
	assert.Equal(t, false, m["A"]["locked"])
	assert.Equal(t, true, m["B"]["locked"])

	require.Equal(t, http.StatusNoContent, env.POST("/admin/huruf/letters/"+b+"/set-free", admin, nil).Code)
	m = manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))
	assert.Equal(t, true, m["A"]["locked"])
	assert.Equal(t, false, m["B"]["locked"])
	var free int64
	env.DB.Model(&models.Letter{}).Where("is_free").Count(&free)
	assert.EqualValues(t, 1, free)

	// Hiding keeps progress.
	child := firstChild(t, env, user)
	require.Equal(t, http.StatusOK, env.PUT("/children/"+child+"/huruf/progress/"+b, user.Token,
		map[string]interface{}{"kenali_done": true}).Code)
	require.Equal(t, http.StatusNoContent, env.POST("/admin/huruf/letters/"+b+"/hide", admin, nil).Code)
	_, listed := manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))["B"]
	assert.False(t, listed)
	assert.Equal(t, http.StatusNotFound, env.GET("/learn/huruf/letters/"+b, user.Token).Code)
	var progress int64
	env.DB.Model(&models.LetterProgress{}).Where("letter_id = ?", b).Count(&progress)
	assert.EqualValues(t, 1, progress)
	require.Equal(t, http.StatusNoContent, env.POST("/admin/huruf/letters/"+b+"/unhide", admin, nil).Code)
	_, listed = manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))["B"]
	assert.True(t, listed)

	// Reorder: B first.
	rows := env.GET("/admin/huruf/letters", admin).JSON()["data"].([]interface{})
	ids := []string{b, a}
	for _, r := range rows {
		id := r.(map[string]interface{})["id"].(string)
		if id != a && id != b {
			ids = append(ids, id)
		}
	}
	assert.Equal(t, http.StatusUnprocessableEntity, env.PUT("/admin/huruf/order", admin, map[string]interface{}{"ids": ids[:25]}).Code)
	require.Equal(t, http.StatusNoContent, env.PUT("/admin/huruf/order", admin, map[string]interface{}{"ids": ids}).Code)
	letters := env.GET("/learn/huruf/manifest", user.Token).Data()["letters"].([]interface{})
	assert.Equal(t, "B", letters[0].(map[string]interface{})["upper"])

	// Unsetting the free letter leaves every letter premium.
	require.Equal(t, http.StatusNoContent, env.POST("/admin/huruf/letters/"+b+"/unset-free", admin, nil).Code)
	m = manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))
	assert.Equal(t, true, m["A"]["locked"])
	assert.Equal(t, true, m["B"]["locked"])
	env.DB.Model(&models.Letter{}).Where("is_free").Count(&free)
	assert.Zero(t, free)
	assert.Equal(t, http.StatusPaymentRequired, env.GET("/learn/huruf/letters/"+b, user.Token).Code)
}

func TestHuruf_ExternalMediaURLs(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	user := env.Register(t)
	id := letterID(t, env, admin, "A")
	const img, la, wa = "https://media.example.com/apel.png", "https://media.example.com/a.mp3", "https://media.example.com/apel.mp3"

	d := completeDraft("A", "Apel", "", "", "")
	d["kenali"] = map[string]interface{}{"word": "Apel", "highlight": []int{0}, "image_url": " " + img + " "}
	d["dengar"] = map[string]interface{}{"letter_audio_url": "http://media.example.com/a.mp3", "word_audio_url": wa}
	_, rev := draftOf(t, env, admin, id)
	saved := saveDraft(env, admin, id, d, rev)
	require.Equal(t, http.StatusOK, saved.Code, string(saved.Body))
	assert.Equal(t, img, saved.Data()["image_url"], "URLs are trimmed")
	assert.EqualValues(t, 2, saved.Data()["audio_count"])

	res := env.POST("/admin/huruf/letters/"+id+"/publish", admin, nil)
	require.Equal(t, http.StatusUnprocessableEntity, res.Code, string(res.Body))
	assert.Contains(t, string(res.Body), `"path":"dengar.letter_audio_url","code":"INVALID_URL"`)

	d["dengar"].(map[string]interface{})["letter_audio_url"] = la
	_, rev = draftOf(t, env, admin, id)
	require.Equal(t, http.StatusOK, saveDraft(env, admin, id, d, rev).Code)
	require.Equal(t, http.StatusOK, env.POST("/admin/huruf/letters/"+id+"/publish", admin, nil).Code)

	assert.Equal(t, img, manifestLetters(t, env.GET("/learn/huruf/manifest", user.Token))["A"]["image_url"])
	letter := env.GET("/learn/huruf/letters/"+id, user.Token)
	require.Equal(t, http.StatusOK, letter.Code, string(letter.Body))
	v := letter.Data()
	assert.Equal(t, img, v["kenali"].(map[string]interface{})["image_url"])
	assert.Equal(t, la, v["dengar"].(map[string]interface{})["letter_audio_url"])
	assert.Equal(t, wa, v["dengar"].(map[string]interface{})["word_audio_url"])

	// An uploaded asset replaces the URL for its slot.
	a := uploadAssets(t, env, admin)
	d["kenali"].(map[string]interface{})["image_asset_id"] = a.image
	_, rev = draftOf(t, env, admin, id)
	saved = saveDraft(env, admin, id, d, rev)
	require.Equal(t, http.StatusOK, saved.Code)
	assert.Contains(t, saved.Data()["image_url"], "/media/huruf/")
	_, hasURL := saved.Data()["draft"].(map[string]interface{})["kenali"].(map[string]interface{})["image_url"]
	assert.False(t, hasURL)
}

func TestHuruf_Preview(t *testing.T) {
	env := NewAPIEnv(t)
	_, editor := editorSession(t, env)
	id := letterID(t, env, editor, "A")
	a := uploadAssets(t, env, editor)
	res := env.POST("/admin/huruf/letters/"+id+"/preview", editor, completeDraft("A", "Anggur", a.image, a.letterAudio, a.wordAudio))
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	v := res.Data()
	assert.Equal(t, "Anggur", v["kenali"].(map[string]interface{})["word"])
	assert.Contains(t, v["kenali"].(map[string]interface{})["image_url"], "/media/huruf/")
	assert.Equal(t, "", draftWord(t, env, editor, id), "preview saves nothing")
}

// ─── app API ──────────────────────────────────────────────────────────────────

func TestHuruf_ManifestETagAndPremium(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	user := env.Register(t)
	publishLetter(t, env, admin, "A", "Apel")
	b := publishLetter(t, env, admin, "B", "Bola")

	res := env.GET("/learn/huruf/manifest", user.Token)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	data := res.Data()
	assert.Equal(t, false, data["premium"])
	th := data["tracing_thresholds"].(map[string]interface{})
	assert.EqualValues(t, 22, th["radius"])
	assert.EqualValues(t, 3, th["max_failures"])
	// D has never been published.
	_, hasD := manifestLetters(t, res)["D"]
	assert.False(t, hasD)

	tag := headerOf(env, "/learn/huruf/manifest", user.Token)
	require.NotEmpty(t, tag)
	assert.Equal(t, http.StatusNotModified, env.getWithHeader("/learn/huruf/manifest", user.Token, "If-None-Match", tag).Code)

	// Publishing changes the ETag.
	require.Equal(t, http.StatusOK, env.POST("/admin/huruf/letters/"+b+"/publish", admin, nil).Code)
	tag2 := headerOf(env, "/learn/huruf/manifest", user.Token)
	assert.NotEqual(t, tag, tag2)

	// So does subscribing.
	fixtures.NewSubscription(t, env.DB, parentOf(t, env, user))
	sub := env.getWithHeader("/learn/huruf/manifest", user.Token, "If-None-Match", tag2)
	require.Equal(t, http.StatusOK, sub.Code)
	assert.Equal(t, true, sub.Data()["premium"])
	assert.Equal(t, false, manifestLetters(t, sub)["B"]["locked"])
}

func headerOf(env *Env, path, token string) string {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	return rec.Header().Get("ETag")
}

func TestHuruf_LetterAccess(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	a := publishLetter(t, env, admin, "A", "Apel")
	b := publishLetter(t, env, admin, "B", "Bola")
	d := letterID(t, env, admin, "D") // never published

	free := env.Register(t)
	res := env.GET("/learn/huruf/letters/"+a, free.Token)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	v := res.Data()
	assert.Equal(t, "Apel", v["kenali"].(map[string]interface{})["word"])
	assert.Contains(t, v["dengar"].(map[string]interface{})["word_audio_url"], "/media/huruf/")
	assert.Len(t, v["tebalkan"].(map[string]interface{})["upper"], 3)

	locked := env.GET("/learn/huruf/letters/"+b, free.Token)
	assert.Equal(t, http.StatusPaymentRequired, locked.Code)
	assert.Equal(t, "PREMIUM_REQUIRED", locked.JSON()["code"])
	assert.Equal(t, http.StatusNotFound, env.GET("/learn/huruf/letters/"+d, free.Token).Code)

	expired := env.Register(t)
	fixtures.NewSubscription(t, env.DB, parentOf(t, env, expired), fixtures.ExpiringAt(time.Now().Add(-24*time.Hour)))
	assert.Equal(t, http.StatusPaymentRequired, env.GET("/learn/huruf/letters/"+b, expired.Token).Code)

	packOwner := env.Register(t)
	fixtures.NewEntitlement(t, env.DB, parentOf(t, env, packOwner), fixtures.NewProduct(t, env.DB))
	assert.Equal(t, http.StatusPaymentRequired, env.GET("/learn/huruf/letters/"+b, packOwner.Token).Code)

	premium := env.Register(t)
	fixtures.NewSubscription(t, env.DB, parentOf(t, env, premium))
	assert.Equal(t, http.StatusOK, env.GET("/learn/huruf/letters/"+b, premium.Token).Code)

	assert.Equal(t, http.StatusUnauthorized, env.GET("/learn/huruf/manifest", "").Code)
}

// Every premium-guarded route answers 402 for a non-free letter and lets the
// free one through.
func TestHuruf_PremiumGuardOnEveryRoute(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	a := publishLetter(t, env, admin, "A", "Apel")
	b := publishLetter(t, env, admin, "B", "Bola")
	user := env.Register(t)
	child := firstChild(t, env, user)

	routes := []struct {
		name string
		call func(letter string) *Response
	}{
		{"GET letter", func(l string) *Response { return env.GET("/learn/huruf/letters/"+l, user.Token) }},
		{"PUT progress", func(l string) *Response {
			return env.PUT("/children/"+child+"/huruf/progress/"+l, user.Token, map[string]interface{}{"kenali_done": true})
		}},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusPaymentRequired, r.call(b).Code, r.name+" locked")
		assert.Equal(t, http.StatusOK, r.call(a).Code, r.name+" free")
	}
}

func TestHuruf_ProgressMerge(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	a := publishLetter(t, env, admin, "A", "Apel")
	user := env.Register(t)
	child := firstChild(t, env, user)
	path := "/children/" + child + "/huruf/progress/" + a

	put := func(body map[string]interface{}) map[string]interface{} {
		res := env.PUT(path, user.Token, body)
		require.Equal(t, http.StatusOK, res.Code, string(res.Body))
		return res.Data()
	}
	put(map[string]interface{}{"tebalkan_done": true, "score": 0.91, "attempt": true})
	got := put(map[string]interface{}{"tebalkan_done": false, "dengar_done": true, "score": 0.84, "attempt": true})
	assert.Equal(t, true, got["tebalkan_done"], "a stale write never clears a completion")
	assert.Equal(t, true, got["dengar_done"])
	assert.InDelta(t, 0.91, got["best_score"], 0.001)
	assert.EqualValues(t, 2, got["attempts"])

	bad := env.PUT(path, user.Token, map[string]interface{}{"score": 1.5})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Code)

	list := env.GET("/children/"+child+"/huruf/progress", user.Token)
	require.Equal(t, http.StatusOK, list.Code)
	rows := list.JSON()["data"].([]interface{})
	require.Len(t, rows, 1)
	assert.Equal(t, a, rows[0].(map[string]interface{})["letter_id"])

	// Another parent's child: 404, nothing stored.
	other := env.Register(t)
	assert.Equal(t, http.StatusNotFound, env.PUT(path, other.Token, map[string]interface{}{"kenali_done": true}).Code)
	assert.Equal(t, http.StatusNotFound, env.GET("/children/"+child+"/huruf/progress", other.Token).Code)
	var n int64
	env.DB.Model(&models.LetterProgress{}).Count(&n)
	assert.EqualValues(t, 1, n)
}

func TestHuruf_AccountDeletionRemovesProgress(t *testing.T) {
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	a := publishLetter(t, env, admin, "A", "Apel")
	user := env.Register(t)
	child := firstChild(t, env, user)
	require.Equal(t, http.StatusOK, env.PUT("/children/"+child+"/huruf/progress/"+a, user.Token,
		map[string]interface{}{"kenali_done": true}).Code)

	res := env.DELETE("/user/me", user.Token)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, res.Code, string(res.Body))
	var n int64
	env.DB.Model(&models.LetterProgress{}).Where("child_id = ?", child).Count(&n)
	assert.Zero(t, n)
}

func TestHuruf_FlagSeeded(t *testing.T) {
	env := NewAPIEnv(t)
	res := env.GET("/app/feature-flags", "")
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	raw, _ := json.Marshal(res.JSON())
	assert.Contains(t, string(raw), `"belajar_huruf":false`)
}

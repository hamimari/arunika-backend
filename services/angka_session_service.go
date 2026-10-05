package services

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"arunika_backend/angkagen"
	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// angkaSessionTTL is how long an untouched session can still be resumed.
const angkaSessionTTL = 7 * 24 * time.Hour

// maxAngkaAnswer bounds a typed answer (two digits on the number pad).
const maxAngkaAnswer = 99

// AngkaSessionView is a started or resumed session with everything the app
// needs to regenerate and play it, pinned to the version it started on.
type AngkaSessionView struct {
	ID           uuid.UUID                `json:"session_id"`
	LevelID      uuid.UUID                `json:"level_id"`
	LevelVersion int                      `json:"level_version"`
	Seed         uint32                   `json:"seed"`
	Status       string                   `json:"status"`
	ObjectIDs    []uuid.UUID              `json:"object_ids"`
	Answers      []models.AngkaAnswer     `json:"answers"`
	Content      models.AngkaLevelContent `json:"content"`
	Objects      []AngkaObjectView        `json:"objects"`
	CountAudio   map[string]string        `json:"count_audio"`
}

func sessionAnswers(s *models.AngkaSession) []models.AngkaAnswer {
	out := []models.AngkaAnswer{}
	_ = json.Unmarshal(s.Answers, &out)
	return out
}

func sessionObjects(s *models.AngkaSession) []uuid.UUID {
	out := []uuid.UUID{}
	_ = json.Unmarshal(s.ObjectIDs, &out)
	return out
}

func randomSeed() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b[:]), nil
}

// abandonStale marks a child's sessions untouched for angkaSessionTTL as
// abandoned and drops them as current (no cron: done when read).
func abandonStale(tx *gorm.DB, childID uuid.UUID) error {
	cutoff := time.Now().Add(-angkaSessionTTL)
	var stale []uuid.UUID
	if err := tx.Model(&models.AngkaSession{}).
		Where("child_id = ? AND status = ? AND updated_at < ?", childID, models.AngkaSessionInProgress, cutoff).
		Pluck("id", &stale).Error; err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	if err := tx.Model(&models.AngkaSession{}).Where("id IN ?", stale).
		Update("status", models.AngkaSessionAbandoned).Error; err != nil {
		return err
	}
	return tx.Model(&models.AngkaLevelProgress{}).Where("current_session_id IN ?", stale).
		Update("current_session_id", nil).Error
}

// loadSession loads the child's session (optionally FOR UPDATE) with the
// content and number of the version it is pinned to.
func loadSession(tx *gorm.DB, childID, id uuid.UUID, lock bool) (*models.AngkaSession, models.AngkaLevelContent, int, error) {
	var sess models.AngkaSession
	q := tx
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.Where("id = ? AND child_id = ?", id, childID).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.AngkaLevelContent{}, 0, ErrAngkaSessionNotFound
		}
		return nil, models.AngkaLevelContent{}, 0, err
	}
	var v models.AngkaLevelVersion
	if err := tx.Where("id = ?", sess.LevelVersionID).First(&v).Error; err != nil {
		return nil, models.AngkaLevelContent{}, 0, err
	}
	return &sess, parseLevel(v.Content), v.Version, nil
}

// finishedCount is how many questions have a correct try or no tries left.
func finishedCount(qs []angkagen.Question, answers []models.AngkaAnswer) int {
	n := 0
	for _, a := range answers {
		if a.Q < 1 || a.Q > len(qs) {
			continue
		}
		if questionFinished(qs[a.Q-1].Count, a.Tries) {
			n++
		}
	}
	return n
}

// questionFinished reports whether the child has given the right answer;
// a child keeps trying until they do.
func questionFinished(count int, tries []int) bool {
	for _, t := range tries {
		if t == count {
			return true
		}
	}
	return false
}

// StartSession starts a level, or resumes the child's in-progress session on
// it. Checks run in order: the child (404), the level (404), access (402),
// then the prerequisite (409).
func (s *AngkaService) StartSession(userID, childID, levelID uuid.UUID, restart bool) (*AngkaSessionView, error) {
	if err := s.ownedChild(userID, childID); err != nil {
		return nil, err
	}
	rows, err := visibleLevels(s.db, "l.id = ?", levelID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrAngkaLevelNotFound
	}
	level := rows[0]
	if err := s.canAccess(userID, level.IsFree); err != nil {
		return nil, err
	}
	content := parseLevel(level.Content)
	if err := s.checkPrerequisite(childID, content.PrerequisiteID); err != nil {
		return nil, err
	}

	var sess *models.AngkaSession
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := abandonStale(tx, childID); err != nil {
			return err
		}
		// The progress row serialises concurrent starts for one level.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.AngkaLevelProgress{
			ChildID: childID, LevelID: levelID, UpdatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
		var prog models.AngkaLevelProgress
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("child_id = ? AND level_id = ?", childID, levelID).First(&prog).Error; err != nil {
			return err
		}
		if prog.CurrentSessionID != nil && !restart {
			var cur models.AngkaSession
			err := tx.Where("id = ? AND status = ?", *prog.CurrentSessionID, models.AngkaSessionInProgress).First(&cur).Error
			if err == nil {
				sess = &cur
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := tx.Model(&models.AngkaSession{}).
			Where("child_id = ? AND level_id = ? AND status = ?", childID, levelID, models.AngkaSessionInProgress).
			Update("status", models.AngkaSessionAbandoned).Error; err != nil {
			return err
		}
		snapshot, err := s.objectSnapshot(tx, content.ObjectIDs)
		if err != nil {
			return err
		}
		seed, err := randomSeed()
		if err != nil {
			return err
		}
		now := time.Now()
		created := models.AngkaSession{
			ChildID: childID, LevelID: levelID, LevelVersionID: level.VersionID,
			Seed: int64(seed), ObjectIDs: models.ToJSONB(snapshot), Answers: models.JSONB("[]"),
			Status: models.AngkaSessionInProgress, StartedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		sess = &created
		return tx.Model(&prog).Updates(map[string]interface{}{
			"current_session_id": created.ID, "updated_at": now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.sessionView(childID, sess.ID)
}

// checkPrerequisite answers ErrLevelLocked when the level's prerequisite is
// visible and the child hasn't completed it. A hidden or unpublished
// prerequisite counts as met, so hiding a level never strands the next.
func (s *AngkaService) checkPrerequisite(childID uuid.UUID, prereq *uuid.UUID) error {
	if prereq == nil {
		return nil
	}
	rows, err := visibleLevels(s.db, "l.id = ?", *prereq)
	if err != nil || len(rows) == 0 {
		return err
	}
	var done int64
	if err := s.db.Model(&models.AngkaLevelProgress{}).
		Where("child_id = ? AND level_id = ? AND completed", childID, *prereq).
		Count(&done).Error; err != nil {
		return err
	}
	if done == 0 {
		return ErrLevelLocked
	}
	return nil
}

// objectSnapshot keeps the level's objects that can be used in new
// questions; if every one was hidden, it keeps them all rather than fail.
func (s *AngkaService) objectSnapshot(tx *gorm.DB, ids []uuid.UUID) ([]uuid.UUID, error) {
	var objs []models.AngkaObject
	if err := tx.Where("id IN ?", ids).Find(&objs).Error; err != nil {
		return nil, err
	}
	usable := map[uuid.UUID]bool{}
	for _, o := range objs {
		usable[o.ID] = visibleObject(o)
	}
	out := []uuid.UUID{}
	for _, id := range ids {
		if usable[id] {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return ids, nil
	}
	return out, nil
}

func (s *AngkaService) sessionView(childID, id uuid.UUID) (*AngkaSessionView, error) {
	sess, content, version, err := loadSession(s.db, childID, id, false)
	if err != nil {
		return nil, err
	}
	objectIDs := sessionObjects(sess)
	lv, err := s.levelView(s.db, sess.LevelID, version, content, objectIDs)
	if err != nil {
		return nil, err
	}
	return &AngkaSessionView{
		ID: sess.ID, LevelID: sess.LevelID, LevelVersion: version, Seed: uint32(sess.Seed),
		Status: sess.Status, ObjectIDs: objectIDs, Answers: sessionAnswers(sess),
		Content: content, Objects: lv.Objects, CountAudio: lv.CountAudio,
	}, nil
}

// TryResult is a recorded try and the server's verdict on it.
type TryResult struct {
	Q        int                  `json:"q"`
	Try      int                  `json:"try"`
	Correct  bool                 `json:"correct"`
	Finished bool                 `json:"finished"`
	Answers  []models.AngkaAnswer `json:"answers"`
}

// RecordTry appends try number try for question q. A repeat of a recorded
// try with the same answer is a no-op. Only ownership is checked, not
// access, so a level started before a subscription ended can be finished.
func (s *AngkaService) RecordTry(userID, childID, sessionID uuid.UUID, q, try, answer int) (*TryResult, error) {
	if err := s.ownedChild(userID, childID); err != nil {
		return nil, err
	}
	var out *TryResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		sess, content, _, err := loadSession(tx, childID, sessionID, true)
		if err != nil {
			return err
		}
		qs := generate(content, sessionObjects(sess), uint32(sess.Seed))
		if q < 1 || q > len(qs) || try < 1 || answer < 0 || answer > maxAngkaAnswer {
			return ErrInvalidTry
		}
		count := qs[q-1].Count
		answers := sessionAnswers(sess)
		idx := -1
		for i, a := range answers {
			if a.Q == q {
				idx = i
			}
		}
		if idx < 0 {
			answers = append(answers, models.AngkaAnswer{Q: q, Tries: []int{}})
			idx = len(answers) - 1
		}
		tries := answers[idx].Tries
		switch {
		case try <= len(tries):
			if tries[try-1] != answer {
				return ErrInvalidTry
			}
		case sess.Status != models.AngkaSessionInProgress,
			try != len(tries)+1,
			try > angkaMaxWrongTries && answer != count,
			questionFinished(count, tries):
			return ErrInvalidTry
		default:
			answers[idx].Tries = append(tries, answer)
			sort.Slice(answers, func(i, j int) bool { return answers[i].Q < answers[j].Q })
			if err := tx.Model(sess).Updates(map[string]interface{}{
				"answers": models.ToJSONB(answers), "updated_at": time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		final := answersFor(answers, q)
		out = &TryResult{
			Q: q, Try: try, Correct: answer == count,
			Finished: questionFinished(count, final), Answers: answers,
		}
		return nil
	})
	return out, err
}

func answersFor(answers []models.AngkaAnswer, q int) []int {
	for _, a := range answers {
		if a.Q == q {
			return a.Tries
		}
	}
	return nil
}

// CompleteResult is the server's scoring of a finished session.
type CompleteResult struct {
	Stars            int         `json:"stars"`
	FirstCorrect     int         `json:"first_correct"`
	BestStars        int         `json:"best_stars"`
	UnlockedLevelIDs []uuid.UUID `json:"unlocked_level_ids"`
}

// CompleteSession regenerates the session's questions, scores the recorded
// tries and updates progress. Completing again returns the same result.
func (s *AngkaService) CompleteSession(userID, childID, sessionID uuid.UUID) (*CompleteResult, error) {
	if err := s.ownedChild(userID, childID); err != nil {
		return nil, err
	}
	var out *CompleteResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		sess, content, _, err := loadSession(tx, childID, sessionID, true)
		if err != nil {
			return err
		}
		var prog models.AngkaLevelProgress
		progErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("child_id = ? AND level_id = ?", childID, sess.LevelID).First(&prog).Error
		if progErr != nil && !errors.Is(progErr, gorm.ErrRecordNotFound) {
			return progErr
		}
		if sess.Status == models.AngkaSessionCompleted {
			unlocked := []uuid.UUID{}
			_ = json.Unmarshal(sess.Unlocked, &unlocked)
			out = &CompleteResult{
				Stars: deInt(sess.Stars), FirstCorrect: deInt(sess.FirstCorrect),
				BestStars: prog.BestStars, UnlockedLevelIDs: unlocked,
			}
			return nil
		}
		qs := generate(content, sessionObjects(sess), uint32(sess.Seed))
		answers := sessionAnswers(sess)
		if finishedCount(qs, answers) < len(qs) {
			return ErrSessionIncomplete
		}
		first := 0
		for _, a := range answers {
			if len(a.Tries) > 0 && a.Tries[0] == qs[a.Q-1].Count {
				first++
			}
		}
		stars := starsFor(first, content.Stars)
		unlocked := []uuid.UUID{}
		if !prog.Completed {
			if unlocked, err = s.unlockedBy(tx, sess.LevelID); err != nil {
				return err
			}
		}
		now := time.Now()
		if err := tx.Model(sess).Updates(map[string]interface{}{
			"status": models.AngkaSessionCompleted, "stars": stars, "first_correct": first,
			"unlocked": models.ToJSONB(unlocked), "completed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		best := stars
		if prog.BestStars > best {
			best = prog.BestStars
		}
		current := prog.CurrentSessionID
		if current != nil && *current == sess.ID {
			current = nil
		}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&models.AngkaLevelProgress{
			ChildID: childID, LevelID: sess.LevelID, BestStars: best, Completed: true,
			CurrentSessionID: current, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
		out = &CompleteResult{Stars: stars, FirstCorrect: first, BestStars: best, UnlockedLevelIDs: unlocked}
		return nil
	})
	return out, err
}

func deInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// unlockedBy lists visible levels whose published prerequisite is levelID.
func (s *AngkaService) unlockedBy(tx *gorm.DB, levelID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := visibleLevels(tx, "v.content->>'prerequisite_id' = ?", levelID.String())
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out, nil
}

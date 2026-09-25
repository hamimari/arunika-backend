package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// RecordDongengPlay's upsert once used an unqualified `play_count + 1`, which
// Postgres rejects as ambiguous between the target table and EXCLUDED — so
// every play-record call returned 500. sqlmock never parses SQL, so only a
// real database can catch this class of defect.
func TestRecordDongengPlay_FirstAndRepeatPlays_CountUp(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	dongeng := fixtures.NewDongeng(t, db)

	require.NoError(t, models.RecordDongengPlay(db, user.ID, dongeng.ID))
	require.NoError(t, models.RecordDongengPlay(db, user.ID, dongeng.ID), "a repeat play must upsert, not fail")

	var row models.DongengPlayHistory
	require.NoError(t, db.Where("user_id = ? AND dongeng_id = ?", user.ID, dongeng.ID).First(&row).Error)
	assert.Equal(t, 2, row.PlayCount)
}

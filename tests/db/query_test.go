package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// These replace the SQL-string assertions in handlers/handler_test.go, which
// matched GORM's rendered output with regexp.QuoteMeta. That approach breaks
// whenever a query is rewritten to produce identical results, and proves
// nothing about the rows actually returned. These assert the data instead.

func TestFindAllFairyTales_Pagination_ReturnsTheRequestedSlice(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	for i := 0; i < 7; i++ {
		fixtures.NewDongeng(t, db)
	}

	first, total, err := models.FindAllFairyTales(db, "", 1, 3, "", "")
	require.NoError(t, err)
	assert.Len(t, first, 3, "page 1 must hold perPage items")
	assert.Equal(t, int64(7), total, "total must count the whole filtered set, not the page")

	second, _, err := models.FindAllFairyTales(db, "", 2, 3, "", "")
	require.NoError(t, err)
	assert.Len(t, second, 3)

	last, _, err := models.FindAllFairyTales(db, "", 3, 3, "", "")
	require.NoError(t, err)
	assert.Len(t, last, 1, "the final page holds the remainder")

	// Pages must not overlap — an off-by-one in OFFSET shows up here and
	// nowhere else.
	seen := map[string]bool{}
	for _, batch := range [][]models.Dongeng{first, second, last} {
		for _, d := range batch {
			assert.False(t, seen[d.ID.String()], "dongeng %s appeared on two pages", d.ID)
			seen[d.ID.String()] = true
		}
	}
	assert.Len(t, seen, 7, "paging through must yield every row exactly once")
}

func TestFindAllFairyTales_PastTheEnd_ReturnsEmptyNotError(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	fixtures.NewDongeng(t, db)

	items, total, err := models.FindAllFairyTales(db, "", 5, 10, "", "")

	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(1), total, "total is independent of the page requested")
}

func TestFindAllFairyTales_SoftDeletedAndHidden_AreExcluded(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	visible := fixtures.NewDongeng(t, db, fixtures.WithTitle("Visible"))
	fixtures.NewDongeng(t, db, fixtures.WithTitle("Deleted"), fixtures.SoftDeleted())
	fixtures.NewDongeng(t, db, fixtures.WithTitle("Hidden"), fixtures.Hidden())

	items, total, err := models.FindAllFairyTales(db, "", 1, 50, "", "")

	require.NoError(t, err)
	require.Len(t, items, 1, "neither soft-deleted nor hidden content may be listed")
	assert.Equal(t, visible.ID, items[0].ID)
	assert.Equal(t, int64(1), total, "the count must apply the same filters as the page")
}

func TestFindAllFairyTales_Search_IsCaseInsensitiveAndPartial(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	fixtures.NewDongeng(t, db, fixtures.WithTitle("Kancil dan Buaya"))
	fixtures.NewDongeng(t, db, fixtures.WithTitle("Timun Mas"))

	for _, term := range []string{"kancil", "KANCIL", "dan Bua"} {
		t.Run("matches "+term, func(t *testing.T) {
			items, total, err := models.FindAllFairyTales(db, term, 1, 50, "", "")
			require.NoError(t, err)
			require.Len(t, items, 1, "ILIKE must match case-insensitively on a substring")
			assert.Equal(t, "Kancil dan Buaya", items[0].Title)
			assert.Equal(t, int64(1), total)
		})
	}

	t.Run("no match returns empty", func(t *testing.T) {
		items, total, err := models.FindAllFairyTales(db, "Nonexistent", 1, 50, "", "")
		require.NoError(t, err)
		assert.Empty(t, items)
		assert.Zero(t, total)
	})
}

// Search terms come straight from a query parameter. GORM parameterises them,
// so metacharacters must be treated as literal text rather than altering the
// query — this is the regression test for that guarantee.
func TestFindAllFairyTales_SearchMetacharacters_AreTreatedAsData(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	fixtures.NewDongeng(t, db, fixtures.WithTitle("Kancil"))

	for _, hostile := range []string{
		"' OR '1'='1",
		"'; DROP TABLE dongengs; --",
		"100%",
	} {
		t.Run(hostile, func(t *testing.T) {
			items, _, err := models.FindAllFairyTales(db, hostile, 1, 50, "", "")
			require.NoError(t, err, "hostile input must not error, just not match")
			assert.Empty(t, items, "metacharacters must not widen the result set")
		})
	}

	// The table is still there and still holds its row.
	var count int64
	require.NoError(t, db.Model(&models.Dongeng{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the dongengs table must be intact")
}

func TestFindAllFairyTales_CategoryFilter_NarrowsToThatCategory(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	wanted := fixtures.NewDongengCategory(t, db)
	other := fixtures.NewDongengCategory(t, db)

	fixtures.NewDongeng(t, db, fixtures.InCategory(wanted.ID))
	fixtures.NewDongeng(t, db, fixtures.InCategory(wanted.ID))
	fixtures.NewDongeng(t, db, fixtures.InCategory(other.ID))
	fixtures.NewDongeng(t, db) // uncategorised

	items, total, err := models.FindAllFairyTales(db, "", 1, 50, wanted.ID.String(), "")

	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, int64(2), total)
	for _, d := range items {
		require.NotNil(t, d.DongengCategoryID)
		assert.Equal(t, wanted.ID, *d.DongengCategoryID)
	}
}

func TestFindFairyTaleByID_HiddenOrDeleted_IsNotRetrievable(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	hidden := fixtures.NewDongeng(t, db, fixtures.Hidden())
	deleted := fixtures.NewDongeng(t, db, fixtures.SoftDeleted())
	visible := fixtures.NewDongeng(t, db)

	for name, id := range map[string]string{
		"hidden":  hidden.ID.String(),
		"deleted": deleted.ID.String(),
	} {
		t.Run(name+" is not retrievable", func(t *testing.T) {
			_, err := models.FindFairyTaleByID(db, id)
			require.Error(t, err, "fetching by id must apply the same visibility rules as listing")
		})
	}

	t.Run("visible is retrievable", func(t *testing.T) {
		got, err := models.FindFairyTaleByID(db, visible.ID.String())
		require.NoError(t, err)
		assert.Equal(t, visible.ID, got.ID)
	})
}

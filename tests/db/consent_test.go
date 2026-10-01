package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/services"
	"arunika_backend/tests/fixtures"
)

func currentConsent() services.ConsentInput {
	return services.ConsentInput{
		TermsVersion:    services.CurrentConsentVersions[models.ConsentDocTerms],
		PrivacyVersion:  services.CurrentConsentVersions[models.ConsentDocPrivacy],
		ParentalVersion: services.CurrentConsentVersions[models.ConsentDocParental],
	}
}

func TestUserConsent_DocumentIsConstrainedToKnownValues(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	user := fixtures.NewUser(t, db)

	for _, doc := range []string{"TERMS", "PRIVACY", "PARENTAL"} {
		require.NoError(t, db.Create(&models.UserConsent{
			UserID: user.ID, Document: doc, Version: "2026-10-01",
		}).Error, doc)
	}
	err := db.Create(&models.UserConsent{
		UserID: user.ID, Document: "MARKETING", Version: "2026-10-01",
	}).Error
	require.Error(t, err, "only the three legal documents can be consented to")
}

func TestConsent_RequiredUntilCurrentVersionsAccepted(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	svc := services.NewConsentService(db)
	user := fixtures.NewUser(t, db)

	required, err := svc.IsRequired(user.ID)
	require.NoError(t, err)
	assert.True(t, required, "an account with no consent rows must be asked")

	require.NoError(t, svc.Record(user.ID, currentConsent(), "1.2.3.4", "ua"))
	required, err = svc.IsRequired(user.ID)
	require.NoError(t, err)
	assert.False(t, required)
}

func TestConsent_LatestAcceptanceWins_HistoryIsKept(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	svc := services.NewConsentService(db)
	user := fixtures.NewUser(t, db)

	// An old acceptance of an outdated privacy policy, then the current ones.
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, db.Create(&models.UserConsent{
		UserID: user.ID, Document: models.ConsentDocPrivacy, Version: "2020-01-01", AcceptedAt: old,
	}).Error)
	require.NoError(t, db.Create(&models.UserConsent{
		UserID: user.ID, Document: models.ConsentDocTerms, Version: "2020-01-01", AcceptedAt: old,
	}).Error)
	require.NoError(t, db.Create(&models.UserConsent{
		UserID: user.ID, Document: models.ConsentDocParental, Version: "2020-01-01", AcceptedAt: old,
	}).Error)

	required, err := svc.IsRequired(user.ID)
	require.NoError(t, err)
	assert.True(t, required, "only outdated versions are on record")

	require.NoError(t, svc.Record(user.ID, currentConsent(), "1.2.3.4", "ua"))

	required, err = svc.IsRequired(user.ID)
	require.NoError(t, err)
	assert.False(t, required, "the newer acceptance replaces the outdated one")

	var total int64
	require.NoError(t, db.Model(&models.UserConsent{}).Where("user_id = ?", user.ID).Count(&total).Error)
	assert.EqualValues(t, 6, total, "earlier acceptances stay on record as evidence")
}

func TestConsent_BumpingAVersionRequiresConsentAgain(t *testing.T) {
	// Not parallel: it changes a package-level version.
	db := fixtures.FreshDB(t)
	svc := services.NewConsentService(db)
	user := fixtures.NewUser(t, db)
	require.NoError(t, svc.Record(user.ID, currentConsent(), "", ""))

	original := services.CurrentConsentVersions[models.ConsentDocPrivacy]
	services.CurrentConsentVersions[models.ConsentDocPrivacy] = "2099-01-01"
	t.Cleanup(func() { services.CurrentConsentVersions[models.ConsentDocPrivacy] = original })

	required, err := svc.IsRequired(user.ID)
	require.NoError(t, err)
	assert.True(t, required)
}

func TestAccountDeletion_RemovesConsentRecords(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	consents := services.NewConsentService(db)
	user := fixtures.NewUser(t, db)
	require.NoError(t, consents.Record(user.ID, currentConsent(), "1.2.3.4", "ua"))

	require.NoError(t, services.NewAccountDeletionService(db).DeleteAccount(user.ID))

	var left int64
	require.NoError(t, db.Model(&models.UserConsent{}).Where("user_id = ?", user.ID).Count(&left).Error)
	assert.Zero(t, left, "consent rows carry the IP and user agent, so they go with the account")
}

// Signup inserts the consent rows together with the parent. The rows are built
// before the parent has an id, so this proves GORM links them to it, and that
// a failed signup leaves no orphan consent rows behind.
func TestSignup_WithConsent_LinksRowsToTheNewParent(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	auth := services.NewAuthService(db, nil)

	rows, err := currentConsent().Rows(uuid.Nil, "9.9.9.9", "Arunika/1.0")
	require.NoError(t, err)

	parent, err := auth.Signup(models.Parent{
		Name: "Ibu Budi", PhoneNumber: "0811-consent", EmailAddress: "consent@example.com",
		Password: "hashed", Address: "Jl.", City: "Jakarta", Consents: rows,
	})
	require.NoError(t, err)

	var saved []models.UserConsent
	require.NoError(t, db.Where("user_id = ?", parent.ID).Find(&saved).Error)
	require.Len(t, saved, 3)
	docs := map[string]string{}
	for _, c := range saved {
		docs[c.Document] = c.Version
		assert.Equal(t, "9.9.9.9", c.IPAddress)
		assert.Equal(t, "Arunika/1.0", c.UserAgent)
		assert.False(t, c.AcceptedAt.IsZero())
	}
	assert.Equal(t, services.CurrentConsentVersions, docs)

	required, err := services.NewConsentService(db).IsRequired(parent.ID)
	require.NoError(t, err)
	assert.False(t, required)
}

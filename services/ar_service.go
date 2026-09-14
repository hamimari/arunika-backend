package services

import (
	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ArService struct {
	db                 *gorm.DB
	productService     *ProductService
	entitlementService *EntitlementService
}

func NewArService(db *gorm.DB, productService *ProductService, entitlementService *EntitlementService) *ArService {
	return &ArService{db: db, productService: productService, entitlementService: entitlementService}
}

func (s *ArService) GetByID(id string, userID *uuid.UUID) (*models.ArCards, error) {
	card, err := models.FindCardById(s.db, id)
	if err != nil {
		return nil, err
	}
	if err := s.applyUnlocked(card, userID); err != nil {
		return nil, err
	}
	return card, nil
}

func (s *ArService) GetAll(categoryID, subCategoryID string, userID *uuid.UUID) ([]models.ArCards, error) {
	cards, err := models.FindAllCards(s.db, categoryID, subCategoryID)
	if err != nil {
		return nil, err
	}
	for i := range cards {
		if err := s.applyUnlocked(&cards[i], userID); err != nil {
			return nil, err
		}
	}
	return cards, nil
}

func (s *ArService) GetAllCategories() ([]models.ArCardCategory, error) {
	return models.FindTopLevelCategories(s.db)
}

// applyUnlocked overwrites card.IsUnlocked with a per-request computed value:
// free content (no linked product) is always unlocked; otherwise it depends
// on the requesting user's entitlements/subscription. Card.IsUnlocked remains
// a stored column for now (dropped once this cutover is verified), but its
// value read from the DB is never trusted here.
func (s *ArService) applyUnlocked(card *models.ArCards, userID *uuid.UUID) error {
	product, err := s.productService.ResolveByArCardID(card.ID)
	if err != nil {
		return err
	}
	if product == nil {
		card.IsUnlocked = true
		return nil
	}

	var unlocked bool
	if userID != nil {
		unlocked, err = s.entitlementService.HasAccess(*userID, product.ID)
		if err != nil {
			return err
		}
	}

	// Inactive products are withdrawn from sale — hide the purchase option
	// from anyone who doesn't already own it. Existing owners (unlocked via
	// entitlement/subscription) keep full access, unaffected by is_active.
	if !product.IsActive && !unlocked {
		card.IsUnlocked = false
		return nil
	}

	card.ProductID = &product.ID
	card.PriceIdr = &product.PriceIdr
	card.IsUnlocked = unlocked
	return nil
}

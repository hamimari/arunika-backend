package services

import (
	"arunika_backend/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ArService struct {
	db                 *gorm.DB
	productService     *ProductService
	entitlementService *EntitlementService
	strike             *StrikePriceService
}

func NewArService(db *gorm.DB, productService *ProductService, entitlementService *EntitlementService) *ArService {
	return &ArService{db: db, productService: productService, entitlementService: entitlementService}
}

// WithStrikePricing enables promotional strike prices on card responses.
// Without it, cards are served with no strike price.
func (s *ArService) WithStrikePricing(strike *StrikePriceService) *ArService {
	s.strike = strike
	return s
}

func (s *ArService) GetByID(id string, userID *uuid.UUID) (*models.ArCards, error) {
	card, err := models.FindCardById(s.db, id)
	if err != nil {
		return nil, err
	}
	rules, err := s.strike.LoadRules()
	if err != nil {
		return nil, err
	}
	if err := s.applyUnlocked(card, userID, rules); err != nil {
		return nil, err
	}
	return card, nil
}

func (s *ArService) GetAll(categoryID, subCategoryID string, userID *uuid.UUID) ([]models.ArCards, error) {
	cards, err := models.FindAllCards(s.db, categoryID, subCategoryID)
	if err != nil {
		return nil, err
	}
	rules, err := s.strike.LoadRules()
	if err != nil {
		return nil, err
	}
	for i := range cards {
		if err := s.applyUnlocked(&cards[i], userID, rules); err != nil {
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
// value read from the DB is never trusted here. The display-only strike
// price is resolved alongside the price from the request's loaded rules.
func (s *ArService) applyUnlocked(card *models.ArCards, userID *uuid.UUID, rules StrikeRules) error {
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
	card.PlayProductID = product.PlayProductID
	card.StrikeDisplay = rules.Resolve(models.StrikeScopeArCard, product.PriceIdr, product.StrikeOverride, time.Now())
	card.IsUnlocked = unlocked
	return nil
}

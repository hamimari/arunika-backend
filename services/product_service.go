package services

import (
	"arunika_backend/models"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type ProductService struct {
	db     *gorm.DB
	strike *StrikePriceService
}

func NewProductService(db *gorm.DB) *ProductService {
	return &ProductService{db: db}
}

// WithStrikePricing makes the admin product list report each product's
// effective strike price. Without it, only the stored override is shown.
func (s *ProductService) WithStrikePricing(strike *StrikePriceService) *ProductService {
	s.strike = strike
	return s
}

const (
	FeatureCodeArCard  = "AR_CARD"
	FeatureCodeDongeng = "DONGENG"
)

type CreateProductInput struct {
	FeatureCode string // FeatureCodeArCard | FeatureCodeDongeng
	PriceIdr    int64
	ArCardID    string     // required when FeatureCode == FeatureCodeArCard
	DongengID   *uuid.UUID // required when FeatureCode == FeatureCodeDongeng
	// PlayProductID is the Google Play SKU that sells this product; without
	// one it can't be bought through Google Play Billing.
	PlayProductID *string
}

// PlayProductUpdate says what an update does to a product's Google Play
// SKU. The zero value leaves it unchanged, so a client that doesn't know
// about the field can't clear a mapping by editing the price.
type PlayProductUpdate struct {
	Set bool
	SKU *string // nil clears the mapping
}

// normalizeSKU trims a Google Play SKU; blank means "no mapping".
func normalizeSKU(sku *string) *string {
	if sku == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*sku)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// ensurePlaySKUFree rejects a SKU already sold by another product or by a
// package: purchases and recoveries look the item up by SKU, so two items
// sharing one would be mixed up. exceptProductID excludes the product being
// edited.
func (s *ProductService) ensurePlaySKUFree(sku string, exceptProductID *uuid.UUID) error {
	q := s.db.Model(&models.Product{}).Where("play_product_id = ?", sku)
	if exceptProductID != nil {
		q = q.Where("id <> ?", *exceptProductID)
	}
	var products int64
	if err := q.Count(&products).Error; err != nil {
		return err
	}
	var packages int64
	if err := s.db.Model(&models.PremiumPackage{}).Where("play_product_id = ?", sku).Count(&packages).Error; err != nil {
		return err
	}
	if products+packages > 0 {
		return validationErrorf("play product id %q is already used by another product or package", sku)
	}
	return nil
}

// Create inserts a product and its single content mapping row in one transaction.
func (s *ProductService) Create(input CreateProductInput) (*models.Product, error) {
	feature, err := models.FindFeatureByCode(s.db, input.FeatureCode)
	if err != nil {
		return nil, err
	}
	sku := normalizeSKU(input.PlayProductID)
	if sku != nil {
		if err := s.ensurePlaySKUFree(*sku, nil); err != nil {
			return nil, err
		}
	}

	var product models.Product
	err = s.db.Transaction(func(tx *gorm.DB) error {
		product = models.Product{
			FeatureID:     feature.ID,
			PriceIdr:      input.PriceIdr,
			PlayProductID: sku,
			IsActive:      true,
		}
		if err := tx.Create(&product).Error; err != nil {
			return err
		}

		switch input.FeatureCode {
		case FeatureCodeArCard:
			if input.ArCardID == "" {
				return errors.New("ar_card_id is required for AR_CARD products")
			}
			return tx.Create(&models.ProductArCard{ProductID: product.ID, ArCardID: input.ArCardID}).Error
		case FeatureCodeDongeng:
			if input.DongengID == nil {
				return errors.New("dongeng_id is required for DONGENG products")
			}
			return tx.Create(&models.ProductDongeng{ProductID: product.ID, DongengID: *input.DongengID}).Error
		default:
			return errors.New("unsupported feature code")
		}
	})
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// List returns every product (admin use).
func (s *ProductService) List() ([]models.Product, error) {
	return models.FindAllProducts(s.db)
}

// AdminProductView is a Product enriched with the human-readable title of
// the AR card / dongeng it unlocks, its feature code, and the id of that
// underlying AR card / dongeng row (so the admin UI can look up its full
// detail on demand instead of needing it all inline).
type AdminProductView struct {
	ID          uuid.UUID `json:"id"`
	FeatureID   uuid.UUID `json:"feature_id"`
	FeatureCode string    `json:"feature_code"`
	DisplayName string    `json:"display_name"`
	ContentID   string    `json:"content_id"`
	PriceIdr    int64     `json:"price_idr"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// PlayProductID is the Google Play SKU selling this product (nil =
	// not purchasable through Google Play Billing).
	PlayProductID *string `json:"play_product_id"`
	// The stored strike-price override and the effective strike price it
	// resolves to right now (override, else the feature's global rule).
	models.StrikeOverride
	models.StrikeDisplay
}

// ListEnriched returns every product with its display name, feature code,
// and linked content id resolved (admin use).
func (s *ProductService) ListEnriched() ([]AdminProductView, error) {
	products, err := models.FindAllProducts(s.db)
	if err != nil {
		return nil, err
	}
	rules, err := s.strike.LoadRules()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	views := make([]AdminProductView, len(products))
	for i, p := range products {
		view := AdminProductView{
			ID:        p.ID,
			FeatureID: p.FeatureID,
			PriceIdr:  p.PriceIdr,
			IsActive:  p.IsActive,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,

			PlayProductID: p.PlayProductID,

			StrikeOverride: p.StrikeOverride,
		}
		var feature models.Feature
		if err := s.db.Where("id = ?", p.FeatureID).First(&feature).Error; err == nil {
			view.FeatureCode = feature.Code
			// Feature codes double as strike-price scopes (AR_CARD / DONGENG).
			view.StrikeDisplay = rules.Resolve(feature.Code, p.PriceIdr, p.StrikeOverride, now)
		}
		if name, err := s.ResolveDisplayName(p.ID); err == nil {
			view.DisplayName = name
		}
		view.ContentID = s.resolveContentID(p.ID)
		views[i] = view
	}
	return views, nil
}

// resolveContentID returns the id of the AR card or dongeng productID
// unlocks (whichever mapping row exists), or "" if neither does.
func (s *ProductService) resolveContentID(productID uuid.UUID) string {
	var pac models.ProductArCard
	if err := s.db.Where("product_id = ?", productID).First(&pac).Error; err == nil {
		return pac.ArCardID
	}
	var pd models.ProductDongeng
	if err := s.db.Where("product_id = ?", productID).First(&pd).Error; err == nil {
		return pd.DongengID.String()
	}
	return ""
}

// GetByID returns a single product by ID.
func (s *ProductService) GetByID(id uuid.UUID) (*models.Product, error) {
	return models.FindProductByID(s.db, id)
}

// Update changes a product's price and its strike-price override (a nil
// strike.Mode clears the override so the product inherits its scope's
// global rule). The content it unlocks (its AR card / dongeng mapping) is
// permanent and not editable.
func (s *ProductService) Update(id uuid.UUID, priceIdr int64, strike StrikeInput, play PlayProductUpdate) (*models.Product, error) {
	o, err := strike.ToOverride(time.Now())
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"price_idr":        priceIdr,
		"strike_mode":      o.StrikeMode,
		"strike_value":     o.StrikeValue,
		"strike_starts_at": o.StrikeStartsAt,
		"strike_ends_at":   o.StrikeEndsAt,
	}
	if play.Set {
		sku := normalizeSKU(play.SKU)
		if sku != nil {
			if err := s.ensurePlaySKUFree(*sku, &id); err != nil {
				return nil, err
			}
		}
		updates["play_product_id"] = sku
	}
	err = s.db.Model(&models.Product{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return nil, err
	}
	return models.FindProductByID(s.db, id)
}

// ToggleActive flips a product's is_active flag. Inactive products are hidden
// from the purchase flow for anyone who doesn't already own them, but users
// with an existing entitlement or subscription keep full access — see
// ArService.applyUnlocked / DongengService.computeUnlocked.
func (s *ProductService) ToggleActive(id uuid.UUID, isActive bool) error {
	return s.db.Model(&models.Product{}).Where("id = ?", id).Update("is_active", isActive).Error
}

// Delete removes a product and its content mapping row. Fails with a
// friendly error if the product is still referenced by a premium package
// (FK RESTRICT on premium_package_items.product_id).
func (s *ProductService) Delete(id uuid.UUID) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", id).Delete(&models.ProductArCard{}).Error; err != nil {
			return err
		}
		if err := tx.Where("product_id = ?", id).Delete(&models.ProductDongeng{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&models.Product{}).Error
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errors.New("product is still bundled in a premium package — remove it there first")
		}
		return err
	}
	return nil
}

// ResolveDisplayName returns the title of the AR card or dongeng productID
// unlocks, for use as the Midtrans line-item name on a single-product
// checkout. Resolved server-side (not trusted from the client) since it's
// shown on the Snap payment page.
func (s *ProductService) ResolveDisplayName(productID uuid.UUID) (string, error) {
	var title string
	err := s.db.Table("product_ar_cards pac").
		Select("a.title").
		Joins("JOIN ar_cards a ON a.id = pac.ar_card_id").
		Where("pac.product_id = ?", productID).
		Scan(&title).Error
	if err != nil {
		return "", err
	}
	if title != "" {
		return title, nil
	}

	err = s.db.Table("product_dongengs pd").
		Select("d.title").
		Joins("JOIN dongengs d ON d.id = pd.dongeng_id").
		Where("pd.product_id = ?", productID).
		Scan(&title).Error
	if err != nil {
		return "", err
	}
	if title == "" {
		return "", gorm.ErrRecordNotFound
	}
	return title, nil
}

// ResolveByArCardID returns the product linked to an AR card, or nil if it's free.
func (s *ProductService) ResolveByArCardID(arCardID string) (*models.Product, error) {
	return models.FindProductByArCardID(s.db, arCardID)
}

// ResolveByDongengID returns the product linked to a dongeng, or nil if it's free.
func (s *ProductService) ResolveByDongengID(dongengID uuid.UUID) (*models.Product, error) {
	return models.FindProductByDongengID(s.db, dongengID)
}

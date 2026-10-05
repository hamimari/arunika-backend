package services

import (
	"errors"
	"time"

	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Cart limits from the PRD: at most 20 items and Rp 500.000 per cart, so
// every total has a pre-registered store product.
const (
	CartMaxItems    = 20
	CartMaxTotalIdr = int64(500_000)
	// CartPriceStepIdr: every item price is a whole multiple of this.
	CartPriceStepIdr = int64(1_000)
	// CartPriceLock is how long an order's prices are honoured after the
	// parent taps Bayar.
	CartPriceLock = 30 * time.Minute
)

// Cart errors map one-to-one to the 409 codes the API returns.
var (
	ErrCartAlreadyOwned      = errors.New("ALREADY_OWNED")
	ErrCartNotForSale        = errors.New("NOT_FOR_SALE")
	ErrCartFreeItem          = errors.New("FREE_ITEM")
	ErrCartSubscription      = errors.New("SUBSCRIPTION_ACTIVE")
	ErrCartFull              = errors.New("CART_FULL")
	ErrCartTotalLimit        = errors.New("CART_TOTAL_LIMIT")
	ErrCartEmpty             = errors.New("CART_EMPTY")
	ErrCartAmountUnavailable = errors.New("AMOUNT_UNAVAILABLE")
)

// Notice codes explain why the server changed a cart.
const (
	NoticeAlreadyOwned = "ALREADY_OWNED"
	NoticeBecameFree   = "BECAME_FREE"
	NoticeSubscription = "SUBSCRIPTION_ACTIVE"
	NoticeUnavailable  = "NOT_FOR_SALE"
	NoticePriceUp      = "PRICE_UP"
)

type CartNotice struct {
	Code      string    `json:"code"`
	ProductID uuid.UUID `json:"product_id"`
	Title     string    `json:"title"`
	// FromIdr/ToIdr are set for PRICE_UP.
	FromIdr *int64 `json:"from_idr,omitempty"`
	ToIdr   *int64 `json:"to_idr,omitempty"`
}

type CartItemView struct {
	ProductID      uuid.UUID `json:"product_id"`
	ItemType       string    `json:"item_type"`
	ContentID      string    `json:"content_id"`
	Title          string    `json:"title"`
	ImageURL       string    `json:"image_url"`
	PriceIdr       int64     `json:"price_idr"`
	StrikePriceIdr *int64    `json:"strike_price_idr"`
	// PriceChangedFrom is the price the parent last saw, when the price
	// has gone up since.
	PriceChangedFrom *int64    `json:"price_changed_from"`
	Unavailable      bool      `json:"unavailable"`
	AddedAt          time.Time `json:"added_at"`
}

type CartView struct {
	Items          []CartItemView `json:"items"`
	ItemCount      int            `json:"item_count"`
	SubtotalIdr    int64          `json:"subtotal_idr"`
	PromoSavingIdr int64          `json:"promo_saving_idr"`
	TotalIdr       int64          `json:"total_idr"`
	Notices        []CartNotice   `json:"notices"`
}

// HasChanges reports whether the parent must look at the cart again before
// paying: something was removed or got more expensive. Unavailable rows are
// already shown greyed out and are simply left out of the order.
func (v *CartView) HasChanges() bool {
	if len(v.Notices) > 0 {
		return true
	}
	for _, it := range v.Items {
		if it.PriceChangedFrom != nil {
			return true
		}
	}
	return false
}

// payable returns the items that can be paid for.
func (v *CartView) payable() []CartItemView {
	out := make([]CartItemView, 0, len(v.Items))
	for _, it := range v.Items {
		if !it.Unavailable {
			out = append(out, it)
		}
	}
	return out
}

// totals fills the summary fields from Items. Unavailable items count
// toward neither the total nor the item count.
func (v *CartView) totals() {
	v.ItemCount, v.SubtotalIdr, v.PromoSavingIdr, v.TotalIdr = 0, 0, 0, 0
	for _, it := range v.payable() {
		v.ItemCount++
		normal := it.PriceIdr
		if it.StrikePriceIdr != nil && *it.StrikePriceIdr > it.PriceIdr {
			normal = *it.StrikePriceIdr
		}
		v.SubtotalIdr += normal
		v.TotalIdr += it.PriceIdr
	}
	v.PromoSavingIdr = v.SubtotalIdr - v.TotalIdr
}

// sellable is one product with everything the cart rules need.
type sellable struct {
	ProductID uuid.UUID
	ItemType  string
	ContentID string
	Title     string
	ImageURL  string
	PriceIdr  int64
	IsActive  bool
	IsFree    bool
	Hidden    bool
	models.StrikeOverride
}

type CartService struct {
	db           *gorm.DB
	entitlements *EntitlementService
	strike       *StrikePriceService
	flags        *FeatureFlagService
	now          func() time.Time
}

func NewCartService(db *gorm.DB, entitlements *EntitlementService, strike *StrikePriceService, flags *FeatureFlagService) *CartService {
	return &CartService{db: db, entitlements: entitlements, strike: strike, flags: flags, now: time.Now}
}

// Enabled reports whether the cart feature flag is on.
func (s *CartService) Enabled() bool {
	if s.flags == nil {
		return true
	}
	on, err := s.flags.IsEnabled("cart")
	return err == nil && on
}

func (s *CartService) loadSellables(db *gorm.DB, productIDs []uuid.UUID) (map[uuid.UUID]sellable, error) {
	out := make(map[uuid.UUID]sellable, len(productIDs))
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ProductID      uuid.UUID
		ItemType       string
		ContentID      string
		Title          string
		ImageURL       string
		PriceIdr       int64
		IsActive       bool
		IsFree         bool
		Hidden         bool
		StrikeMode     *string
		StrikeValue    *int
		StrikeStartsAt *time.Time
		StrikeEndsAt   *time.Time
	}
	err := db.Raw(`
		SELECT p.id AS product_id, 'ar_card' AS item_type, a.id::text AS content_id,
		       a.title, COALESCE(a.image_url, '') AS image_url, p.price_idr, p.is_active,
		       a.is_free, COALESCE(a.hidden, false) AS hidden,
		       p.strike_mode, p.strike_value, p.strike_starts_at, p.strike_ends_at
		  FROM products p
		  JOIN product_ar_cards pac ON pac.product_id = p.id
		  JOIN ar_cards a ON a.id = pac.ar_card_id
		 WHERE p.id IN ?
		UNION ALL
		SELECT p.id, 'dongeng', d.id::text, d.title, COALESCE(d.image_url, ''), p.price_idr, p.is_active,
		       d.is_free, COALESCE(d.hidden, false),
		       p.strike_mode, p.strike_value, p.strike_starts_at, p.strike_ends_at
		  FROM products p
		  JOIN product_dongengs pd ON pd.product_id = p.id
		  JOIN dongengs d ON d.id = pd.dongeng_id
		 WHERE p.id IN ?`, productIDs, productIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ProductID] = sellable{
			ProductID: r.ProductID, ItemType: r.ItemType, ContentID: r.ContentID,
			Title: r.Title, ImageURL: r.ImageURL, PriceIdr: r.PriceIdr,
			IsActive: r.IsActive, IsFree: r.IsFree, Hidden: r.Hidden,
			StrikeOverride: models.StrikeOverride{
				StrikeMode: r.StrikeMode, StrikeValue: r.StrikeValue,
				StrikeStartsAt: r.StrikeStartsAt, StrikeEndsAt: r.StrikeEndsAt,
			},
		}
	}
	return out, nil
}

func (s *CartService) ownedSet(db *gorm.DB, userID uuid.UUID, productIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	owned := map[uuid.UUID]bool{}
	if len(productIDs) == 0 {
		return owned, nil
	}
	var ids []uuid.UUID
	err := db.Model(&models.UserEntitlement{}).
		Where("user_id = ? AND product_id IN ? AND (expires_at IS NULL OR expires_at > NOW())", userID, productIDs).
		Pluck("product_id", &ids).Error
	for _, id := range ids {
		owned[id] = true
	}
	return owned, err
}

// checkAddable applies the "what can go in the cart" rules to one product.
func (s *CartService) checkAddable(db *gorm.DB, userID uuid.UUID, productID uuid.UUID) (sellable, error) {
	if sub, err := s.entitlements.HasActiveSubscription(userID); err != nil {
		return sellable{}, err
	} else if sub {
		return sellable{}, ErrCartSubscription
	}
	items, err := s.loadSellables(db, []uuid.UUID{productID})
	if err != nil {
		return sellable{}, err
	}
	it, ok := items[productID]
	if !ok || !it.IsActive || it.Hidden || it.PriceIdr <= 0 {
		return sellable{}, ErrCartNotForSale
	}
	if it.IsFree {
		return sellable{}, ErrCartFreeItem
	}
	owned, err := s.ownedSet(db, userID, []uuid.UUID{productID})
	if err != nil {
		return sellable{}, err
	}
	if owned[productID] {
		return sellable{}, ErrCartAlreadyOwned
	}
	return it, nil
}

// Add puts productID in the user's cart. Adding an item already in the
// cart is a no-op.
func (s *CartService) Add(userID, productID uuid.UUID) (*CartView, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Serialise adds per user so two devices can't both pass the limits.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "cart:"+userID.String()).Error; err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&models.CartItem{}).Where("user_id = ? AND product_id = ?", userID, productID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}
		it, err := s.checkAddable(tx, userID, productID)
		if err != nil {
			return err
		}
		var count int64
		var total int64
		if err := tx.Raw(`SELECT COUNT(*), COALESCE(SUM(p.price_idr), 0)
			FROM cart_items c JOIN products p ON p.id = c.product_id
			WHERE c.user_id = ? AND p.is_active`, userID).Row().Scan(&count, &total); err != nil {
			return err
		}
		if count >= CartMaxItems {
			return ErrCartFull
		}
		if total+it.PriceIdr > CartMaxTotalIdr {
			return ErrCartTotalLimit
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.CartItem{
			UserID: userID, ProductID: productID, SeenPriceIdr: it.PriceIdr, AddedAt: s.now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.Get(userID)
}

// Remove takes one product out of the cart; removing a missing item is fine.
func (s *CartService) Remove(userID, productID uuid.UUID) (*CartView, error) {
	if err := s.db.Where("user_id = ? AND product_id = ?", userID, productID).Delete(&models.CartItem{}).Error; err != nil {
		return nil, err
	}
	return s.Get(userID)
}

// Clear empties the cart ("Hapus semua").
func (s *CartService) Clear(userID uuid.UUID) error {
	return s.db.Where("user_id = ?", userID).Delete(&models.CartItem{}).Error
}

// Get returns the cart at current prices. Items that became owned, free or
// covered by Akses Premium are removed and reported in Notices; items no
// longer on sale stay, flagged Unavailable and left out of the total.
func (s *CartService) Get(userID uuid.UUID) (*CartView, error) {
	return s.read(s.db, userID)
}

func (s *CartService) read(db *gorm.DB, userID uuid.UUID) (*CartView, error) {
	var rows []models.CartItem
	if err := db.Where("user_id = ?", userID).Order("added_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	view := &CartView{Items: []CartItemView{}, Notices: []CartNotice{}}
	if len(rows) == 0 {
		return view, nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ProductID
	}
	sellables, err := s.loadSellables(db, ids)
	if err != nil {
		return nil, err
	}
	owned, err := s.ownedSet(db, userID, ids)
	if err != nil {
		return nil, err
	}
	subscribed, err := s.entitlements.HasActiveSubscription(userID)
	if err != nil {
		return nil, err
	}
	rules, err := s.strike.LoadRules()
	if err != nil {
		return nil, err
	}
	now := s.now()

	var drop []uuid.UUID
	for _, r := range rows {
		it, found := sellables[r.ProductID]
		switch {
		case subscribed:
			drop = append(drop, r.ProductID)
			view.Notices = append(view.Notices, CartNotice{Code: NoticeSubscription, ProductID: r.ProductID, Title: it.Title})
			continue
		case found && owned[r.ProductID]:
			drop = append(drop, r.ProductID)
			view.Notices = append(view.Notices, CartNotice{Code: NoticeAlreadyOwned, ProductID: r.ProductID, Title: it.Title})
			continue
		case found && it.IsFree:
			drop = append(drop, r.ProductID)
			view.Notices = append(view.Notices, CartNotice{Code: NoticeBecameFree, ProductID: r.ProductID, Title: it.Title})
			continue
		case !found:
			// The product or its content is gone; nothing left to show.
			drop = append(drop, r.ProductID)
			view.Notices = append(view.Notices, CartNotice{Code: NoticeUnavailable, ProductID: r.ProductID})
			continue
		}
		scope := models.StrikeScopeArCard
		if it.ItemType == models.ItemTypeDongeng {
			scope = models.StrikeScopeDongeng
		}
		strike := rules.Resolve(scope, it.PriceIdr, it.StrikeOverride, now)
		item := CartItemView{
			ProductID: r.ProductID, ItemType: it.ItemType, ContentID: it.ContentID,
			Title: it.Title, ImageURL: it.ImageURL, PriceIdr: it.PriceIdr,
			StrikePriceIdr: strike.StrikePriceIdr,
			Unavailable:    !it.IsActive || it.Hidden || it.PriceIdr <= 0,
			AddedAt:        r.AddedAt,
		}
		if it.PriceIdr > r.SeenPriceIdr {
			from := r.SeenPriceIdr
			item.PriceChangedFrom = &from
		}
		view.Items = append(view.Items, item)
	}
	if len(drop) > 0 {
		if err := db.Where("user_id = ? AND product_id IN ?", userID, drop).Delete(&models.CartItem{}).Error; err != nil {
			return nil, err
		}
	}
	view.totals()
	return view, nil
}

// acceptPrices records the current prices as seen, so a price rise is only
// reported once and a price drop is accepted silently.
func (s *CartService) acceptPrices(db *gorm.DB, userID uuid.UUID) error {
	return db.Exec(`UPDATE cart_items c SET seen_price_idr = p.price_idr
		FROM products p WHERE p.id = c.product_id AND c.user_id = ? AND c.seen_price_idr <> p.price_idr`, userID).Error
}

// silentlyAcceptDrops records price drops without asking the parent.
func (s *CartService) silentlyAcceptDrops(db *gorm.DB, userID uuid.UUID) error {
	return db.Exec(`UPDATE cart_items c SET seen_price_idr = p.price_idr
		FROM products p WHERE p.id = c.product_id AND c.user_id = ? AND p.price_idr < c.seen_price_idr`, userID).Error
}

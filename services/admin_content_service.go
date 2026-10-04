package services

import (
	"arunika_backend/models"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AdminContentService provides CRUD + visibility operations for all content types.
type AdminContentService struct {
	db *gorm.DB
}

func NewAdminContentService(db *gorm.DB) *AdminContentService {
	return &AdminContentService{db: db}
}

// ─── Fairy Tales ──────────────────────────────────────────────────────────────

func (s *AdminContentService) ListFairyTales(search string, page, perPage int) ([]AdminFairyTaleView, int64, error) {
	var items []models.Dongeng
	var total int64
	q := s.db.Model(&models.Dongeng{}).Where("is_deleted = false")
	if search != "" {
		q = q.Where("title ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	if err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, total, err
	}
	views, err := s.fairyTaleViews(items)
	return views, total, err
}

func (s *AdminContentService) GetFairyTale(id string) (*AdminFairyTaleView, error) {
	var item models.Dongeng
	if err := s.db.Preload("Pages").Where("id = ? AND is_deleted = false", id).First(&item).Error; err != nil {
		return &AdminFairyTaleView{Dongeng: item}, err
	}
	views, err := s.fairyTaleViews([]models.Dongeng{item})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *AdminContentService) CreateFairyTale(input models.Dongeng) (*models.Dongeng, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateFairyTale(id string, input models.Dongeng) (*models.Dongeng, error) {
	var item models.Dongeng
	if err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select(
		"title", "age_start", "age_end", "image_url", "audio_url",
		"is_free", "category_id", "duration",
		"dongeng_category_id", "dongeng_sub_category_id",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteFairyTale(id string) error {
	return s.db.Model(&models.Dongeng{}).Where("id = ?", id).Update("is_deleted", true).Error
}

// SetFairyTaleFree flips only the free flag; see SetArCardFree.
func (s *AdminContentService) SetFairyTaleFree(id string, isFree bool) error {
	return setContentFree(s.db, &models.Dongeng{}, id, isFree)
}

func (s *AdminContentService) ToggleFairyTaleVisibility(id string, hidden bool) error {
	return s.db.Model(&models.Dongeng{}).Where("id = ?", id).Update("hidden", hidden).Error
}

// ─── AR Cards ────────────────────────────────────────────────────────────────

func (s *AdminContentService) ListArCards(search string, page, perPage int) ([]AdminArCardView, int64, error) {
	var items []models.ArCards
	var total int64
	q := s.db.Model(&models.ArCards{})
	if search != "" {
		q = q.Where("title ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	if err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, total, err
	}
	views, err := s.arCardViews(items)
	return views, total, err
}

func (s *AdminContentService) GetArCard(id string) (*AdminArCardView, error) {
	var item models.ArCards
	if err := s.db.Where("id = ?", id).First(&item).Error; err != nil {
		return &AdminArCardView{ArCards: item}, err
	}
	views, err := s.arCardViews([]models.ArCards{item})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *AdminContentService) CreateArCard(input models.ArCards) (*models.ArCards, error) {
	input.ID = uuid.NewString()
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateArCard(id string, input models.ArCards) (*models.ArCards, error) {
	var item models.ArCards
	if err := s.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select(
		"type", "title", "file_url", "sound_url", "short_code", "image_url", "printable_img",
		"description", "category_id", "sub_category_id",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteArCard(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.ArCards{}).Error
}

// SetArCardFree flips only the free flag. The product, its orders and the
// entitlements of earlier buyers are left alone, so it is fully reversible.
func (s *AdminContentService) SetArCardFree(id string, isFree bool) error {
	return setContentFree(s.db, &models.ArCards{}, id, isFree)
}

func (s *AdminContentService) ToggleArCardVisibility(id string, hidden bool) error {
	return s.db.Model(&models.ArCards{}).Where("id = ?", id).Update("hidden", hidden).Error
}

// ─── Tracing Items ────────────────────────────────────────────────────────────

func (s *AdminContentService) ListTracingItems(search string, page, perPage int) ([]models.TracingItem, int64, error) {
	var items []models.TracingItem
	var total int64
	q := s.db.Model(&models.TracingItem{})
	if search != "" {
		q = q.Where("label ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&items).Error
	return items, total, err
}

func (s *AdminContentService) GetTracingItem(id string) (*models.TracingItem, error) {
	var item models.TracingItem
	err := s.db.Where("id = ?", id).First(&item).Error
	return &item, err
}

func (s *AdminContentService) CreateTracingItem(input models.TracingItem) (*models.TracingItem, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateTracingItem(id string, input models.TracingItem) (*models.TracingItem, error) {
	var item models.TracingItem
	if err := s.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select(
		"type", "label", "guide_path_json", "difficulty",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteTracingItem(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.TracingItem{}).Error
}

func (s *AdminContentService) ToggleTracingItemVisibility(id string, hidden bool) error {
	return s.db.Model(&models.TracingItem{}).Where("id = ?", id).Update("hidden", hidden).Error
}

// ─── Counting Questions ───────────────────────────────────────────────────────

func (s *AdminContentService) ListCountingQuestions(search string, page, perPage int) ([]models.CountingQuestion, int64, error) {
	var items []models.CountingQuestion
	var total int64
	q := s.db.Model(&models.CountingQuestion{})
	if search != "" {
		q = q.Where("level ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&items).Error
	return items, total, err
}

func (s *AdminContentService) GetCountingQuestion(id string) (*models.CountingQuestion, error) {
	var item models.CountingQuestion
	err := s.db.Where("id = ?", id).First(&item).Error
	return &item, err
}

func (s *AdminContentService) CreateCountingQuestion(input models.CountingQuestion) (*models.CountingQuestion, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateCountingQuestion(id string, input models.CountingQuestion) (*models.CountingQuestion, error) {
	var item models.CountingQuestion
	if err := s.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select(
		"level", "question_json", "answer",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteCountingQuestion(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.CountingQuestion{}).Error
}

func (s *AdminContentService) ToggleCountingQuestionVisibility(id string, hidden bool) error {
	return s.db.Model(&models.CountingQuestion{}).Where("id = ?", id).Update("hidden", hidden).Error
}

// ─── Badges ───────────────────────────────────────────────────────────────────

func (s *AdminContentService) ListBadges(search string, page, perPage int) ([]models.Badge, int64, error) {
	var items []models.Badge
	var total int64
	q := s.db.Model(&models.Badge{})
	if search != "" {
		q = q.Where("feature ILIKE ? OR level ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Limit(perPage).Offset((page - 1) * perPage).Find(&items).Error
	return items, total, err
}

func (s *AdminContentService) GetBadge(id string) (*models.Badge, error) {
	var item models.Badge
	err := s.db.Where("id = ?", id).First(&item).Error
	return &item, err
}

func (s *AdminContentService) CreateBadge(input models.Badge) (*models.Badge, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateBadge(id string, input models.Badge) (*models.Badge, error) {
	var item models.Badge
	if err := s.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select(
		"feature", "level", "threshold",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteBadge(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.Badge{}).Error
}

func (s *AdminContentService) ToggleBadgeVisibility(id string, hidden bool) error {
	return s.db.Model(&models.Badge{}).Where("id = ?", id).Update("hidden", hidden).Error
}

// ─── Categories ───────────────────────────────────────────────────────────────

func (s *AdminContentService) ListCategories(search string, page, perPage int) ([]models.Categories, int64, error) {
	var items []models.Categories
	var total int64
	q := s.db.Model(&models.Categories{}).Where("is_deleted = false")
	if search != "" {
		q = q.Where("name ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&items).Error
	return items, total, err
}

func (s *AdminContentService) GetCategory(id string) (*models.Categories, error) {
	var item models.Categories
	err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error
	return &item, err
}

func (s *AdminContentService) CreateCategory(input models.Categories) (*models.Categories, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateCategory(id string, input models.Categories) (*models.Categories, error) {
	var item models.Categories
	if err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select(
		"name", "image_url",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteCategory(id string) error {
	return s.db.Model(&models.Categories{}).Where("id = ?", id).Update("is_deleted", true).Error
}

func (s *AdminContentService) ToggleCategoryVisibility(id string, hidden bool) error {
	return s.db.Model(&models.Categories{}).Where("id = ?", id).Update("hidden", hidden).Error
}

// ─── Dongeng Pages ────────────────────────────────────────────────────────────

func (s *AdminContentService) ListDongengPages(dongengId string) ([]models.DongengPage, error) {
	return models.FindPagesByDongengId(s.db, dongengId)
}

func (s *AdminContentService) GetDongengPage(id string) (*models.DongengPage, error) {
	var item models.DongengPage
	err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error
	return &item, err
}

func (s *AdminContentService) CreateDongengPage(input models.DongengPage) (*models.DongengPage, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateDongengPage(id string, input models.DongengPage) (*models.DongengPage, error) {
	var item models.DongengPage
	if err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select("page_number", "image_url", "text", "audio_url").Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteDongengPage(id string) error {
	return s.db.Model(&models.DongengPage{}).Where("id = ?", id).Update("is_deleted", true).Error
}

// ─── AR Card Categories ───────────────────────────────────────────────────────

func (s *AdminContentService) ListArCardCategories() ([]models.ArCardCategory, error) {
	var items []models.ArCardCategory
	err := s.db.Order("sort_order ASC, created_at DESC").Find(&items).Error
	return items, err
}

func (s *AdminContentService) GetArCardCategory(id string) (*models.ArCardCategory, error) {
	var item models.ArCardCategory
	err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error
	return &item, err
}

func (s *AdminContentService) CreateArCardCategory(input models.ArCardCategory) (*models.ArCardCategory, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateArCardCategory(id string, input models.ArCardCategory) (*models.ArCardCategory, error) {
	var item models.ArCardCategory
	if err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select("name", "image_url", "parent_id", "sort_order").Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteArCardCategory(id string) error {
	return s.db.Model(&models.ArCardCategory{}).Where("id = ?", id).Update("is_deleted", true).Error
}

func (s *AdminContentService) ToggleArCardCategoryVisibility(id string, hidden bool) error {
	return s.db.Model(&models.ArCardCategory{}).Where("id = ?", id).Update("is_deleted", hidden).Error
}

// ─── Dongeng Categories ────────────────────────────────────────────────────────

func (s *AdminContentService) ListDongengCategories() ([]models.DongengCategory, error) {
	var items []models.DongengCategory
	err := s.db.Order("sort_order ASC, created_at DESC").Find(&items).Error
	return items, err
}

func (s *AdminContentService) GetDongengCategory(id string) (*models.DongengCategory, error) {
	var item models.DongengCategory
	err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error
	return &item, err
}

func (s *AdminContentService) CreateDongengCategory(input models.DongengCategory) (*models.DongengCategory, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *AdminContentService) UpdateDongengCategory(id string, input models.DongengCategory) (*models.DongengCategory, error) {
	var item models.DongengCategory
	if err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select("name", "image_url", "parent_id", "sort_order").Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteDongengCategory(id string) error {
	return s.db.Model(&models.DongengCategory{}).Where("id = ?", id).Update("is_deleted", true).Error
}

func (s *AdminContentService) ToggleDongengCategoryVisibility(id string, hidden bool) error {
	return s.db.Model(&models.DongengCategory{}).Where("id = ?", id).Update("is_deleted", hidden).Error
}

// ─── Content access (free / paid) ────────────────────────────────────────────

// Effective access of a piece of content, as shown to admins. Two things can
// make content free — the is_free flag and having no product — so the
// backoffice shows one computed value instead of both.
const (
	AccessFree          = "FREE"            // flagged free (a product may still exist)
	AccessFreeNoProduct = "FREE_NO_PRODUCT" // not flagged, but nothing is for sale
	AccessPaid          = "PAID"            // active product
	AccessPaidInactive  = "PAID_INACTIVE"   // product withdrawn from sale
)

// AdminArCardView is an AR card with its effective access. PriceIdr (from
// ArCards) carries the product's price whenever a product exists, including
// for a card flagged free, so the admin can see what it was sold for.
type AdminArCardView struct {
	models.ArCards
	Access string `json:"access"`
}

// AdminFairyTaleView is a dongeng with its effective access and product price.
type AdminFairyTaleView struct {
	models.Dongeng
	Access   string `json:"access"`
	PriceIdr *int64 `json:"price_idr,omitempty"`
}

type contentProduct struct {
	ContentID string
	PriceIdr  int64
	IsActive  bool
}

func accessFor(isFree bool, p *contentProduct) string {
	switch {
	case isFree:
		return AccessFree
	case p == nil:
		return AccessFreeNoProduct
	case p.IsActive:
		return AccessPaid
	default:
		return AccessPaidInactive
	}
}

// setContentFree updates is_free on the given content table (model is
// &models.ArCards{} or &models.Dongeng{}), erroring when the row is missing.
func setContentFree(db *gorm.DB, model interface{}, id string, isFree bool) error {
	res := db.Model(model).Where("id = ?", id).Update("is_free", isFree)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("not found")
	}
	return nil
}

func (s *AdminContentService) arCardViews(items []models.ArCards) ([]AdminArCardView, error) {
	views := make([]AdminArCardView, len(items))
	if len(items) == 0 {
		return views, nil
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	var rows []contentProduct
	err := s.db.Raw(`SELECT pac.ar_card_id AS content_id, p.price_idr, p.is_active
		FROM product_ar_cards pac JOIN products p ON p.id = pac.product_id
		WHERE pac.ar_card_id IN ?`, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*contentProduct, len(rows))
	for i := range rows {
		byID[rows[i].ContentID] = &rows[i]
	}
	for i, it := range items {
		p := byID[it.ID]
		if p != nil {
			price := p.PriceIdr
			it.PriceIdr = &price
		}
		views[i] = AdminArCardView{ArCards: it, Access: accessFor(it.IsFree, p)}
	}
	return views, nil
}

func (s *AdminContentService) fairyTaleViews(items []models.Dongeng) ([]AdminFairyTaleView, error) {
	views := make([]AdminFairyTaleView, len(items))
	if len(items) == 0 {
		return views, nil
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID.String()
	}
	var rows []contentProduct
	err := s.db.Raw(`SELECT pd.dongeng_id::text AS content_id, p.price_idr, p.is_active
		FROM product_dongengs pd JOIN products p ON p.id = pd.product_id
		WHERE pd.dongeng_id::text IN ?`, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*contentProduct, len(rows))
	for i := range rows {
		byID[rows[i].ContentID] = &rows[i]
	}
	for i, it := range items {
		p := byID[it.ID.String()]
		view := AdminFairyTaleView{Dongeng: it, Access: accessFor(it.IsFree, p)}
		if p != nil {
			price := p.PriceIdr
			view.PriceIdr = &price
		}
		views[i] = view
	}
	return views, nil
}

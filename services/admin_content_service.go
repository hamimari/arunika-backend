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

func (s *AdminContentService) ListFairyTales(search string, page, perPage int) ([]models.Dongeng, int64, error) {
	var items []models.Dongeng
	var total int64
	q := s.db.Model(&models.Dongeng{}).Where("is_deleted = false")
	if search != "" {
		q = q.Where("title ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&items).Error
	return items, total, err
}

func (s *AdminContentService) GetFairyTale(id string) (*models.Dongeng, error) {
	var item models.Dongeng
	err := s.db.Preload("Pages").Where("id = ? AND is_deleted = false", id).First(&item).Error
	return &item, err
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
		"is_free", "category_id", "duration", "hidden",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteFairyTale(id string) error {
	return s.db.Model(&models.Dongeng{}).Where("id = ?", id).Update("is_deleted", true).Error
}

func (s *AdminContentService) ToggleFairyTaleVisibility(id string, hidden bool) error {
	return s.db.Model(&models.Dongeng{}).Where("id = ?", id).Update("hidden", hidden).Error
}

// ─── AR Cards ────────────────────────────────────────────────────────────────

func (s *AdminContentService) ListArCards(search string, page, perPage int) ([]models.ArCards, int64, error) {
	var items []models.ArCards
	var total int64
	q := s.db.Model(&models.ArCards{})
	if search != "" {
		q = q.Where("title ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&items).Error
	return items, total, err
}

func (s *AdminContentService) GetArCard(id string) (*models.ArCards, error) {
	var item models.ArCards
	err := s.db.Where("id = ?", id).First(&item).Error
	return &item, err
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
		"type", "title", "file_url", "sound_url", "short_code", "hidden", "image_url", "printable_img",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *AdminContentService) DeleteArCard(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.ArCards{}).Error
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
		"type", "label", "guide_path_json", "difficulty", "hidden",
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
		"level", "question_json", "answer", "hidden",
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
		"feature", "level", "threshold", "hidden",
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
		"name", "image_url", "hidden",
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
	if err := s.db.Model(&item).Select("name", "emoji", "image_url", "parent_id", "sort_order").Updates(input).Error; err != nil {
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

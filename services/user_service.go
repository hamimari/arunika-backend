package services

import (
	"arunika_backend/models"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// SubscriptionDetail is the client-facing view of a user's active
// subscription — plan name (resolved from premium_packages, falling back to
// a generic label for admin manual-grants that have no package_id), status,
// and days remaining so the profile screen can show "N hari lagi" and a
// pay/extend CTA.
type SubscriptionDetail struct {
	PlanName  string     `json:"plan_name"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	DaysLeft  *int       `json:"days_left,omitempty"`
}

func (s *UserService) GetUserByID(id string) (*models.Parent, string, *SubscriptionDetail, error) {
	var user models.Parent
	if err := s.db.Preload("Children").First(&user, "id = ?", id).Error; err != nil {
		return nil, "", nil, err
	}

	// Fetch subscription status; default to "free" if not found or expired.
	// A lapsed subscription must not keep reporting "premium" — this is
	// consulted by the client to decide whether to show the premium upsell.
	var sub models.UserSubscription
	status := "free"
	var detail *SubscriptionDetail
	if err := s.db.Where("user_id = ?", id).First(&sub).Error; err == nil {
		if sub.Status == "premium" && (sub.ExpiresAt == nil || sub.ExpiresAt.After(time.Now())) {
			status = "premium"
			detail = s.buildSubscriptionDetail(&sub)
		}
	}

	// Ensure Children is never nil so JSON serialises as [] not null
	if user.Children == nil {
		user.Children = []models.Children{}
	}

	return &user, status, detail, nil
}

func (s *UserService) buildSubscriptionDetail(sub *models.UserSubscription) *SubscriptionDetail {
	planName := "Langganan Premium"
	if sub.PackageID != nil {
		var pkg models.PremiumPackage
		if err := s.db.Select("name").Where("id = ?", sub.PackageID.String()).First(&pkg).Error; err == nil && pkg.Name != "" {
			planName = pkg.Name
		}
	}

	detail := &SubscriptionDetail{
		PlanName:  planName,
		Status:    sub.Status,
		ExpiresAt: sub.ExpiresAt,
	}
	if sub.ExpiresAt != nil {
		days := int(math.Ceil(time.Until(*sub.ExpiresAt).Hours() / 24))
		if days < 0 {
			days = 0
		}
		detail.DaysLeft = &days
	}
	return detail
}

func (s *UserService) UpdateUser(req *models.Parent) (*models.Parent, error) {
	return s.updateUserTx(req)
}

func (s *UserService) updateUserTx(req *models.Parent) (*models.Parent, error) {
	var parent models.Parent

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Load parent + children
		if err := tx.Preload("Children").
			First(&parent, req.ID).Error; err != nil {
			return err
		}

		// 2. Update parent fields
		parent.Name = req.Name
		parent.PhoneNumber = req.PhoneNumber
		parent.EmailAddress = req.EmailAddress
		parent.City = req.City
		parent.Address = req.Address

		// 3. Diff children
		existing := make(map[uuid.UUID]*models.Children)
		for i := range parent.Children {
			child := &parent.Children[i]
			existing[child.ID] = child
		}

		var newChildren []models.Children
		var keepIDs []uuid.UUID

		for _, c := range req.Children {
			dob := c.DateOfBirth

			if c.ID != uuid.Nil {
				// update existing
				child, ok := existing[c.ID]
				if !ok {
					return fmt.Errorf("child %d not found", c.ID)
				}
				child.Name = c.Name
				child.DateOfBirth = dob
				child.Gender = c.Gender
				child.UpdatedAt = time.Now()
				keepIDs = append(keepIDs, c.ID)
				tx.Save(child)
			} else {
				// add new
				newChildren = append(newChildren, models.Children{
					ParentId:    parent.ID.String(),
					Name:        c.Name,
					DateOfBirth: dob,
					Gender:      c.Gender,
				})
			}
		}

		// 4. Delete removed children
		if err := tx.
			Where("parent_id = ? AND id NOT IN ?", parent.ID, keepIDs).
			Delete(&models.Children{}).Error; err != nil {
			return err
		}

		// 5. Insert new children
		if len(newChildren) > 0 {
			if err := tx.Create(&newChildren).Error; err != nil {
				return err
			}
		}

		// 6. Save parent
		return tx.Save(&parent).Error
	})

	if err != nil {
		return nil, err
	}

	return &parent, nil
}

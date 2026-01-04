package services

import (
	"arunika_backend/models"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetUserByID(id string) (*models.Parent, error) {
	var user models.Parent
	if err := s.db.Preload("Children").First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
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

package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Parent struct {
	BaseModel
	Name         string `json:"name"`
	PhoneNumber  string `json:"phone_number"`
	EmailAddress string `gorm:"unique" json:"email_address"`
	// Never serialized. handlers.userResponse embeds *Parent, so GET /user/:id
	// was returning the account's bcrypt hash to the client, which then
	// persisted it in the app's local profile storage. No client ever read
	// the field, and the only inbound JSON bind into Parent was SendOtp,
	// removed alongside the rest of the dead OTP flow.
	Password string     `json:"-"`
	Address  string     `json:"address"`
	City     string     `json:"city"`
	Children []Children `json:"children" gorm:"foreignKey:ParentId"`
	// EmailVerified reports whether the account holder has proven control of
	// EmailAddress. It gates password-reset delivery and nothing else —
	// entitlements key off the user ID, so an unverified account still has
	// access to everything it paid for.
	EmailVerified bool `gorm:"column:email_verified;not null;default:false" json:"email_verified"`
}

func FindUserByEmail(db *gorm.DB, email string) (*Parent, error) {
	var user Parent
	result := db.Where("email_address = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// FindUserById looks a user up by primary key — used when refreshing a
// session, where the refresh token is the only credential presented.
func FindUserById(db *gorm.DB, id string) (*Parent, error) {
	var user Parent
	result := db.Where("id = ?", id).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func FindUserByPhoneNumber(db *gorm.DB, phone string) (*Parent, error) {
	var user Parent
	result := db.Where("phone_number = ?", phone).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func CheckPassword(storedHash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	return err == nil
}

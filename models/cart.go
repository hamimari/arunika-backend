package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ItemTypeArCard  = "ar_card"
	ItemTypeDongeng = "dongeng"
)

// CartItem is one product in a parent's cart. An item is in a cart at most
// once (primary key user_id + product_id).
type CartItem struct {
	UserID       uuid.UUID `gorm:"column:user_id;type:uuid;primaryKey"`
	ProductID    uuid.UUID `gorm:"column:product_id;type:uuid;primaryKey"`
	SeenPriceIdr int64     `gorm:"column:seen_price_idr;not null"`
	AddedAt      time.Time `gorm:"column:added_at;not null;default:now()"`
}

func (CartItem) TableName() string { return "cart_items" }

// OrderItem is one product of a cart order, with its title and prices
// frozen when the order was created.
type OrderItem struct {
	OrderID   uuid.UUID `gorm:"column:order_id;type:uuid;primaryKey" json:"-"`
	ProductID uuid.UUID `gorm:"column:product_id;type:uuid;primaryKey" json:"product_id"`
	Title     string    `gorm:"column:title;not null"                 json:"title"`
	ItemType  string    `gorm:"column:item_type;not null"             json:"item_type"`
	NormalIdr int64     `gorm:"column:normal_idr;not null"            json:"normal_idr"`
	PriceIdr  int64     `gorm:"column:price_idr;not null"             json:"price_idr"`
}

func (OrderItem) TableName() string { return "order_items" }

const StoreGoogle = "google"

// StoreProduct is a consumable store product whose price is one possible
// cart total (arunika.cart.t<rupiah>).
type StoreProduct struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Store         string    `gorm:"column:store;not null"           json:"store"`
	PlayProductID string    `gorm:"column:play_product_id;not null" json:"play_product_id"`
	PriceIdr      int64     `gorm:"column:price_idr;not null"       json:"price_idr"`
	Active        bool      `gorm:"column:active;not null"          json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

func (StoreProduct) TableName() string { return "store_products" }

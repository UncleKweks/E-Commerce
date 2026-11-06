package models

import (
	"time"

	"gorm.io/gorm"
)

// Category groups related products.
type Category struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"uniqueIndex" json:"name"`
	Description string         `json:"description"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Products []Product `gorm:"foreignKey:CategoryID" json:"products,omitempty"`
}

// Product is an item for sale.
type Product struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"uniqueIndex" json:"name"`
	CategoryID  uint           `json:"category_id"`
	Description string         `json:"description"`
	Price       float64        `json:"price"`
	Stock       int            `json:"stock"`
	SKU         string         `gorm:"uniqueIndex" json:"sku"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Category   Category    `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	CartItems  []CartItem  `gorm:"foreignKey:ProductID" json:"cart_items,omitempty"`
	OrderItems []OrderItem `gorm:"foreignKey:ProductID" json:"order_items,omitempty"`
}

// ProductImage is an image attached to a product.
type ProductImage struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ProductID uint           `json:"product_id"`
	URL       string         `json:"url"`
	AltText   string         `json:"alt_text"`
	IsPrimary bool           `json:"is_primary"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Product Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}

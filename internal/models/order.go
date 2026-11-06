// Package models defines the database tables as GORM structs.
package models

import (
	"time"

	"gorm.io/gorm"
)

// Order is a customer's placed order.
type Order struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `json:"user_id"`
	Status    OrderStatus    `json:"status"`
	Total     float64        `json:"total"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User       User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	OrderItems []OrderItem `gorm:"foreignKey:OrderID" json:"order_items,omitempty"`
}

// OrderStatus is the lifecycle state of an order.
type OrderStatus string

// Order statuses.
const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusReturned  OrderStatus = "returned"
	OrderStatusFailed    OrderStatus = "Delivery Failed"
)

// OrderItem is one product line in an order, with the price paid at checkout.
type OrderItem struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	OrderID   uint           `json:"order_id"`
	ProductID uint           `json:"product_id"`
	Quantity  int            `json:"quantity"`
	Price     float64        `json:"price"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Order   Order   `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	Product Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}

// Cart is a user's shopping cart.
type Cart struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `json:"user_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User      User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CartItems []CartItem `gorm:"foreignKey:CartID" json:"cart_items,omitempty"`
}

// CartItem is one product line in a cart.
type CartItem struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CartID    uint           `json:"cart_id"`
	UserID    uint           `json:"user_id"`
	ProductID uint           `json:"product_id"`
	Quantity  int            `json:"quantity"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Product Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}

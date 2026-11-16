// Package models defines Go structs that map to the database tables.
// The tables themselves are defined by the SQL files in db/migrations.
package models

import (
	"time"
)

// Order is a customer's placed order. Orders are never deleted.
type Order struct {
	ID         uint        `gorm:"primaryKey" json:"id"`
	UserID     uint        `json:"user_id"`
	Status     OrderStatus `json:"status"`
	Currency   string      `json:"currency"` // ISO 4217 code, e.g. "USD"
	TotalCents int64       `json:"total_cents"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`

	// Relationships
	Items []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
}

// OrderStatus is the lifecycle state of an order.
type OrderStatus string

// Order statuses. These must match the orders_status_check constraint.
const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusRefunded  OrderStatus = "refunded"
)

// OrderItem is one product line in an order. Name and price are copied
// from the product at checkout, so they never change afterwards.
type OrderItem struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	OrderID        uint      `json:"order_id"`
	ProductID      uint      `json:"product_id"`
	ProductName    string    `json:"product_name"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	Quantity       int       `json:"quantity"`
	CreatedAt      time.Time `json:"created_at"`
}

// Cart is a user's shopping cart. Each user has at most one.
type Cart struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Items []CartItem `gorm:"foreignKey:CartID" json:"items,omitempty"`
}

// CartItem is one product line in a cart.
type CartItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CartID    uint      `json:"cart_id"`
	ProductID uint      `json:"product_id"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Product *Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}

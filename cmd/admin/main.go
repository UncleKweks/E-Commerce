// Command admin makes an existing user an admin (or back into a customer).
//
//	go run ./cmd/admin -email ada@example.com
//	go run ./cmd/admin -email ada@example.com -role customer
//
// There is deliberately no API endpoint for this: the first admin has to be
// created by someone with direct access to the server and database.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/config"
	"github.com/UncleKweks/E-Commerce/internal/database"
	"github.com/UncleKweks/E-Commerce/internal/models"
)

func main() {
	email := flag.String("email", "", "email of the user to change (required)")
	role := flag.String("role", string(models.UserRoleAdmin), "new role: admin or customer")
	flag.Parse()

	if err := run(strings.ToLower(strings.TrimSpace(*email)), models.UserRole(*role)); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(email string, role models.UserRole) error {
	if email == "" {
		return fmt.Errorf("-email is required, e.g. -email ada@example.com")
	}
	if role != models.UserRoleAdmin && role != models.UserRoleCustomer {
		return fmt.Errorf("-role must be admin or customer")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := database.New(&cfg.Database)
	if err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		defer func() { _ = sqlDB.Close() }()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res := db.WithContext(ctx).Model(&models.User{}).Where("email = ?", email).Update("role", role)
	if res.Error != nil {
		return fmt.Errorf("update role: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("no user with email %s; register first", email)
	}

	fmt.Printf("%s is now %s. Log in again to get a token with the new role.\n", email, role)
	return nil
}

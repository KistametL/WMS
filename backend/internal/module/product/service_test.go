package product

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestService creates a Service backed by a real database connection.
//
// The DSN is read from the TEST_DATABASE_URL environment variable so that
// credentials are never stored in source code (CWE-798).
//
// Set the variable before running:
//
//	TEST_DATABASE_URL="host=localhost port=5432 user=wms_user password=... dbname=wms_dev sslmode=disable" \
//	  go test ./internal/module/product/...
//
// In CI, set TEST_DATABASE_URL as a secret environment variable.
// If the variable is absent the test is skipped rather than failing.
func newTestService(t *testing.T) *Service {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewService(pool)
}

func TestCreateSKU(t *testing.T) {
	svc := newTestService(t)

	// Create a product first
	prod, err := svc.CreateProduct(context.Background(), CreateProductRequest{
		Name: "Test Product for SKU",
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	t.Logf("Created product: %s", prod.ID)

	f32000 := float64(32000)
	f38900 := float64(38900)
	f42900 := float64(42900)
	w187 := int32(187)

	// Use unique SKU code per test run to avoid conflicts with leftover data
	skuCode := fmt.Sprintf("TEST-SKU-%d", time.Now().UnixNano())

	sku, err := svc.CreateSKU(context.Background(), prod.ID, CreateSKURequest{
		SKUCode:        skuCode,
		Name:           "Test SKU",
		CostPrice:      f32000,
		SellingPrice:   f38900,
		CompareAtPrice: &f42900,
		WeightGrams:    &w187,
		Attributes:     json.RawMessage(`{"color":"Black"}`),
	})
	if err != nil {
		t.Fatalf("CreateSKU: %v", err)
	}
	t.Logf("Created SKU: %s code=%s", sku.ID, sku.SKUCode)

	// Cleanup — errors intentionally ignored (test data)
	if err := svc.DeleteSKU(context.Background(), sku.ID); err != nil {
		t.Logf("cleanup DeleteSKU: %v", err)
	}
	if err := svc.DeleteProduct(context.Background(), prod.ID); err != nil {
		t.Logf("cleanup DeleteProduct: %v", err)
	}
}

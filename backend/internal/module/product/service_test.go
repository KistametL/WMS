package product

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dsn := "host=localhost port=5432 user=wms_user password=f0xRxChlXg+uwkbLOfrgL/vF2biIk6qEVUFhkwhQCzE= dbname=wms_dev sslmode=disable"
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

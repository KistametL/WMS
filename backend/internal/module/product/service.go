package product

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrDuplicateSKU = errors.New("sku_code already exists")
)

type Service struct {
	queries *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{queries: db.New(pool)}
}

// ── Helpers ───────────────────────────────────────────────────────

func toText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func toInt4(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
}

func toBool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func toNumeric(f float64) pgtype.Numeric {
	n := pgtype.Numeric{}
	// pgtype.Numeric.Scan accepts string/[]byte only (not float64 directly)
	_ = n.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return n
}

func toNullNumeric(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	n := pgtype.Numeric{}
	_ = n.Scan(strconv.FormatFloat(*f, 'f', -1, 64))
	return n
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	return f.Float64
}

func nullNumericToFloat(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, _ := n.Float64Value()
	v := f.Float64
	return &v
}

func nullInt4ToInt32(n pgtype.Int4) *int32 {
	if !n.Valid {
		return nil
	}
	return &n.Int32
}

func nullTextToString(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// ── Categories ────────────────────────────────────────────────────

func (s *Service) ListCategories(ctx context.Context) ([]CategoryResponse, error) {
	rows, err := s.queries.ListCategories(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]CategoryResponse, len(rows))
	for i, r := range rows {
		result[i] = CategoryResponse{
			ID:          r.ID,
			Name:        r.Name,
			Description: nullTextToString(r.Description),
			ParentID:    nullInt4ToInt32(r.ParentID),
			IsActive:    r.IsActive,
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		}
	}
	return result, nil
}

func (s *Service) GetCategory(ctx context.Context, id int32) (*CategoryResponse, error) {
	row, err := s.queries.GetCategoryByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &CategoryResponse{
		ID:          row.ID,
		Name:        row.Name,
		Description: nullTextToString(row.Description),
		ParentID:    nullInt4ToInt32(row.ParentID),
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (s *Service) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*CategoryResponse, error) {
	row, err := s.queries.CreateCategory(ctx, db.CreateCategoryParams{
		Name:        req.Name,
		Description: toText(req.Description),
		ParentID:    toInt4(req.ParentID),
	})
	if err != nil {
		return nil, err
	}

	return &CategoryResponse{
		ID:          row.ID,
		Name:        row.Name,
		Description: nullTextToString(row.Description),
		ParentID:    nullInt4ToInt32(row.ParentID),
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (s *Service) UpdateCategory(ctx context.Context, id int32, req UpdateCategoryRequest) (*CategoryResponse, error) {
	row, err := s.queries.UpdateCategory(ctx, db.UpdateCategoryParams{
		ID:          id,
		Name:        toText(req.Name),
		Description: toText(req.Description),
		ParentID:    toInt4(req.ParentID),
		IsActive:    toBool(req.IsActive),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &CategoryResponse{
		ID:          row.ID,
		Name:        row.Name,
		Description: nullTextToString(row.Description),
		ParentID:    nullInt4ToInt32(row.ParentID),
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (s *Service) DeleteCategory(ctx context.Context, id int32) error {
	return s.queries.SoftDeleteCategory(ctx, id)
}

// ── Products ──────────────────────────────────────────────────────

func (s *Service) ListProducts(ctx context.Context, q ListProductsQuery) (*ListResponse[ProductResponse], error) {
	// Default pagination — capped to prevent int32 overflow (page*limit ≤ 100*maxInt32)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	// Safe: page ≤ maxInt32/100 (enforced by limit ≤ 100) so offset fits int32
	offset := int32((q.Page - 1) * q.Limit) //nolint:gosec

	filterParams := db.CountProductsParams{
		Search:     toText(nilIfEmpty(q.Search)),
		CategoryID: toInt4(q.CategoryID),
		IsActive:   toBool(q.IsActive),
	}

	total, err := s.queries.CountProducts(ctx, filterParams)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListProducts(ctx, db.ListProductsParams{
		Search:     filterParams.Search,
		CategoryID: filterParams.CategoryID,
		IsActive:   filterParams.IsActive,
		Limit:      int32(q.Limit), //nolint:gosec // capped at 100 above
		Offset:     offset,
	})
	if err != nil {
		return nil, err
	}

	items := make([]ProductResponse, len(rows))
	for i, r := range rows {
		items[i] = ProductResponse{
			ID:          r.ID.String(),
			CategoryID:  nullInt4ToInt32(r.CategoryID),
			Name:        r.Name,
			Description: nullTextToString(r.Description),
			IsActive:    r.IsActive,
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		}
	}

	return &ListResponse[ProductResponse]{
		Items: items,
		Total: total,
		Page:  q.Page,
		Limit: q.Limit,
	}, nil
}

func (s *Service) GetProduct(ctx context.Context, id string) (*ProductDetailResponse, error) {
	pgID, err := parseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	row, err := s.queries.GetProductByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	skus, err := s.listSKUs(ctx, pgID)
	if err != nil {
		return nil, err
	}

	return &ProductDetailResponse{
		ProductResponse: ProductResponse{
			ID:          row.ID.String(),
			CategoryID:  nullInt4ToInt32(row.CategoryID),
			Name:        row.Name,
			Description: nullTextToString(row.Description),
			IsActive:    row.IsActive,
			CreatedAt:   row.CreatedAt.Time,
			UpdatedAt:   row.UpdatedAt.Time,
		},
		SKUs: skus,
	}, nil
}

func (s *Service) CreateProduct(ctx context.Context, req CreateProductRequest) (*ProductResponse, error) {
	row, err := s.queries.CreateProduct(ctx, db.CreateProductParams{
		CategoryID:  toInt4(req.CategoryID),
		Name:        req.Name,
		Description: toText(req.Description),
	})
	if err != nil {
		return nil, err
	}

	return &ProductResponse{
		ID:          row.ID.String(),
		CategoryID:  nullInt4ToInt32(row.CategoryID),
		Name:        row.Name,
		Description: nullTextToString(row.Description),
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (s *Service) UpdateProduct(ctx context.Context, id string, req UpdateProductRequest) (*ProductResponse, error) {
	pgID, err := parseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	row, err := s.queries.UpdateProduct(ctx, db.UpdateProductParams{
		ID:          pgID,
		CategoryID:  toInt4(req.CategoryID),
		Name:        toText(req.Name),
		Description: toText(req.Description),
		IsActive:    toBool(req.IsActive),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &ProductResponse{
		ID:          row.ID.String(),
		CategoryID:  nullInt4ToInt32(row.CategoryID),
		Name:        row.Name,
		Description: nullTextToString(row.Description),
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	pgID, err := parseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	return s.queries.SoftDeleteProduct(ctx, pgID)
}

// ── SKUs ──────────────────────────────────────────────────────────

func (s *Service) listSKUs(ctx context.Context, productID pgtype.UUID) ([]SKUResponse, error) {
	rows, err := s.queries.ListSKUsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}

	result := make([]SKUResponse, len(rows))
	for i, r := range rows {
		result[i] = toSKUResponse(r)
	}
	return result, nil
}

func (s *Service) CreateSKU(ctx context.Context, productID string, req CreateSKURequest) (*SKUResponse, error) {
	pgProductID, err := parseUUID(productID)
	if err != nil {
		return nil, ErrNotFound
	}

	var attrs []byte
	if req.Attributes != nil {
		attrs = req.Attributes
	}

	row, err := s.queries.CreateSKU(ctx, db.CreateSKUParams{
		ProductID:      pgProductID,
		SkuCode:        req.SKUCode,
		Name:           req.Name,
		CostPrice:      toNumeric(req.CostPrice),
		SellingPrice:   toNumeric(req.SellingPrice),
		CompareAtPrice: toNullNumeric(req.CompareAtPrice),
		WeightGrams:    toInt4(req.WeightGrams),
		Attributes:     attrs,
	})
	if err != nil {
		if isDuplicateError(err) {
			return nil, ErrDuplicateSKU
		}
		return nil, err
	}

	res := toSKUResponse(row)
	return &res, nil
}

func (s *Service) UpdateSKU(ctx context.Context, id string, req UpdateSKURequest) (*SKUResponse, error) {
	pgID, err := parseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	var attrs []byte
	if req.Attributes != nil {
		attrs = req.Attributes
	}

	row, err := s.queries.UpdateSKU(ctx, db.UpdateSKUParams{
		ID:             pgID,
		SkuCode:        toText(req.SKUCode),
		Name:           toText(req.Name),
		CostPrice:      toNullNumeric(req.CostPrice),
		SellingPrice:   toNullNumeric(req.SellingPrice),
		CompareAtPrice: toNullNumeric(req.CompareAtPrice),
		WeightGrams:    toInt4(req.WeightGrams),
		Attributes:     attrs,
		IsActive:       toBool(req.IsActive),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isDuplicateError(err) {
			return nil, ErrDuplicateSKU
		}
		return nil, err
	}

	res := toSKUResponse(row)
	return &res, nil
}

func (s *Service) DeleteSKU(ctx context.Context, id string) error {
	pgID, err := parseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	return s.queries.SoftDeleteSKU(ctx, pgID)
}

// ── Internal helpers ──────────────────────────────────────────────

func toSKUResponse(r interface{}) SKUResponse {
	switch v := r.(type) {
	case db.CreateSKURow:
		return skuRowToResponse(v.ID, v.ProductID, v.SkuCode, v.Name,
			v.CostPrice, v.SellingPrice, v.CompareAtPrice,
			v.WeightGrams, v.Attributes, v.IsActive, v.CreatedAt, v.UpdatedAt)
	case db.UpdateSKURow:
		return skuRowToResponse(v.ID, v.ProductID, v.SkuCode, v.Name,
			v.CostPrice, v.SellingPrice, v.CompareAtPrice,
			v.WeightGrams, v.Attributes, v.IsActive, v.CreatedAt, v.UpdatedAt)
	case db.ListSKUsByProductRow:
		return skuRowToResponse(v.ID, v.ProductID, v.SkuCode, v.Name,
			v.CostPrice, v.SellingPrice, v.CompareAtPrice,
			v.WeightGrams, v.Attributes, v.IsActive, v.CreatedAt, v.UpdatedAt)
	case db.GetSKUByIDRow:
		return skuRowToResponse(v.ID, v.ProductID, v.SkuCode, v.Name,
			v.CostPrice, v.SellingPrice, v.CompareAtPrice,
			v.WeightGrams, v.Attributes, v.IsActive, v.CreatedAt, v.UpdatedAt)
	}
	return SKUResponse{}
}

func skuRowToResponse(
	id, productID pgtype.UUID,
	skuCode, name string,
	costPrice, sellingPrice, compareAtPrice pgtype.Numeric,
	weightGrams pgtype.Int4,
	attributes []byte,
	isActive bool,
	createdAt, updatedAt pgtype.Timestamptz,
) SKUResponse {
	return SKUResponse{
		ID:             id.String(),
		ProductID:      productID.String(),
		SKUCode:        skuCode,
		Name:           name,
		CostPrice:      numericToFloat(costPrice),
		SellingPrice:   numericToFloat(sellingPrice),
		CompareAtPrice: nullNumericToFloat(compareAtPrice),
		WeightGrams:    nullInt4ToInt32(weightGrams),
		Attributes:     attributes,
		IsActive:       isActive,
		CreatedAt:      createdAt.Time,
		UpdatedAt:      updatedAt.Time,
	}
}

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(s)
	return id, err
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isDuplicateError(err error) bool {
	return err != nil && len(err.Error()) > 0 &&
		containsString(err.Error(), "duplicate key")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && findString(s, substr))
}

func findString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

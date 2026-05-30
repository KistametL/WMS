package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
	"github.com/KistametL/WMS/backend/internal/pgutil"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrStockNotFound     = errors.New("stock level not initialized for this SKU")
	ErrInsufficientStock = errors.New("insufficient stock: qty_on_hand would go below qty_reserved")
	ErrNegativeStock     = errors.New("qty_on_hand cannot be negative")
)

type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:    pool,
		queries: db.New(pool),
	}
}

// ── Stock Levels ──────────────────────────────────────────────────

func (s *Service) GetStockBySKU(ctx context.Context, skuID string) (*StockLevelResponse, error) {
	pgID, err := pgutil.ParseUUID(skuID)
	if err != nil {
		return nil, ErrNotFound
	}

	row, err := s.queries.GetStockBySKU(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStockNotFound
		}
		return nil, err
	}

	return stockRowToResponse(row), nil
}

func (s *Service) ListStock(ctx context.Context, q ListStockQuery) (*ListResponse[StockLevelResponse], error) {
	page, limit, offset := pgutil.NormalizePagination(q.Page, q.Limit)

	var lowStockOnly pgtype.Bool
	if q.LowStockOnly != nil {
		lowStockOnly = pgtype.Bool{Bool: *q.LowStockOnly, Valid: true}
	}

	total, err := s.queries.CountStockLevels(ctx, lowStockOnly)
	if err != nil {
		return nil, err
	}

	// Safe: limit ≤ 100, offset = (page-1)*limit ≤ ~100*maxInt32, fits int32
	offset32 := int32(offset) //nolint:gosec // G115: bounded by normalizePagination
	limit32 := int32(limit)   //nolint:gosec // G115: capped at 100 above

	rows, err := s.queries.ListStockLevels(ctx, db.ListStockLevelsParams{
		LowStockOnly: lowStockOnly,
		Offset:       pgtype.Int4{Int32: offset32, Valid: true},
		Limit:        pgtype.Int4{Int32: limit32, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	items := make([]StockLevelResponse, len(rows))
	for i, r := range rows {
		items[i] = listStockRowToResponse(r)
	}

	return &ListResponse[StockLevelResponse]{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *Service) UpdateLowStockThreshold(ctx context.Context, skuID string, threshold int32) (*StockLevelResponse, error) {
	pgID, err := pgutil.ParseUUID(skuID)
	if err != nil {
		return nil, ErrNotFound
	}

	level, err := s.queries.UpdateLowStockThreshold(ctx, db.UpdateLowStockThresholdParams{
		SkuID:             pgID,
		LowStockThreshold: threshold,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStockNotFound
		}
		return nil, err
	}

	return stockLevelToResponse(level, "", ""), nil
}

// ── Stock Movements ───────────────────────────────────────────────

// CreateMovement สร้าง movement และ update stock level ภายใน transaction เดียวกัน
//
// การทำงาน:
//  1. Begin transaction
//  2. InitStockLevel — สร้าง stock_level ถ้ายังไม่มี (idempotent)
//  3. GetStockBySKUForUpdate — lock row (SELECT FOR UPDATE)
//  4. คำนวณ qty ใหม่ + validate
//  5. UpdateStockLevel — update qty
//  6. CreateStockMovement — บันทึก movement
//  7. Commit
func (s *Service) CreateMovement(ctx context.Context, req CreateMovementRequest, userID string) (*MovementResponse, error) {
	skuPgID, err := pgutil.ParseUUID(req.SKUID)
	if err != nil {
		return nil, fmt.Errorf("invalid sku_id: %w", err)
	}

	var userPgID pgtype.UUID
	if userID != "" {
		if parsed, parseErr := pgutil.ParseUUID(userID); parseErr == nil {
			userPgID = parsed
		}
	}

	var movement db.InventoryStockMovement

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// 1. สร้าง stock level ถ้ายังไม่มี (qty=0 เป็น default)
		if _, err := q.InitStockLevel(ctx, skuPgID); err != nil {
			return fmt.Errorf("init stock level: %w", err)
		}

		// 2. Lock row เพื่อป้องกัน race condition
		level, err := q.GetStockBySKUForUpdate(ctx, skuPgID)
		if err != nil {
			return fmt.Errorf("get stock for update: %w", err)
		}

		qtyBefore := level.QtyOnHand
		newOnHand, newReserved, qtyChange, err := calculateNewQty(level, req)
		if err != nil {
			return err
		}

		// 3. Update stock level
		if _, err := q.UpdateStockLevel(ctx, db.UpdateStockLevelParams{
			SkuID:       skuPgID,
			QtyOnHand:   newOnHand,
			QtyReserved: newReserved,
		}); err != nil {
			return fmt.Errorf("update stock level: %w", err)
		}

		// 4. บันทึก movement record
		movement, err = q.CreateStockMovement(ctx, db.CreateStockMovementParams{
			SkuID:         skuPgID,
			MovementType:  req.MovementType,
			QtyChange:     qtyChange,
			QtyBefore:     qtyBefore,
			QtyAfter:      newOnHand,
			ReferenceType: toText(req.ReferenceType),
			ReferenceID:   toText(req.ReferenceID),
			Note:          toText(req.Note),
			CreatedBy:     userPgID,
		})
		if err != nil {
			return fmt.Errorf("create movement record: %w", err)
		}

		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, ErrInsufficientStock) || errors.Is(txErr, ErrNegativeStock) {
			return nil, txErr
		}
		return nil, txErr
	}

	return movementToResponse(movement), nil
}

func (s *Service) GetMovement(ctx context.Context, id int64) (*MovementResponse, error) {
	m, err := s.queries.GetStockMovement(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return movementToResponse(m), nil
}

func (s *Service) ListMovements(ctx context.Context, q ListMovementsQuery) (*ListResponse[MovementResponse], error) {
	page, limit, offset := pgutil.NormalizePagination(q.Page, q.Limit)

	var skuPgID pgtype.UUID
	if q.SKUID != "" {
		if parsed, err := pgutil.ParseUUID(q.SKUID); err == nil {
			skuPgID = parsed
		}
	}

	var movementType pgtype.Text
	if q.MovementType != "" {
		movementType = pgtype.Text{String: q.MovementType, Valid: true}
	}

	total, err := s.queries.CountStockMovements(ctx, db.CountStockMovementsParams{
		SkuID:        skuPgID,
		MovementType: movementType,
	})
	if err != nil {
		return nil, err
	}

	// Safe: limit ≤ 100, offset = (page-1)*limit ≤ ~100*maxInt32, fits int32
	offset32 := int32(offset) //nolint:gosec // G115: bounded by normalizePagination
	limit32 := int32(limit)   //nolint:gosec // G115: capped at 100 above

	rows, err := s.queries.ListStockMovements(ctx, db.ListStockMovementsParams{
		SkuID:        skuPgID,
		MovementType: movementType,
		Offset:       pgtype.Int4{Int32: offset32, Valid: true},
		Limit:        pgtype.Int4{Int32: limit32, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	items := make([]MovementResponse, len(rows))
	for i, r := range rows {
		items[i] = *movementToResponse(r)
	}

	return &ListResponse[MovementResponse]{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// ── Business Logic ────────────────────────────────────────────────

// calculateNewQty คำนวณ qty ใหม่และ validate ตาม movement type
// คืนค่า (newOnHand, newReserved, qtyChange, error)
//
// ตาราง logic:
//
//	receive: qty_on_hand += qty  (qty_reserved ไม่เปลี่ยน)
//	adjust:  qty_on_hand = qty   (absolute; qty_change = qty - old_on_hand)
//	damage:  qty_on_hand -= qty  (qty_reserved ต้องไม่เกิน on_hand ใหม่)
//	return:  qty_on_hand += qty  (เหมือน receive)
func calculateNewQty(level db.InventoryStockLevel, req CreateMovementRequest) (newOnHand, newReserved, qtyChange int32, err error) {
	oldOnHand := level.QtyOnHand
	reserved := level.QtyReserved

	switch req.MovementType {
	case MovementReceive, MovementReturn:
		newOnHand = oldOnHand + req.Qty
		newReserved = reserved
		qtyChange = req.Qty

	case MovementAdjust:
		if req.Qty < 0 {
			return 0, 0, 0, ErrNegativeStock
		}
		if req.Qty < reserved {
			// ไม่สามารถ adjust ให้ต่ำกว่าจำนวนที่ reserve ไว้แล้ว
			return 0, 0, 0, fmt.Errorf("%w: cannot adjust below qty_reserved (%d)", ErrInsufficientStock, reserved)
		}
		qtyChange = req.Qty - oldOnHand // อาจเป็นลบถ้า adjust ลง
		newOnHand = req.Qty
		newReserved = reserved

	case MovementDamage:
		newOnHand = oldOnHand - req.Qty
		if newOnHand < 0 {
			return 0, 0, 0, fmt.Errorf("%w: cannot damage %d units when only %d on hand", ErrNegativeStock, req.Qty, oldOnHand)
		}
		if newOnHand < reserved {
			return 0, 0, 0, fmt.Errorf("%w: damaging %d units would leave qty_on_hand (%d) below qty_reserved (%d)",
				ErrInsufficientStock, req.Qty, newOnHand, reserved)
		}
		newReserved = reserved
		qtyChange = -req.Qty

	default:
		// ไม่ควร reach ถึงนี้ เพราะ binding:"oneof=..." validate ไว้แล้ว
		return 0, 0, 0, fmt.Errorf("unsupported movement_type: %s", req.MovementType)
	}

	return newOnHand, newReserved, qtyChange, nil
}

// ── Converters ────────────────────────────────────────────────────

func stockRowToResponse(r db.GetStockBySKURow) *StockLevelResponse {
	available := r.QtyAvailable.Int32
	return &StockLevelResponse{
		ID:                r.ID,
		SKUID:             r.SkuID.String(),
		SKUCode:           r.SkuCode,
		SKUName:           r.SkuName,
		QtyOnHand:         r.QtyOnHand,
		QtyReserved:       r.QtyReserved,
		QtyAvailable:      available,
		LowStockThreshold: r.LowStockThreshold,
		IsLowStock:        available <= r.LowStockThreshold,
		UpdatedAt:         r.UpdatedAt.Time,
	}
}

func listStockRowToResponse(r db.ListStockLevelsRow) StockLevelResponse {
	available := r.QtyAvailable.Int32
	return StockLevelResponse{
		ID:                r.ID,
		SKUID:             r.SkuID.String(),
		SKUCode:           r.SkuCode,
		SKUName:           r.SkuName,
		QtyOnHand:         r.QtyOnHand,
		QtyReserved:       r.QtyReserved,
		QtyAvailable:      available,
		LowStockThreshold: r.LowStockThreshold,
		IsLowStock:        available <= r.LowStockThreshold,
		UpdatedAt:         r.UpdatedAt.Time,
	}
}

// stockLevelToResponse ใช้ตอน UpdateLowStockThreshold ที่ไม่มี sku_code/sku_name
// ข้อมูลที่ขาดจะ empty string — client ควร call GET ถ้าต้องการ full info
func stockLevelToResponse(l db.InventoryStockLevel, skuCode, skuName string) *StockLevelResponse {
	available := l.QtyAvailable.Int32
	return &StockLevelResponse{
		ID:                l.ID,
		SKUID:             l.SkuID.String(),
		SKUCode:           skuCode,
		SKUName:           skuName,
		QtyOnHand:         l.QtyOnHand,
		QtyReserved:       l.QtyReserved,
		QtyAvailable:      available,
		LowStockThreshold: l.LowStockThreshold,
		IsLowStock:        available <= l.LowStockThreshold,
		UpdatedAt:         l.UpdatedAt.Time,
	}
}

func movementToResponse(m db.InventoryStockMovement) *MovementResponse {
	r := &MovementResponse{
		ID:           m.ID,
		SKUID:        m.SkuID.String(),
		MovementType: m.MovementType,
		QtyChange:    m.QtyChange,
		QtyBefore:    m.QtyBefore,
		QtyAfter:     m.QtyAfter,
		CreatedAt:    m.CreatedAt.Time,
	}
	if m.ReferenceType.Valid {
		r.ReferenceType = &m.ReferenceType.String
	}
	if m.ReferenceID.Valid {
		r.ReferenceID = &m.ReferenceID.String
	}
	if m.Note.Valid {
		r.Note = &m.Note.String
	}
	if m.CreatedBy.Valid {
		s := pgutil.UUIDString(m.CreatedBy)
		r.CreatedBy = &s
	}
	return r
}

// ── Helpers ───────────────────────────────────────────────────────

func toText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

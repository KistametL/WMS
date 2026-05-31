package fulfillment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
	"github.com/KistametL/WMS/backend/internal/pgutil"
)

// ── Status constants (mirror of order module) ─────────────────────
// ไม่ import จาก order package เพื่อหลีกเลี่ยง circular dependency
const (
	statusConfirmed   = "confirmed"
	statusPicking     = "picking"
	statusPacking     = "packing"
	statusReadyToShip = "ready_to_ship"
	statusShipped     = "shipped"
)

// ── Service ───────────────────────────────────────────────────────

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

// ── Queue ─────────────────────────────────────────────────────────

// GetQueue returns a paginated list of orders for a given fulfillment status.
// Default status: "confirmed" (orders waiting to be picked).
func (s *Service) GetQueue(ctx context.Context, q QueueQuery) (*ListResponse[FulfillmentOrderResponse], error) {
	status := q.Status
	if status == "" {
		status = statusConfirmed
	}

	page, limit, offset := pgutil.NormalizePagination(q.Page, q.Limit)

	total, err := s.queries.CountFulfillmentOrders(ctx, status)
	if err != nil {
		return nil, err
	}

	offset32 := int32(offset) //nolint:gosec // G115: bounded by NormalizePagination
	limit32 := int32(limit)   //nolint:gosec // G115: capped at 100

	rows, err := s.queries.ListFulfillmentOrders(ctx, db.ListFulfillmentOrdersParams{
		Status: status,
		Offset: pgtype.Int4{Int32: offset32, Valid: true},
		Limit:  pgtype.Int4{Int32: limit32, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	items := make([]FulfillmentOrderResponse, len(rows))
	for i, r := range rows {
		items[i] = listRowToResponse(r)
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (int(total) + limit - 1) / limit
	}

	return &ListResponse[FulfillmentOrderResponse]{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// ── Workflow Transitions ──────────────────────────────────────────

// StartPick transitions order from confirmed → picking.
// SELECT FOR UPDATE ป้องกัน 2 staff กด start-pick order เดียวกันพร้อมกัน
func (s *Service) StartPick(ctx context.Context, id, userID string) (*TransitionResponse, error) {
	return s.simpleTransition(ctx, id, statusConfirmed, statusPicking, userID)
}

// ConfirmPick validates scanned items then transitions picking → packing.
// ตรวจ exact match: ทุก item ใน order ต้อง scan ครบ และ quantity ตรง
func (s *Service) ConfirmPick(ctx context.Context, id string, req ConfirmPickRequest, userID string) (*TransitionResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	userPgID := parseUserID(userID)

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// ── Lock + validate status ─────────────────────────────────────
		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("lock order row: %w", err)
		}
		if locked.Status != statusPicking {
			return fmt.Errorf("%w: expected picking, got %s", ErrInvalidTransition, locked.Status)
		}

		// ── Validate scanned items vs order items ──────────────────────
		items, err := q.ListOrderItems(ctx, pgID)
		if err != nil {
			return fmt.Errorf("list order items: %w", err)
		}

		// expected: sku_code → required quantity
		expected := make(map[string]int32, len(items))
		for _, item := range items {
			expected[item.SkuCode] += item.Quantity
		}

		// scanned: sku_code → scanned quantity
		scanned := make(map[string]int32, len(req.ScannedItems))
		for _, s := range req.ScannedItems {
			scanned[s.SKUCode] += s.Quantity
		}

		// ตรวจ: ทุก item ที่ order ต้องการต้อง scan ครบถ้วน
		for code, qty := range expected {
			if scanned[code] != qty {
				return fmt.Errorf("%w: sku %s expected qty %d, scanned %d",
					ErrScanMismatch, code, qty, scanned[code])
			}
		}
		// ตรวจ: ไม่มี item แปลกปลอมที่ไม่อยู่ใน order
		for code := range scanned {
			if _, ok := expected[code]; !ok {
				return fmt.Errorf("%w: sku %s not in order", ErrScanMismatch, code)
			}
		}

		// ── Transition ────────────────────────────────────────────────
		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     pgID,
			Status: statusPacking,
		}); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    pgID,
			ToStatus:   statusPacking,
			FromStatus: pgtype.Text{String: statusPicking, Valid: true},
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	return &TransitionResponse{OrderID: id, Status: statusPacking}, nil
}

// ConfirmPack transitions order from packing → ready_to_ship.
func (s *Service) ConfirmPack(ctx context.Context, id, userID string) (*TransitionResponse, error) {
	return s.simpleTransition(ctx, id, statusPacking, statusReadyToShip, userID)
}

// ── Ship ──────────────────────────────────────────────────────────

// Ship atomically:
//  1. Locks order row (SELECT FOR UPDATE)
//  2. Validates status is ready_to_ship
//  3. Deducts stock (qty_on_hand - qty, qty_reserved - qty) for every item
//  4. Creates stock_movement "fulfill" for every item
//  5. Updates order status → shipped
//  6. Creates status history record
//  7. Creates shipment record (courier + tracking)
//
// ทุกอย่างใน TX เดียว — ป้องกัน partial failure (shipped แต่ไม่มี shipment record)
func (s *Service) Ship(ctx context.Context, id string, req ShipRequest, userID string) (*ShipmentResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	userPgID := parseUserID(userID)
	orderUUID := id

	var result *ShipmentResponse

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// ── Lock + validate status ─────────────────────────────────────
		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("lock order row: %w", err)
		}
		if locked.Status != statusReadyToShip {
			return fmt.Errorf("%w: expected ready_to_ship, got %s", ErrInvalidTransition, locked.Status)
		}

		// ── Idempotency: ตรวจ shipment ซ้ำ ───────────────────────────
		// UNIQUE constraint บน order_id ใน DB จะ catch ได้อยู่แล้ว
		// แต่ดักก่อนเพื่อ error message ที่ชัดกว่า
		_, errCheck := q.GetShipmentByOrderID(ctx, pgID)
		if errCheck == nil {
			return ErrAlreadyShipped
		}
		if !errors.Is(errCheck, pgx.ErrNoRows) {
			return fmt.Errorf("check existing shipment: %w", errCheck)
		}

		// ── List items ─────────────────────────────────────────────────
		items, err := q.ListOrderItems(ctx, pgID)
		if err != nil {
			return fmt.Errorf("list order items: %w", err)
		}

		// ── Fulfill stock ──────────────────────────────────────────────
		for _, item := range items {
			level, err := q.GetStockBySKUForUpdate(ctx, item.SkuID)
			if err != nil {
				return fmt.Errorf("lock stock for sku %s: %w", item.SkuCode, err)
			}

			newOnHand := level.QtyOnHand - item.Quantity
			newReserved := level.QtyReserved - item.Quantity
			if newOnHand < 0 {
				newOnHand = 0
			}
			if newReserved < 0 {
				newReserved = 0
			}

			if _, err := q.UpdateStockLevel(ctx, db.UpdateStockLevelParams{
				SkuID:       item.SkuID,
				QtyOnHand:   newOnHand,
				QtyReserved: newReserved,
			}); err != nil {
				return fmt.Errorf("fulfill stock for sku %s: %w", item.SkuCode, err)
			}

			if _, err := q.CreateStockMovement(ctx, db.CreateStockMovementParams{
				SkuID:         item.SkuID,
				MovementType:  "fulfill",
				QtyChange:     -item.Quantity,
				QtyBefore:     level.QtyOnHand,
				QtyAfter:      newOnHand,
				ReferenceType: pgtype.Text{String: "order", Valid: true},
				ReferenceID:   pgtype.Text{String: orderUUID, Valid: true},
				CreatedBy:     userPgID,
			}); err != nil {
				return fmt.Errorf("create fulfill movement for sku %s: %w", item.SkuCode, err)
			}
		}

		// ── Update order status ────────────────────────────────────────
		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     pgID,
			Status: statusShipped,
		}); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		if err := q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    pgID,
			ToStatus:   statusShipped,
			FromStatus: pgtype.Text{String: statusReadyToShip, Valid: true},
			ChangedBy:  userPgID,
		}); err != nil {
			return fmt.Errorf("create status history: %w", err)
		}

		// ── Create shipment record ─────────────────────────────────────
		shipment, err := q.CreateShipment(ctx, db.CreateShipmentParams{
			OrderID:        pgID,
			Courier:        req.Courier,
			TrackingNumber: req.TrackingNumber,
			LabelUrl:       pgutil.ToText(req.LabelURL),
			ShippedBy:      userPgID,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrAlreadyShipped
			}
			return fmt.Errorf("create shipment: %w", err)
		}

		result = shipmentToResponse(shipment)
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	return result, nil
}

// ── Shipment ──────────────────────────────────────────────────────

// GetShipment returns the shipment record for an order.
func (s *Service) GetShipment(ctx context.Context, orderID string) (*ShipmentResponse, error) {
	pgID, err := pgutil.ParseUUID(orderID)
	if err != nil {
		return nil, ErrNotFound
	}

	shipment, err := s.queries.GetShipmentByOrderID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return shipmentToResponse(shipment), nil
}

// ── Internal Helpers ──────────────────────────────────────────────

// simpleTransition: lock → validate from-status → update → history
// ใช้สำหรับ transitions ที่ไม่มี stock side effects (start-pick, confirm-pack)
func (s *Service) simpleTransition(ctx context.Context, id, from, to, userID string) (*TransitionResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	userPgID := parseUserID(userID)

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("lock order row: %w", err)
		}
		if locked.Status != from {
			return fmt.Errorf("%w: expected %s, got %s (concurrent request?)",
				ErrInvalidTransition, from, locked.Status)
		}

		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     pgID,
			Status: to,
		}); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    pgID,
			ToStatus:   to,
			FromStatus: pgtype.Text{String: from, Valid: true},
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	return &TransitionResponse{OrderID: id, Status: to}, nil
}

// parseUserID แปลง userID string → pgtype.UUID (invalid UUID ถ้าว่าง)
func parseUserID(userID string) pgtype.UUID {
	if userID == "" {
		return pgtype.UUID{}
	}
	parsed, err := pgutil.ParseUUID(userID)
	if err != nil {
		return pgtype.UUID{}
	}
	return parsed
}

// ── Response Converters ───────────────────────────────────────────

func listRowToResponse(r db.ListFulfillmentOrdersRow) FulfillmentOrderResponse {
	resp := FulfillmentOrderResponse{
		ID:          pgutil.UUIDString(r.ID),
		OrderNumber: r.OrderNumber,
		Channel:     r.Channel,
		Status:      r.Status,
		Total:       pgutil.NumericToFloat(r.Total),
		IsCOD:       r.IsCod,
		CODAmount:   pgutil.NumericToFloat(r.CodAmount),
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
	if r.CustomerName.Valid {
		resp.CustomerName = &r.CustomerName.String
	}
	if r.CustomerPhone.Valid {
		resp.CustomerPhone = &r.CustomerPhone.String
	}
	if r.Note.Valid {
		resp.Note = &r.Note.String
	}
	return resp
}

func shipmentToResponse(s db.FulfillmentShipment) *ShipmentResponse {
	resp := &ShipmentResponse{
		ID:             pgutil.UUIDString(s.ID),
		OrderID:        pgutil.UUIDString(s.OrderID),
		Courier:        s.Courier,
		TrackingNumber: s.TrackingNumber,
		ShippedAt:      s.ShippedAt.Time,
		CreatedAt:      s.CreatedAt.Time,
	}
	if s.LabelUrl.Valid {
		resp.LabelURL = &s.LabelUrl.String
	}
	if s.ShippedBy.Valid {
		str := pgutil.UUIDString(s.ShippedBy)
		resp.ShippedBy = &str
	}
	return resp
}

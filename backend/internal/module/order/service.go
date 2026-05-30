package order

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
	"github.com/KistametL/WMS/backend/internal/pgutil"
)

// ── Sentinel Errors ───────────────────────────────────────────────

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrOrderNotEditable  = errors.New("order can only be edited in pending or confirmed status")
	ErrInsufficientStock = errors.New("insufficient stock")
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

// ── Create ────────────────────────────────────────────────────────

// CreateOrder creates a new order with items in a single transaction.
//
// Flow:
//  1. Validate all SKUs exist and are active (read-only, outside TX)
//  2. Calculate subtotal / total
//  3. BEGIN TX:
//     a. nextval(order_number_seq) → unique order number
//     b. INSERT order
//     c. INSERT each item
//     d. INSERT initial status_history (→ pending)
func (s *Service) CreateOrder(ctx context.Context, req CreateOrderRequest, userID string) (*OrderDetailResponse, error) {
	// ── 1. Validate SKUs ─────────────────────────────────────────────
	type skuInfo struct {
		Code string
		Name string
	}
	skuMap := make(map[string]skuInfo, len(req.Items))
	for _, item := range req.Items {
		pgID, err := pgutil.ParseUUID(item.SKUID)
		if err != nil {
			return nil, fmt.Errorf("invalid sku_id %q: %w", item.SKUID, err)
		}
		sku, err := s.queries.GetSKUByID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("sku %q not found", item.SKUID)
			}
			return nil, err
		}
		if !sku.IsActive {
			return nil, fmt.Errorf("sku %q is inactive", sku.SkuCode)
		}
		skuMap[item.SKUID] = skuInfo{Code: sku.SkuCode, Name: sku.Name}
	}

	// ── 2. Calculate totals ──────────────────────────────────────────
	var subtotal float64
	for _, item := range req.Items {
		lineTotal := (item.UnitPrice * float64(item.Quantity)) - item.DiscountAmount
		subtotal += lineTotal
	}
	total := subtotal + req.ShippingCost - req.DiscountTotal
	if total < 0 {
		total = 0
	}

	var userPgID pgtype.UUID
	if userID != "" {
		if parsed, err := pgutil.ParseUUID(userID); err == nil {
			userPgID = parsed
		}
	}

	// ── 3. Transaction ───────────────────────────────────────────────
	var createdOrder db.CreateOrderRow
	var createdItems []db.CreateOrderItemRow

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		orderNum, err := generateOrderNumber(ctx, tx)
		if err != nil {
			return err
		}

		// ── Re-validate SKUs inside TX (catch deactivation race) ─────────
		// SKU อาจถูก deactivate ระหว่าง pre-check (step 1) กับ INSERT จริง
		// ทำซ้ำภายใน TX เพื่อ guarantee ว่า SKU ยังคง active ณ เวลา insert
		for _, item := range req.Items {
			pgSkuID, _ := pgutil.ParseUUID(item.SKUID)
			sku, err := q.GetSKUByID(ctx, pgSkuID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("sku %q was deleted after validation", item.SKUID)
				}
				return err
			}
			if !sku.IsActive {
				return fmt.Errorf("sku %q was deactivated after validation", sku.SkuCode)
			}
		}

		createdOrder, err = q.CreateOrder(ctx, db.CreateOrderParams{
			OrderNumber:     orderNum,
			Channel:         req.Channel,
			ChannelOrderID:  toText(req.ChannelOrderID),
			CustomerName:    toText(req.CustomerName),
			CustomerPhone:   toText(req.CustomerPhone),
			ShippingAddress: req.ShippingAddress,
			ShippingMethod:  toText(req.ShippingMethod),
			ShippingCost:    toNumeric(req.ShippingCost),
			IsCod:           req.IsCOD,
			CodAmount:       toNumeric(req.CODAmount),
			Subtotal:        toNumeric(subtotal),
			DiscountTotal:   toNumeric(req.DiscountTotal),
			Total:           toNumeric(total),
			Note:            toText(req.Note),
			CreatedBy:       userPgID,
		})
		if err != nil {
			return fmt.Errorf("create order: %w", err)
		}

		createdItems = make([]db.CreateOrderItemRow, 0, len(req.Items))
		for _, item := range req.Items {
			info := skuMap[item.SKUID]
			pgSkuID, _ := pgutil.ParseUUID(item.SKUID) // validated above
			lineTotal := (item.UnitPrice * float64(item.Quantity)) - item.DiscountAmount
			created, err := q.CreateOrderItem(ctx, db.CreateOrderItemParams{
				OrderID:        createdOrder.ID,
				SkuID:          pgSkuID,
				SkuCode:        info.Code,
				Name:           info.Name,
				Quantity:       item.Quantity,
				UnitPrice:      toNumeric(item.UnitPrice),
				TotalPrice:     toNumeric(lineTotal),
				DiscountAmount: toNumeric(item.DiscountAmount),
			})
			if err != nil {
				return fmt.Errorf("create order item: %w", err)
			}
			createdItems = append(createdItems, created)
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    createdOrder.ID,
			ToStatus:   StatusPending,
			FromStatus: pgtype.Text{},
			Note:       pgtype.Text{},
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	return createOrderToDetail(createdOrder, createdItems), nil
}

// ── Get ───────────────────────────────────────────────────────────

func (s *Service) GetOrder(ctx context.Context, id string) (*OrderDetailResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	order, err := s.queries.GetOrderByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	items, err := s.queries.ListOrderItems(ctx, pgID)
	if err != nil {
		return nil, err
	}

	history, err := s.queries.ListStatusHistory(ctx, pgID)
	if err != nil {
		return nil, err
	}

	return orderDetailToResponse(order, items, history), nil
}

// ── List ──────────────────────────────────────────────────────────

func (s *Service) ListOrders(ctx context.Context, q ListOrdersQuery) (*ListResponse[OrderResponse], error) {
	page, limit, offset := pgutil.NormalizePagination(q.Page, q.Limit)

	countParams := db.CountOrdersParams{
		Status:  optText(q.Status),
		Channel: optText(q.Channel),
		Search:  optText(q.Search),
	}
	if q.IsCOD != nil {
		countParams.IsCod = pgtype.Bool{Bool: *q.IsCOD, Valid: true}
	}

	total, err := s.queries.CountOrders(ctx, countParams)
	if err != nil {
		return nil, err
	}

	// Safe: limit ≤ 100, offset = (page-1)*limit — fits int32
	offset32 := int32(offset) //nolint:gosec // G115: bounded by NormalizePagination
	limit32 := int32(limit)   //nolint:gosec // G115: capped at 100 above

	rows, err := s.queries.ListOrders(ctx, db.ListOrdersParams{
		Status:  countParams.Status,
		Channel: countParams.Channel,
		IsCod:   countParams.IsCod,
		Search:  countParams.Search,
		Offset:  pgtype.Int4{Int32: offset32, Valid: true},
		Limit:   pgtype.Int4{Int32: limit32, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	items := make([]OrderResponse, len(rows))
	for i, r := range rows {
		items[i] = listRowToResponse(r)
	}

	return &ListResponse[OrderResponse]{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// ── Update Info ───────────────────────────────────────────────────

// UpdateOrder แก้ไข customer/shipping/note ได้เฉพาะ pending หรือ confirmed
// total ถูก recalculate อัตโนมัติใน SQL เมื่อ shipping_cost หรือ discount_total เปลี่ยน
func (s *Service) UpdateOrder(ctx context.Context, id string, req UpdateOrderRequest) (*OrderResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	existing, err := s.queries.GetOrderByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if existing.Status != StatusPending && existing.Status != StatusConfirmed {
		return nil, ErrOrderNotEditable
	}

	params := db.UpdateOrderParams{
		ID:             pgID,
		CustomerName:   toText(req.CustomerName),
		CustomerPhone:  toText(req.CustomerPhone),
		ShippingMethod: toText(req.ShippingMethod),
		Note:           toText(req.Note),
	}
	if len(req.ShippingAddress) > 0 {
		params.ShippingAddress = req.ShippingAddress
	}
	if req.ShippingCost != nil {
		params.ShippingCost = toNumeric(*req.ShippingCost)
	}
	if req.DiscountTotal != nil {
		params.DiscountTotal = toNumeric(*req.DiscountTotal)
	}

	updated, err := s.queries.UpdateOrder(ctx, params)
	if err != nil {
		return nil, err
	}

	return updateRowToResponse(updated), nil
}

// ── Status Transitions ────────────────────────────────────────────

// UpdateStatus validates the state machine and applies the transition.
//
//	confirmed → reserve stock (SELECT FOR UPDATE prevents double-reservation)
//	shipped   → fulfill stock (deduct on_hand, create "fulfill" movements)
//	cancelled → unreserve stock if was in reserved status
//	others    → simple status update + history record
func (s *Service) UpdateStatus(ctx context.Context, id string, req UpdateStatusRequest, userID string) (*OrderDetailResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	existing, err := s.queries.GetOrderByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if !isValidTransition(existing.Status, req.Status) {
		return nil, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, existing.Status, req.Status)
	}

	var userPgID pgtype.UUID
	if userID != "" {
		if parsed, err := pgutil.ParseUUID(userID); err == nil {
			userPgID = parsed
		}
	}

	switch req.Status {
	case StatusCancelled:
		return s.cancelWithLock(ctx, pgID, existing.Status, req.Note, userPgID)
	case StatusConfirmed:
		return s.confirmWithStockReserve(ctx, pgID, existing.Status, req.Note, userPgID)
	case StatusShipped:
		return s.fulfillAndShip(ctx, pgID, existing.Status, req.Note, userPgID)
	default:
		return s.applyStatusUpdate(ctx, pgID, existing.Status, req.Status, req.Note, pgtype.Text{}, userPgID)
	}
}

// CancelOrder is the dedicated cancel endpoint.
// รองรับ reason field ที่ละเอียดกว่า UpdateStatus
func (s *Service) CancelOrder(ctx context.Context, id string, req CancelOrderRequest, userID string) (*OrderDetailResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	existing, err := s.queries.GetOrderByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if !isValidTransition(existing.Status, StatusCancelled) {
		return nil, fmt.Errorf("%w: %s → cancelled", ErrInvalidTransition, existing.Status)
	}

	var userPgID pgtype.UUID
	if userID != "" {
		if parsed, err := pgutil.ParseUUID(userID); err == nil {
			userPgID = parsed
		}
	}

	return s.cancelWithLock(ctx, pgID, existing.Status, req.Reason, userPgID)
}

// ── Internal Transaction Helpers ──────────────────────────────────

// confirmWithStockReserve reserves qty_reserved for every item then sets status = confirmed.
//
// Race condition protection (C1 fix):
//   - GetOrderByIDForUpdate → SELECT FOR UPDATE on orders row ก่อนทำอะไร
//   - ถ้า concurrent request confirm พร้อมกัน → อีก TX รอ lock → พอได้ lock
//     พบว่า status ไม่ใช่ pending อีกแล้ว → return error ทันที
//
// C2 fix: ListOrderItems อยู่ภายใน TX แล้ว
//
// M4 fix: สร้าง stock_movement type "reserve" ทุก item
func (s *Service) confirmWithStockReserve(ctx context.Context, pgID pgtype.UUID, fromStatus string, note *string, userPgID pgtype.UUID) (*OrderDetailResponse, error) {
	orderUUID := pgutil.UUIDString(pgID)

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// ── Lock orders row (C1: ป้องกัน double-reservation) ──────────
		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			return fmt.Errorf("lock order row: %w", err)
		}
		if locked.Status != StatusPending {
			return fmt.Errorf("%w: order is already %s (concurrent request?)", ErrInvalidTransition, locked.Status)
		}

		// ── List items ภายใน TX (C2 fix) ─────────────────────────────
		items, err := q.ListOrderItems(ctx, pgID)
		if err != nil {
			return fmt.Errorf("list order items: %w", err)
		}

		// ── Reserve stock สำหรับทุก item ──────────────────────────────
		for _, item := range items {
			level, err := q.GetStockBySKUForUpdate(ctx, item.SkuID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: no stock record for sku %s — create a receive movement first",
						ErrInsufficientStock, item.SkuCode)
				}
				return err
			}
			available := level.QtyOnHand - level.QtyReserved
			if available < item.Quantity {
				return fmt.Errorf("%w: sku %s needs %d but only %d available (on_hand=%d reserved=%d)",
					ErrInsufficientStock, item.SkuCode, item.Quantity, available,
					level.QtyOnHand, level.QtyReserved)
			}

			newReserved := level.QtyReserved + item.Quantity
			if _, err := q.UpdateStockLevel(ctx, db.UpdateStockLevelParams{
				SkuID:       item.SkuID,
				QtyOnHand:   level.QtyOnHand,
				QtyReserved: newReserved,
			}); err != nil {
				return fmt.Errorf("reserve stock for sku %s: %w", item.SkuCode, err)
			}

			// ── M4: บันทึก reserve movement เพื่อ audit trail ────────
			if _, err := q.CreateStockMovement(ctx, db.CreateStockMovementParams{
				SkuID:         item.SkuID,
				MovementType:  "reserve",
				QtyChange:     item.Quantity,
				QtyBefore:     level.QtyReserved,
				QtyAfter:      newReserved,
				ReferenceType: pgtype.Text{String: "order", Valid: true},
				ReferenceID:   pgtype.Text{String: orderUUID, Valid: true},
				CreatedBy:     userPgID,
			}); err != nil {
				return fmt.Errorf("create reserve movement for sku %s: %w", item.SkuCode, err)
			}
		}

		// ── Update status ──────────────────────────────────────────────
		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     pgID,
			Status: StatusConfirmed,
		}); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    pgID,
			ToStatus:   StatusConfirmed,
			FromStatus: pgtype.Text{String: fromStatus, Valid: true},
			Note:       toText(note),
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	return s.GetOrder(ctx, pgutil.UUIDString(pgID))
}

// fulfillAndShip deducts qty_on_hand and qty_reserved for every item
// then transitions to shipped.  This is the "physical fulfillment" step.
//
// ทำไมต้องทำตอน shipped (ไม่ใช่ delivered):
//   - "shipped" = สินค้าออกจาก warehouse แล้ว — stock ควรลดทันที
//   - "delivered" = ลูกค้าได้รับแล้ว — ไม่ใช่จุดที่ stock เปลี่ยน
//
// M2 fix: จาก code review — เดิม stock ไม่ถูกตัดเลย
// M4 fix: สร้าง stock_movement type "fulfill" ทุก item
func (s *Service) fulfillAndShip(ctx context.Context, pgID pgtype.UUID, fromStatus string, note *string, userPgID pgtype.UUID) (*OrderDetailResponse, error) {
	orderUUID := pgutil.UUIDString(pgID)

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// ── Lock orders row ────────────────────────────────────────────
		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			return fmt.Errorf("lock order row: %w", err)
		}
		if locked.Status != fromStatus {
			return fmt.Errorf("%w: order status changed to %s before ship could complete",
				ErrInvalidTransition, locked.Status)
		}

		// ── List items ภายใน TX ────────────────────────────────────────
		items, err := q.ListOrderItems(ctx, pgID)
		if err != nil {
			return fmt.Errorf("list order items: %w", err)
		}

		// ── Fulfill stock สำหรับทุก item ──────────────────────────────
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

			// ── M4: บันทึก fulfill movement ───────────────────────────
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

		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     pgID,
			Status: StatusShipped,
		}); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    pgID,
			ToStatus:   StatusShipped,
			FromStatus: pgtype.Text{String: fromStatus, Valid: true},
			Note:       toText(note),
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	return s.GetOrder(ctx, pgutil.UUIDString(pgID))
}

// cancelWithLock locks the order row, unreserves stock (when needed),
// then transitions to cancelled.
//
// C1 fix: GetOrderByIDForUpdate ล็อก orders row ก่อน
// C2 fix: ListOrderItems อยู่ภายใน TX
// M4 fix: สร้าง stock_movement type "unreserve" ทุก item
func (s *Service) cancelWithLock(ctx context.Context, pgID pgtype.UUID, fromStatus string, reason *string, userPgID pgtype.UUID) (*OrderDetailResponse, error) {
	orderUUID := pgutil.UUIDString(pgID)
	needsUnreserve := reservedStatuses[fromStatus]

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// ── Lock orders row (C1 fix) ───────────────────────────────────
		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			return fmt.Errorf("lock order row: %w", err)
		}
		if locked.Status != fromStatus {
			return fmt.Errorf("%w: order status changed to %s (concurrent request?)",
				ErrInvalidTransition, locked.Status)
		}

		// ── Unreserve stock ภายใน TX (C2 fix) ────────────────────────
		if needsUnreserve {
			items, err := q.ListOrderItems(ctx, pgID)
			if err != nil {
				return fmt.Errorf("list order items: %w", err)
			}
			for _, item := range items {
				level, err := q.GetStockBySKUForUpdate(ctx, item.SkuID)
				if err != nil {
					return fmt.Errorf("lock stock for sku %s: %w", item.SkuCode, err)
				}
				newReserved := level.QtyReserved - item.Quantity
				if newReserved < 0 {
					newReserved = 0
				}
				if _, err := q.UpdateStockLevel(ctx, db.UpdateStockLevelParams{
					SkuID:       item.SkuID,
					QtyOnHand:   level.QtyOnHand,
					QtyReserved: newReserved,
				}); err != nil {
					return fmt.Errorf("unreserve stock for sku %s: %w", item.SkuCode, err)
				}

				// ── M4: บันทึก unreserve movement ─────────────────────
				if _, err := q.CreateStockMovement(ctx, db.CreateStockMovementParams{
					SkuID:         item.SkuID,
					MovementType:  "unreserve",
					QtyChange:     -item.Quantity,
					QtyBefore:     level.QtyReserved,
					QtyAfter:      newReserved,
					ReferenceType: pgtype.Text{String: "order", Valid: true},
					ReferenceID:   pgtype.Text{String: orderUUID, Valid: true},
					Note:          toText(reason),
					CreatedBy:     userPgID,
				}); err != nil {
					return fmt.Errorf("create unreserve movement for sku %s: %w", item.SkuCode, err)
				}
			}
		}

		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:           pgID,
			Status:       StatusCancelled,
			CancelReason: toText(reason),
		}); err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    pgID,
			ToStatus:   StatusCancelled,
			FromStatus: pgtype.Text{String: fromStatus, Valid: true},
			Note:       toText(reason),
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	return s.GetOrder(ctx, pgutil.UUIDString(pgID))
}

// applyStatusUpdate handles simple transitions with no stock side effects.
// (picking, packing, ready_to_ship, delivered, completed)
func (s *Service) applyStatusUpdate(ctx context.Context, pgID pgtype.UUID, fromStatus, toStatus string, note *string, cancelReason pgtype.Text, userPgID pgtype.UUID) (*OrderDetailResponse, error) {
	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:           pgID,
			Status:       toStatus,
			CancelReason: cancelReason,
		}); err != nil {
			return err
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    pgID,
			ToStatus:   toStatus,
			FromStatus: pgtype.Text{String: fromStatus, Valid: true},
			Note:       toText(note),
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	return s.GetOrder(ctx, pgutil.UUIDString(pgID))
}

// ── Order Number ──────────────────────────────────────────────────

// generateOrderNumber queries PostgreSQL sequence inside the transaction.
// Format: ORD-YYYYMMDD-NNNNNN  e.g. ORD-20260531-000001
// Sequence is global (not per-day reset) — keeps it simple and atomic.
// วันที่มาจาก NOW() ใน DB เพื่อหลีกเลี่ยง timezone mismatch กับ app server.
func generateOrderNumber(ctx context.Context, tx pgx.Tx) (string, error) {
	var seq int64
	var dateStr string
	err := tx.QueryRow(ctx,
		`SELECT nextval('"order".order_number_seq'), TO_CHAR(NOW(), 'YYYYMMDD')`,
	).Scan(&seq, &dateStr)
	if err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}
	return fmt.Sprintf("ORD-%s-%06d", dateStr, seq), nil
}

// ── Converters ────────────────────────────────────────────────────

func createOrderToDetail(o db.CreateOrderRow, items []db.CreateOrderItemRow) *OrderDetailResponse {
	base := orderBaseFromCreate(o)
	respItems := make([]OrderItemResponse, len(items))
	for i, it := range items {
		respItems[i] = createItemToResponse(it)
	}
	return &OrderDetailResponse{
		OrderResponse: base,
		Items:         respItems,
		History:       []StatusHistoryResponse{},
	}
}

func orderBaseFromCreate(o db.CreateOrderRow) OrderResponse {
	resp := OrderResponse{
		ID:            pgutil.UUIDString(o.ID),
		OrderNumber:   o.OrderNumber,
		Channel:       o.Channel,
		Status:        o.Status,
		ShippingCost:  numericToFloat(o.ShippingCost),
		IsCOD:         o.IsCod,
		CODAmount:     numericToFloat(o.CodAmount),
		Subtotal:      numericToFloat(o.Subtotal),
		DiscountTotal: numericToFloat(o.DiscountTotal),
		Total:         numericToFloat(o.Total),
		CreatedAt:     o.CreatedAt.Time,
		UpdatedAt:     o.UpdatedAt.Time,
	}
	if o.ChannelOrderID.Valid {
		resp.ChannelOrderID = &o.ChannelOrderID.String
	}
	if o.CustomerName.Valid {
		resp.CustomerName = &o.CustomerName.String
	}
	if o.CustomerPhone.Valid {
		resp.CustomerPhone = &o.CustomerPhone.String
	}
	if len(o.ShippingAddress) > 0 {
		resp.ShippingAddress = o.ShippingAddress
	}
	if o.ShippingMethod.Valid {
		resp.ShippingMethod = &o.ShippingMethod.String
	}
	if o.Note.Valid {
		resp.Note = &o.Note.String
	}
	if o.CreatedBy.Valid {
		s := pgutil.UUIDString(o.CreatedBy)
		resp.CreatedBy = &s
	}
	return resp
}

func orderDetailToResponse(o db.GetOrderByIDRow, items []db.ListOrderItemsRow, history []db.OrderStatusHistory) *OrderDetailResponse {
	resp := OrderResponse{
		ID:            pgutil.UUIDString(o.ID),
		OrderNumber:   o.OrderNumber,
		Channel:       o.Channel,
		Status:        o.Status,
		ShippingCost:  numericToFloat(o.ShippingCost),
		IsCOD:         o.IsCod,
		CODAmount:     numericToFloat(o.CodAmount),
		Subtotal:      numericToFloat(o.Subtotal),
		DiscountTotal: numericToFloat(o.DiscountTotal),
		Total:         numericToFloat(o.Total),
		CreatedAt:     o.CreatedAt.Time,
		UpdatedAt:     o.UpdatedAt.Time,
	}
	if o.ChannelOrderID.Valid {
		resp.ChannelOrderID = &o.ChannelOrderID.String
	}
	if o.CustomerName.Valid {
		resp.CustomerName = &o.CustomerName.String
	}
	if o.CustomerPhone.Valid {
		resp.CustomerPhone = &o.CustomerPhone.String
	}
	if len(o.ShippingAddress) > 0 {
		resp.ShippingAddress = o.ShippingAddress
	}
	if o.ShippingMethod.Valid {
		resp.ShippingMethod = &o.ShippingMethod.String
	}
	if o.Note.Valid {
		resp.Note = &o.Note.String
	}
	if o.CreatedBy.Valid {
		s := pgutil.UUIDString(o.CreatedBy)
		resp.CreatedBy = &s
	}

	detail := &OrderDetailResponse{
		OrderResponse: resp,
		Items:         make([]OrderItemResponse, len(items)),
		History:       make([]StatusHistoryResponse, len(history)),
	}
	if o.ConfirmedAt.Valid {
		detail.ConfirmedAt = &o.ConfirmedAt.Time
	}
	if o.PackedAt.Valid {
		detail.PackedAt = &o.PackedAt.Time
	}
	if o.ShippedAt.Valid {
		detail.ShippedAt = &o.ShippedAt.Time
	}
	if o.DeliveredAt.Valid {
		detail.DeliveredAt = &o.DeliveredAt.Time
	}
	if o.CancelledAt.Valid {
		detail.CancelledAt = &o.CancelledAt.Time
	}
	if o.CancelReason.Valid {
		detail.CancelReason = &o.CancelReason.String
	}

	for i, it := range items {
		detail.Items[i] = listItemToResponse(it)
	}
	for i, h := range history {
		detail.History[i] = historyToResponse(h)
	}
	return detail
}

func listRowToResponse(r db.ListOrdersRow) OrderResponse {
	resp := OrderResponse{
		ID:            pgutil.UUIDString(r.ID),
		OrderNumber:   r.OrderNumber,
		Channel:       r.Channel,
		Status:        r.Status,
		ShippingCost:  numericToFloat(r.ShippingCost),
		IsCOD:         r.IsCod,
		CODAmount:     numericToFloat(r.CodAmount),
		Subtotal:      numericToFloat(r.Subtotal),
		DiscountTotal: numericToFloat(r.DiscountTotal),
		Total:         numericToFloat(r.Total),
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
	if r.ChannelOrderID.Valid {
		resp.ChannelOrderID = &r.ChannelOrderID.String
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
	if r.CreatedBy.Valid {
		s := pgutil.UUIDString(r.CreatedBy)
		resp.CreatedBy = &s
	}
	return resp
}

func updateRowToResponse(r db.UpdateOrderRow) *OrderResponse {
	resp := &OrderResponse{
		ID:            pgutil.UUIDString(r.ID),
		OrderNumber:   r.OrderNumber,
		Channel:       r.Channel,
		Status:        r.Status,
		ShippingCost:  numericToFloat(r.ShippingCost),
		IsCOD:         r.IsCod,
		CODAmount:     numericToFloat(r.CodAmount),
		Subtotal:      numericToFloat(r.Subtotal),
		DiscountTotal: numericToFloat(r.DiscountTotal),
		Total:         numericToFloat(r.Total),
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
	if r.CustomerName.Valid {
		resp.CustomerName = &r.CustomerName.String
	}
	if r.CustomerPhone.Valid {
		resp.CustomerPhone = &r.CustomerPhone.String
	}
	if len(r.ShippingAddress) > 0 {
		resp.ShippingAddress = r.ShippingAddress
	}
	if r.ShippingMethod.Valid {
		resp.ShippingMethod = &r.ShippingMethod.String
	}
	if r.Note.Valid {
		resp.Note = &r.Note.String
	}
	return resp
}

func createItemToResponse(r db.CreateOrderItemRow) OrderItemResponse {
	return OrderItemResponse{
		ID:             r.ID,
		SKUID:          pgutil.UUIDString(r.SkuID),
		SKUCode:        r.SkuCode,
		Name:           r.Name,
		Quantity:       r.Quantity,
		UnitPrice:      numericToFloat(r.UnitPrice),
		DiscountAmount: numericToFloat(r.DiscountAmount),
		TotalPrice:     numericToFloat(r.TotalPrice),
	}
}

func listItemToResponse(r db.ListOrderItemsRow) OrderItemResponse {
	return OrderItemResponse{
		ID:             r.ID,
		SKUID:          pgutil.UUIDString(r.SkuID),
		SKUCode:        r.SkuCode,
		Name:           r.Name,
		Quantity:       r.Quantity,
		UnitPrice:      numericToFloat(r.UnitPrice),
		DiscountAmount: numericToFloat(r.DiscountAmount),
		TotalPrice:     numericToFloat(r.TotalPrice),
	}
}

func historyToResponse(h db.OrderStatusHistory) StatusHistoryResponse {
	resp := StatusHistoryResponse{
		ID:        h.ID,
		ToStatus:  h.ToStatus,
		ChangedAt: h.ChangedAt.Time,
	}
	if h.FromStatus.Valid {
		resp.FromStatus = &h.FromStatus.String
	}
	if h.Note.Valid {
		resp.Note = &h.Note.String
	}
	if h.ChangedBy.Valid {
		s := pgutil.UUIDString(h.ChangedBy)
		resp.ChangedBy = &s
	}
	return resp
}

// ── Helpers ───────────────────────────────────────────────────────

func toText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func optText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// toNumeric converts float64 to pgtype.Numeric via string representation.
// pgtype.Numeric.Scan accepts string input (pgx v5 pgtype/numeric.go).
func toNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return n
}

// numericToFloat converts pgtype.Numeric → float64.
// Formula: value = Int * 10^Exp
// Uses big.Rat for negative exponents (typical for prices: NUMERIC(15,2) → Exp=-2).
func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid || n.NaN || n.Int == nil {
		return 0
	}
	if n.Exp >= 0 {
		exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Exp)), nil)
		val := new(big.Int).Mul(n.Int, exp)
		f, _ := new(big.Float).SetInt(val).Float64()
		return f
	}
	// negative exponent (ราคา: 10050 * 10^-2 = 100.50)
	div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil)
	rat := new(big.Rat).SetFrac(n.Int, div)
	f, _ := rat.Float64()
	return f
}

package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
	"github.com/KistametL/WMS/backend/internal/pgutil"
)

// ── Sentinel Errors ───────────────────────────────────────────────

var (
	ErrNotFound              = errors.New("not found")
	ErrInvalidTransition     = errors.New("invalid status transition")
	ErrOrderNotEditable      = errors.New("order can only be edited in pending or confirmed status")
	ErrInsufficientStock     = errors.New("insufficient stock")
	ErrSKUNotFound           = errors.New("sku not found")
	ErrSKUInactive           = errors.New("sku is inactive")
	ErrDuplicateChannelOrder = errors.New("order with this channel_order_id already exists")
	ErrInvalidInput          = errors.New("invalid input")
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
				return nil, fmt.Errorf("%w: %s", ErrSKUNotFound, item.SKUID)
			}
			return nil, err
		}
		if !sku.IsActive {
			return nil, fmt.Errorf("%w: %s", ErrSKUInactive, sku.SkuCode)
		}
		skuMap[item.SKUID] = skuInfo{Code: sku.SkuCode, Name: sku.Name}

		// N1: ตรวจว่า discount ไม่เกิน line total ป้องกัน negative subtotal ใน DB
		maxDiscount := item.UnitPrice * float64(item.Quantity)
		if item.DiscountAmount > maxDiscount {
			return nil, fmt.Errorf("%w: discount %.2f exceeds line total %.2f for %s",
				ErrInvalidInput, item.DiscountAmount, maxDiscount, sku.SkuCode)
		}
	}

	// ── 1b. Validate shipping_address format ─────────────────────────
	// json.RawMessage รับทุก value ที่เป็น valid JSON แต่ shipping_address
	// ต้องเป็น JSON object เท่านั้น (ไม่ใช่ string, number, array)
	if err := validateShippingAddress(req.ShippingAddress); err != nil {
		return nil, err
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
	// N4 fix: capture order ID เท่านั้น แล้วเรียก GetOrder หลัง TX
	// เพื่อให้ response รวม history ครั้งแรก (pending) ด้วย
	var orderID pgtype.UUID

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		orderNum, err := generateOrderNumber(ctx, tx)
		if err != nil {
			return err
		}

		// ── Re-validate SKUs inside TX (catch deactivation race) ─────────
		// SKU อาจถูก deactivate ระหว่าง pre-check (step 1) กับ INSERT จริง
		for _, item := range req.Items {
			pgSkuID, _ := pgutil.ParseUUID(item.SKUID)
			sku, err := q.GetSKUByID(ctx, pgSkuID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: %s (deleted after validation)", ErrSKUNotFound, item.SKUID)
				}
				return err
			}
			if !sku.IsActive {
				return fmt.Errorf("%w: %s (deactivated after validation)", ErrSKUInactive, sku.SkuCode)
			}
		}

		created, err := q.CreateOrder(ctx, db.CreateOrderParams{
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
			// idx_orders_channel_order_unique: (channel, channel_order_id) ซ้ำ
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrDuplicateChannelOrder
			}
			return fmt.Errorf("create order: %w", err)
		}
		orderID = created.ID

		for _, item := range req.Items {
			info := skuMap[item.SKUID]
			pgSkuID, _ := pgutil.ParseUUID(item.SKUID)
			lineTotal := (item.UnitPrice * float64(item.Quantity)) - item.DiscountAmount
			if _, err := q.CreateOrderItem(ctx, db.CreateOrderItemParams{
				OrderID:        created.ID,
				SkuID:          pgSkuID,
				SkuCode:        info.Code,
				Name:           info.Name,
				Quantity:       item.Quantity,
				UnitPrice:      toNumeric(item.UnitPrice),
				TotalPrice:     toNumeric(lineTotal),
				DiscountAmount: toNumeric(item.DiscountAmount),
			}); err != nil {
				return fmt.Errorf("create order item: %w", err)
			}
		}

		return q.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{
			OrderID:    created.ID,
			ToStatus:   StatusPending,
			FromStatus: pgtype.Text{},
			Note:       pgtype.Text{},
			ChangedBy:  userPgID,
		})
	})
	if txErr != nil {
		return nil, txErr
	}

	// N4 fix: GetOrder รวม items + history (pending) ครบ — consistent กับ GET /orders/:id
	return s.GetOrder(ctx, pgutil.UUIDString(orderID))
}

// ── Get ───────────────────────────────────────────────────────────

func (s *Service) GetOrder(ctx context.Context, id string) (*OrderDetailResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	// ── RepeatableRead + ReadOnly TX (N2 fix) ────────────────────────
	// อ่าน 3 queries ใน snapshot เดียวกัน ป้องกัน inconsistency
	// เช่น status history ที่เพิ่งถูก insert ระหว่าง query 1 กับ query 3
	var result *OrderDetailResponse
	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		order, err := q.GetOrderByID(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		items, err := q.ListOrderItems(ctx, pgID)
		if err != nil {
			return err
		}

		history, err := q.ListStatusHistory(ctx, pgID)
		if err != nil {
			return err
		}

		result = orderDetailToResponse(order, items, history)
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, txErr
	}
	return result, nil
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

	totalPages := 0
	if limit > 0 {
		totalPages = (int(total) + limit - 1) / limit // ceiling division
	}

	return &ListResponse[OrderResponse]{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// ── Update Info ───────────────────────────────────────────────────

// UpdateOrder แก้ไข customer/shipping/note ได้เฉพาะ pending หรือ confirmed
// total ถูก recalculate อัตโนมัติใน SQL เมื่อ shipping_cost หรือ discount_total เปลี่ยน
//
// M1 fix: ใช้ TX + GetOrderByIDForUpdate ป้องกัน race condition ระหว่าง
// status check กับ UPDATE (concurrent status change จะถูก detect ก่อน update)
func (s *Service) UpdateOrder(ctx context.Context, id string, req UpdateOrderRequest) (*OrderResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	// N3: validate shipping_address ก่อน TX เพื่อ fast-fail
	if err := validateShippingAddress(req.ShippingAddress); err != nil {
		return nil, err
	}

	var result *OrderResponse
	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// Lock orders row ก่อน read status (ป้องกัน TOCTOU)
		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if locked.Status != StatusPending && locked.Status != StatusConfirmed {
			return ErrOrderNotEditable
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

		updated, err := q.UpdateOrder(ctx, params)
		if err != nil {
			return err
		}
		result = updateRowToResponse(updated)
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
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
		return s.applyStatusUpdate(ctx, pgID, existing.Status, req.Status, req.Note, userPgID)
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
//
// C1 fix: เพิ่ม GetOrderByIDForUpdate เพื่อ lock orders row ก่อน UPDATE
// ป้องกัน race condition ที่ concurrent cancel อาจ cancel order ไปแล้ว
// แต่ TX นี้ยัง set status = picking บน cancelled order
//
// N2 fix: ตัด cancelReason parameter ออก — ไม่เคยใช้ใน path นี้เลย
func (s *Service) applyStatusUpdate(ctx context.Context, pgID pgtype.UUID, fromStatus, toStatus string, note *string, userPgID pgtype.UUID) (*OrderDetailResponse, error) {
	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		// ── Lock + re-validate (C1 fix) ───────────────────────────────
		locked, err := q.GetOrderByIDForUpdate(ctx, pgID)
		if err != nil {
			return fmt.Errorf("lock order row: %w", err)
		}
		if locked.Status != fromStatus {
			return fmt.Errorf("%w: order status changed to %s (concurrent request?)",
				ErrInvalidTransition, locked.Status)
		}

		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     pgID,
			Status: toStatus,
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
	// M4 fix: include fields now present in RETURNING clause
	if r.ChannelOrderID.Valid {
		resp.ChannelOrderID = &r.ChannelOrderID.String
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
	if r.CreatedBy.Valid {
		s := pgutil.UUIDString(r.CreatedBy)
		resp.CreatedBy = &s
	}
	return resp
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

// validateShippingAddress ตรวจว่า shipping_address เป็น JSON object ถ้า provided
// json.RawMessage รับทุก valid JSON value (string, number, array ฯลฯ)
// แต่ JSONB address ต้องเป็น object เท่านั้น
//
// N5 fix: ไม่ expose underlying json error (มี Go type name ใน message)
// M2 fix: wrap ErrInvalidInput ให้ handler map → 400 ได้ถูกต้อง
func validateShippingAddress(addr json.RawMessage) error {
	if len(addr) == 0 || string(addr) == "null" {
		return nil // optional field — ไม่ต้องส่งก็ได้
	}
	var obj map[string]any
	if err := json.Unmarshal(addr, &obj); err != nil {
		return fmt.Errorf("%w: shipping_address must be a JSON object, e.g. {\"street\":\"...\"}",
			ErrInvalidInput)
	}
	return nil
}

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
//
// N5 fix: ตรวจ InfinityModifier ก่อน — ค่า Infinity ไม่ควรเกิดกับ price column
// แต่ถ้าเกิดขึ้น (data corruption / migration) ให้ return 0 แทน panic/wrong result
func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid || n.NaN || n.InfinityModifier != pgtype.Finite || n.Int == nil {
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

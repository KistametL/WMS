package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
	"github.com/KistametL/WMS/backend/internal/pgutil"
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

// ── List / Get ────────────────────────────────────────────────────

func (s *Service) ListUsers(ctx context.Context, q ListUsersQuery) (*ListResponse[UserResponse], error) {
	page, limit, offset := pgutil.NormalizePagination(q.Page, q.Limit)

	countParams := db.CountUsersParams{Search: pgutil.OptText(q.Search)}
	if q.IsActive != nil {
		countParams.IsActive = pgtype.Bool{Bool: *q.IsActive, Valid: true}
	}
	total, err := s.queries.CountUsers(ctx, countParams)
	if err != nil {
		return nil, err
	}

	offset32 := int32(offset) //nolint:gosec // G115: bounded by NormalizePagination
	limit32 := int32(limit)   //nolint:gosec // G115: capped at 100

	rows, err := s.queries.ListUsers(ctx, db.ListUsersParams{
		Search:   countParams.Search,
		IsActive: countParams.IsActive,
		Offset:   pgtype.Int4{Int32: offset32, Valid: true},
		Limit:    pgtype.Int4{Int32: limit32, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	items := make([]UserResponse, len(rows))
	for i, r := range rows {
		items[i] = UserResponse{
			ID:        pgutil.UUIDString(r.ID),
			Email:     r.Email,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			IsActive:  r.IsActive,
			CreatedAt: r.CreatedAt.Time,
			UpdatedAt: r.UpdatedAt.Time,
		}
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (int(total) + limit - 1) / limit
	}

	return &ListResponse[UserResponse]{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) GetUser(ctx context.Context, id string) (*UserDetailResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	profile, err := s.queries.GetUserProfile(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	roles, err := s.queries.GetUserRoles(ctx, pgID)
	if err != nil {
		return nil, err
	}

	permissions, err := s.queries.GetUserPermissions(ctx, pgID)
	if err != nil {
		return nil, err
	}

	roleResponses := make([]RoleResponse, len(roles))
	for i, r := range roles {
		rr := RoleResponse{ID: int32(r.ID), Name: r.Name} //nolint:gosec // role IDs are small ints
		if r.Description.Valid {
			rr.Description = &r.Description.String
		}
		roleResponses[i] = rr
	}

	return &UserDetailResponse{
		UserResponse: UserResponse{
			ID:        pgutil.UUIDString(profile.ID),
			Email:     profile.Email,
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
			IsActive:  profile.IsActive,
			CreatedAt: profile.CreatedAt.Time,
			UpdatedAt: profile.UpdatedAt.Time,
		},
		Roles:       roleResponses,
		Permissions: permissions,
	}, nil
}

// ── Create ────────────────────────────────────────────────────────

// CreateUser สร้าง user ใหม่ + assign roles ใน TX เดียว
// ถ้า role assignment ล้มเหลว → rollback ทั้งหมด
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest, createdByID string) (*UserDetailResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	createdByPgID := parseUserID(createdByID)

	var newUserID pgtype.UUID

	txErr := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		created, err := q.CreateUser(ctx, db.CreateUserParams{
			Email:        req.Email,
			PasswordHash: string(hash),
			FirstName:    req.FirstName,
			LastName:     req.LastName,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrDuplicateEmail
			}
			return fmt.Errorf("create user: %w", err)
		}
		newUserID = created.ID

		for _, roleID := range req.RoleIDs {
			if err := q.AssignRole(ctx, db.AssignRoleParams{
				UserID:     created.ID,
				RoleID:     roleID,
				AssignedBy: createdByPgID,
			}); err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23503" {
					return fmt.Errorf("%w: id %d", ErrRoleNotFound, roleID)
				}
				return fmt.Errorf("assign role %d: %w", roleID, err)
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	return s.GetUser(ctx, pgutil.UUIDString(newUserID))
}

// ── Update ────────────────────────────────────────────────────────

// UpdateUser แก้ไข user info
// ถ้า is_active เปลี่ยนเป็น false → revoke tokens ทั้งหมด
func (s *Service) UpdateUser(ctx context.Context, id, requesterID string, req UpdateUserRequest) (*UserDetailResponse, error) {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	// ป้องกัน admin deactivate ตัวเอง
	if req.IsActive != nil && !*req.IsActive && id == requesterID {
		return nil, ErrCannotDeactivateSelf
	}

	params := db.UpdateUserParams{ID: pgID}
	if req.Email != nil {
		params.Email = pgutil.ToText(req.Email)
	}
	if req.FirstName != nil {
		params.FirstName = pgutil.ToText(req.FirstName)
	}
	if req.LastName != nil {
		params.LastName = pgutil.ToText(req.LastName)
	}
	if req.IsActive != nil {
		params.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}

	updated, err := s.queries.UpdateUser(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}

	// deactivate → revoke tokens ทันที (force re-login)
	if req.IsActive != nil && !*req.IsActive {
		_ = s.queries.RevokeAllUserTokens(ctx, updated.ID) // best-effort
	}

	return s.GetUser(ctx, pgutil.UUIDString(updated.ID))
}

// ── Delete ────────────────────────────────────────────────────────

// DeleteUser soft-deletes user + revokes tokens ใน TX เดียว
func (s *Service) DeleteUser(ctx context.Context, id, requesterID string) error {
	if id == requesterID {
		return ErrCannotDeleteSelf
	}

	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return ErrNotFound
	}

	// ตรวจว่า user มีอยู่จริงก่อน
	if _, err := s.queries.GetUserProfile(ctx, pgID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if err := q.SoftDeleteUser(ctx, pgID); err != nil {
			return err
		}
		return q.RevokeAllUserTokens(ctx, pgID)
	})
}

// ── Roles ─────────────────────────────────────────────────────────

func (s *Service) ListRoles(ctx context.Context) ([]RoleResponse, error) {
	rows, err := s.queries.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]RoleResponse, len(rows))
	for i, r := range rows {
		rr := RoleResponse{ID: int32(r.ID), Name: r.Name} //nolint:gosec
		if r.Description.Valid {
			rr.Description = &r.Description.String
		}
		result[i] = rr
	}
	return result, nil
}

func (s *Service) AssignRole(ctx context.Context, userID string, roleID int32, assignedByID string) error {
	pgUserID, err := pgutil.ParseUUID(userID)
	if err != nil {
		return ErrNotFound
	}

	// ตรวจ user มีอยู่
	if _, err := s.queries.GetUserProfile(ctx, pgUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	err = s.queries.AssignRole(ctx, db.AssignRoleParams{
		UserID:     pgUserID,
		RoleID:     roleID,
		AssignedBy: parseUserID(assignedByID),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrRoleNotFound
		}
		return err
	}
	return nil
}

func (s *Service) RemoveRole(ctx context.Context, userID string, roleID int32) error {
	pgUserID, err := pgutil.ParseUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	return s.queries.RemoveRole(ctx, db.RemoveRoleParams{
		UserID: pgUserID,
		RoleID: roleID,
	})
}

// ── Password ──────────────────────────────────────────────────────

// ResetPassword admin รีเซ็ต password ให้ user + revoke tokens
func (s *Service) ResetPassword(ctx context.Context, id string, req ResetPasswordRequest) error {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return ErrNotFound
	}

	if _, err := s.queries.GetUserProfile(ctx, pgID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if err := q.UpdatePassword(ctx, db.UpdatePasswordParams{
			ID:           pgID,
			PasswordHash: string(hash),
		}); err != nil {
			return err
		}
		return q.RevokeAllUserTokens(ctx, pgID)
	})
}

// ChangePassword user เปลี่ยน password ตัวเอง (ต้องใส่ current password)
func (s *Service) ChangePassword(ctx context.Context, id string, req ChangePasswordRequest) error {
	pgID, err := pgutil.ParseUUID(id)
	if err != nil {
		return ErrNotFound
	}

	// ต้องการ password_hash เพื่อ verify — ใช้ GetUserByID (รวม hash)
	user, err := s.queries.GetUserByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return ErrInvalidPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return s.queries.UpdatePassword(ctx, db.UpdatePasswordParams{
		ID:           pgID,
		PasswordHash: string(hash),
	})
}

// ── Helpers ───────────────────────────────────────────────────────

func parseUserID(id string) pgtype.UUID {
	if id == "" {
		return pgtype.UUID{}
	}
	parsed, err := pgutil.ParseUUID(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return parsed
}

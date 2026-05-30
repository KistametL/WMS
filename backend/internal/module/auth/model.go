package auth

// Request/Response structs สำหรับ HTTP layer

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	TokenType    string   `json:"token_type"`
	AccessToken  string   `json:"access_token"`
	ExpiresIn    int      `json:"expires_in"` // วินาที เช่น 86400 = 24 ชั่วโมง
	RefreshToken string   `json:"refresh_token"`
	UserID       string   `json:"user_id"`
	Email        string   `json:"email"`
	FirstName    string   `json:"first_name"`
	LastName     string   `json:"last_name"`
	Roles        []string `json:"roles"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

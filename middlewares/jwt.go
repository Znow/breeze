package middleware

import (
	"fmt"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nelthaarion/breeze"
)

// JWTOptions defines configurable JWT authentication behavior.
type JWTOptions struct {
	AccessSecret       string                                            // Secret key for access tokens
	RefreshSecret      string                                            // Secret key for refresh tokens
	SigningMethod       jwt.SigningMethod                                 // e.g., jwt.SigningMethodHS256
	TokenLookup        func(ctx *breeze.Context) (string, string, error) // returns (accessToken, refreshToken, error)
	OnUnauthorized     func(ctx *breeze.Context, err error)              // Optional: custom 401 handler
	UserContextKey     string                                            // Key to store claims JSON in ctx.Params
	RequiredRoles      []string                                          // Optional: roles required to access the route
	ClaimsValidator    func(claims jwt.MapClaims) bool                   // Optional: extra claim validation
	EnableRefreshToken bool                                              // Enable refresh token support
}

// DefaultTokenLookup extracts the access token from the Authorization header.
func DefaultTokenLookup(ctx *breeze.Context) (string, string, error) {
	authHeader := ctx.Req.Header["authorization"]
	if authHeader == "" {
		return "", "", fmt.Errorf("authorization header missing")
	}
	// Fast path: avoid strings.Split allocation for the common "Bearer <tok>" case.
	const prefix = "bearer "
	lower := strings.ToLower(authHeader)
	if !strings.HasPrefix(lower, prefix) {
		return "", "", fmt.Errorf("invalid authorization header format")
	}
	return authHeader[len(prefix):], "", nil
}

// DefaultUnauthorizedHandler returns 401 Unauthorized.
func DefaultUnauthorizedHandler(ctx *breeze.Context, err error) {
	ctx.Status(401)
	ctx.WriteString("Unauthorized: " + err.Error())
}

// JWTAuthMiddleware returns a JWT authentication middleware.
func JWTAuthMiddleware(opts JWTOptions) breeze.HandlerFunc {
	if opts.SigningMethod == nil {
		opts.SigningMethod = jwt.SigningMethodHS256
	}
	if opts.UserContextKey == "" {
		opts.UserContextKey = "user"
	}
	if opts.TokenLookup == nil {
		opts.TokenLookup = func(ctx *breeze.Context) (string, string, error) {
			tk, _, err := DefaultTokenLookup(ctx)
			return tk, "", err
		}
	}
	if opts.OnUnauthorized == nil {
		opts.OnUnauthorized = DefaultUnauthorizedHandler
	}

	// Pre-convert secrets to []byte once at middleware creation time,
	// not on every request, to avoid a heap allocation per call.
	accessKey := []byte(opts.AccessSecret)
	refreshKey := []byte(opts.RefreshSecret)

	return func(ctx *breeze.Context) {
		accessToken, refreshToken, err := opts.TokenLookup(ctx)
		if err != nil {
			opts.OnUnauthorized(ctx, err)
			return
		}

		claims, valid := validateJWT(accessToken, accessKey, opts.SigningMethod)
		if !valid && opts.EnableRefreshToken && refreshToken != "" {
			refreshClaims, ok := validateJWT(refreshToken, refreshKey, opts.SigningMethod)
			if ok {
				newAccessToken, err := generateJWTBytes(accessKey, jwt.MapClaims{
					"user_id": refreshClaims["user_id"],
					"role":    refreshClaims["role"],
				}, 15*time.Minute, opts.SigningMethod)
				if err == nil {
					ctx.SetHeader("X-New-Access-Token", newAccessToken)
					claims = refreshClaims
					valid = true
				}
			}
		}

		if !valid {
			opts.OnUnauthorized(ctx, fmt.Errorf("invalid token"))
			return
		}

		// Role check
		if len(opts.RequiredRoles) > 0 {
			role, _ := claims["role"].(string)
			found := false
			for _, r := range opts.RequiredRoles {
				if r == role {
					found = true
					break
				}
			}
			if !found {
				opts.OnUnauthorized(ctx, fmt.Errorf("insufficient role"))
				return
			}
		}

		// Extra claims validation
		if opts.ClaimsValidator != nil && !opts.ClaimsValidator(claims) {
			opts.OnUnauthorized(ctx, fmt.Errorf("claims validation failed"))
			return
		}

		// Store claims as a proper JSON string so handlers can unmarshal them.
		// Using json.Marshal instead of fmt.Sprintf("%v") makes the value
		// actually parseable by downstream code.
		claimsJSON, err := gojson.Marshal(claims)
		if err != nil {
			opts.OnUnauthorized(ctx, fmt.Errorf("failed to encode claims"))
			return
		}
		ctx.SetParam(opts.UserContextKey, string(claimsJSON))
		ctx.Next()
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

// validateJWT parses and validates a token string.
// Accepts the secret as []byte to avoid repeated string→[]byte conversion.
func validateJWT(tokenString string, secret []byte, method jwt.SigningMethod) (jwt.MapClaims, bool) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != method.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

// generateJWTBytes is like GenerateJWT but takes the secret as []byte
// (internal use only, avoids the extra allocation on the refresh path).
func generateJWTBytes(secret []byte, claims jwt.MapClaims, duration time.Duration, method jwt.SigningMethod) (string, error) {
	if claims == nil {
		claims = jwt.MapClaims{}
	}
	claims["exp"] = time.Now().Add(duration).Unix()
	token := jwt.NewWithClaims(method, claims)
	return token.SignedString(secret)
}

// GenerateJWT generates a new signed JWT token.
func GenerateJWT(secret string, claims jwt.MapClaims, duration time.Duration, method jwt.SigningMethod) (string, error) {
	if method == nil {
		method = jwt.SigningMethodHS256
	}
	return generateJWTBytes([]byte(secret), claims, duration, method)
}

// GenerateRefreshToken generates a refresh token.
func GenerateRefreshToken(secret string, claims jwt.MapClaims, duration time.Duration, method jwt.SigningMethod) (string, error) {
	if method == nil {
		method = jwt.SigningMethodHS256
	}
	if claims == nil {
		claims = jwt.MapClaims{}
	}
	claims["exp"] = time.Now().Add(duration).Unix()
	claims["type"] = "refresh"
	token := jwt.NewWithClaims(method, claims)
	return token.SignedString([]byte(secret))
}

package controller

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/golang-jwt/jwt/v5"
)

func TestCanvasGroupType(t *testing.T) {
	tests := map[string]string{
		"生图分组-flux": "image",
		"视频生成":      "video",
		"grok":      "text",
		"default":   "text",
	}
	for group, want := range tests {
		if got := canvasGroupType(group); got != want {
			t.Fatalf("canvasGroupType(%q) = %q, want %q", group, got, want)
		}
	}
}

func TestCreateCanvasSSOTicket(t *testing.T) {
	t.Setenv("CANVAS_SSO_ISSUER", "new-api-test")
	t.Setenv("CANVAS_SSO_AUDIENCE", "infinite-canvas-test")
	secret := "0123456789abcdef0123456789abcdef"
	now := time.Unix(1_800_000_000, 0).UTC()
	user := &model.User{Id: 42, Username: "canvas-user", DisplayName: "Canvas User", Role: common.RoleRootUser}

	ticket, expiresAt, err := createCanvasSSOTicket(user, now, secret)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := expiresAt, now.Add(canvasSSOTicketTTL); !got.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", got, want)
	}

	claims := &canvasSSOClaims{}
	parsed, err := jwt.ParseWithClaims(ticket, claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithAudience("infinite-canvas-test"), jwt.WithIssuer("new-api-test"), jwt.WithTimeFunc(func() time.Time { return now.Add(time.Second) }))
	if err != nil || !parsed.Valid {
		t.Fatalf("ticket parse failed: %v", err)
	}
	if claims.Subject != strconv.Itoa(user.Id) || claims.Username != user.Username || claims.DisplayName != user.DisplayName || !claims.IsRoot || claims.ID == "" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil || claims.NotBefore == nil {
		t.Fatalf("registered times missing: %+v", claims.RegisteredClaims)
	}
}

func TestCreateCanvasSSOTicketRejectsShortSecret(t *testing.T) {
	_, _, err := createCanvasSSOTicket(&model.User{Id: 1}, time.Now(), "short")
	if err == nil {
		t.Fatal("expected short secret error")
	}
}

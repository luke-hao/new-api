package controller

import (
	"context"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"os"
	"testing"
	"time"
)

func TestKeyConsolePostgres(t *testing.T) {
	dsn := os.Getenv("KEY_CONSOLE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("isolated PostgreSQL fixture not configured")
	}
	db, managed := openTokenControllerExternalDB(t, "postgres", dsn)
	migrateTokenControllerTestDB(t, db)
	*managed = true
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Migrator().DropTable(&model.Log{}) })
	group := "Quoted group"
	token := model.Token{Id: 9101, UserId: 901, Name: "Case_%", Key: "fixture-postgres", Status: 1, ExpiredTime: -1, RemainQuota: 100, Group: group}
	if err := db.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	rows, total, err := model.SearchUserTokens(901, "Case_%", "", 0, 20, model.TokenListOptions{ContainsName: true, Group: &group, Sort: "group", Status: []int{1}})
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("%+v %d %v", rows, total, err)
	}
	start, end := service.TokenDayBounds(time.Now())
	logs := []model.Log{{UserId: 901, TokenId: 9101, CreatedAt: start, Type: 2, Quota: 50, PromptTokens: 2, CompletionTokens: 3}, {UserId: 901, TokenId: 9101, CreatedAt: start + 1, Type: 6, Quota: 5}}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}
	usage, err := model.GetTokenDailyUsage(context.Background(), 901, []int{9101}, start, end)
	if err != nil || len(usage) != 1 || usage[0].Quota != 45 || usage[0].Tokens != 5 {
		t.Fatalf("%+v %v", usage, err)
	}
	if _, err := model.UpdateTokenStatusOnly(9101, 901, 2); err != nil {
		t.Fatal(err)
	}
}

package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"testing"
)

func TestModelRoutingPostgresMigrationAndSelection(t *testing.T) {
	dsn := os.Getenv("MODEL_ROUTING_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("isolated PostgreSQL fixture not configured")
	}
	previousDB, previousLog := DB, LOG_DB
	previousPG, previousSQLite, previousMySQL, previousCache := common.UsingPostgreSQL, common.UsingSQLite, common.UsingMySQL, common.MemoryCacheEnabled
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLog
		common.UsingPostgreSQL, common.UsingSQLite, common.UsingMySQL, common.MemoryCacheEnabled = previousPG, previousSQLite, previousMySQL, previousCache
		initCol()
	})
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	DB, LOG_DB = db, db
	common.UsingPostgreSQL = true
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.MemoryCacheEnabled = false
	initCol()
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &ChannelGroupRouting{}, &ChannelGroupStabilityPolicy{}, &ChannelModelRouting{}, &ChannelModelStabilityPolicy{}))
	require.NoError(t, db.AutoMigrate(&ChannelModelRouting{}, &ChannelModelStabilityPolicy{}))
	a := insertRoutingTestChannel(t, 9811, 7, 0, "pg-fixture")
	b := insertRoutingTestChannel(t, 9812, 6, 0, "pg-fixture")
	for _, c := range []*Channel{a, b} {
		c.Models = "sol,astra"
		require.NoError(t, c.Update())
	}
	_, err = UpdateChannelModelRoutings("pg-fixture", "astra", []ChannelGroupRoutingPatch{{ChannelId: b.Id, Priority: modelPriority(20)}}, "")
	require.NoError(t, err)
	selected, err := GetRandomSatisfiedChannel("pg-fixture", "astra", 0)
	require.NoError(t, err)
	require.Equal(t, b.Id, selected.Id)
	selected, err = GetRandomSatisfiedChannel("pg-fixture", "sol", 0)
	require.NoError(t, err)
	require.Equal(t, a.Id, selected.Id)
	parent, err := GetOrCreateChannelGroupStabilityPolicy("pg-fixture")
	require.NoError(t, err)
	child, err := GetChannelModelPolicy("pg-fixture", "astra")
	require.NoError(t, err)
	p := ResolveChannelModelPolicy(*parent, *child)
	_, snapshot, err := GetChannelModelStabilityCandidates(p.Group, p.Model)
	require.NoError(t, err)
	_, err = CommitChannelModelStabilityRun(p, false, snapshot, []ChannelModelRouting{{ChannelId: b.Id, Result: "success", LatencyMs: 20}}, nil, ChannelGroupStabilityRunUpdate{LastResult: "healthy", LastPrimaryChannelId: b.Id}, true)
	require.NoError(t, err)
	require.NoError(t, RestoreChannelModelRoutingProjection())
}

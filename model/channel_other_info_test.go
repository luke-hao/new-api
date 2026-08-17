package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"

	"gorm.io/gorm"
)

func TestChannelOtherInfoWritersPreserveIndependentMetadata(t *testing.T) {
	originalDB := DB
	originalLogDB := LOG_DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	defer func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
	}()

	db, err := gorm.Open(sqlite.Open("file:channel-other-info?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Channel{}); err != nil {
		t.Fatal(err)
	}
	DB = db
	LOG_DB = db
	common.MemoryCacheEnabled = false
	common.UsingSQLite = true

	channel := Channel{
		Name:      "probe-test",
		Key:       "secret",
		OtherInfo: `{"existing":"value","last_test_error_code":"old"}`,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := MergeChannelOtherInfo(channel.Id, func(otherInfo map[string]interface{}) error {
		otherInfo["upstream_billing_probe"] = map[string]interface{}{"status": "ok"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	channel.UpdateResponseTime(125)
	var afterSuccess Channel
	if err := db.First(&afterSuccess, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	assertChannelOtherInfoValue(t, afterSuccess.OtherInfo, "existing", "value")
	assertChannelProbeStatus(t, afterSuccess.OtherInfo, "ok")
	if _, ok := afterSuccess.GetOtherInfo()[ChannelLastTestErrorCodeKey]; ok {
		t.Fatal("successful channel test did not clear its own previous error")
	}

	afterSuccess.UpdateTestFailure("probe_failure", "failed")
	var afterFailure Channel
	if err := db.First(&afterFailure, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	assertChannelOtherInfoValue(t, afterFailure.OtherInfo, "existing", "value")
	assertChannelProbeStatus(t, afterFailure.OtherInfo, "ok")
	assertChannelOtherInfoValue(t, afterFailure.OtherInfo, ChannelLastTestErrorCodeKey, "probe_failure")

	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-start
		afterFailure.UpdateResponseTime(250)
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		_, mergeErr := MergeChannelOtherInfo(channel.Id, func(otherInfo map[string]interface{}) error {
			otherInfo["upstream_billing_probe"] = map[string]interface{}{"status": "partial"}
			return nil
		})
		if mergeErr != nil {
			t.Errorf("concurrent probe merge failed: %v", mergeErr)
		}
	}()
	close(start)
	waitGroup.Wait()

	var afterConcurrentWrites Channel
	if err := db.First(&afterConcurrentWrites, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	assertChannelOtherInfoValue(t, afterConcurrentWrites.OtherInfo, "existing", "value")
	assertChannelProbeStatus(t, afterConcurrentWrites.OtherInfo, "partial")
	if _, ok := afterConcurrentWrites.GetOtherInfo()[ChannelLastTestErrorCodeKey]; ok {
		t.Fatal("successful concurrent channel test did not clear its own previous error")
	}
}

func assertChannelOtherInfoValue(t *testing.T, raw, key string, expected interface{}) {
	t.Helper()
	var otherInfo map[string]interface{}
	if err := common.Unmarshal([]byte(raw), &otherInfo); err != nil {
		t.Fatal(err)
	}
	if actual := otherInfo[key]; actual != expected {
		t.Fatalf("other_info[%q] = %#v, want %#v", key, actual, expected)
	}
}

func assertChannelProbeStatus(t *testing.T, raw, expected string) {
	t.Helper()
	var otherInfo map[string]interface{}
	if err := common.Unmarshal([]byte(raw), &otherInfo); err != nil {
		t.Fatal(err)
	}
	probe, ok := otherInfo["upstream_billing_probe"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing probe metadata: %#v", otherInfo)
	}
	if actual := probe["status"]; actual != expected {
		t.Fatalf("probe status = %#v, want %q", actual, expected)
	}
}

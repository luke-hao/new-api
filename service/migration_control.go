package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// StartMigrationControl is opt-in and local to a private Unix socket. The
// response deliberately distinguishes billing flush from whole-site readiness.
func StartMigrationControl() error {
	path := os.Getenv("MIGRATION_CONTROL_SOCKET")
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("migration control socket must be absolute")
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return err
	}
	if parent.Mode().Perm()&0007 != 0 {
		return fmt.Errorf("migration control directory must not allow other users")
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0600); err != nil {
		listener.Close()
		return err
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" && r.URL.Path != "/flush" {
			http.NotFound(w, r)
			return
		}
		if (r.URL.Path == "/status" && r.Method != http.MethodGet) || (r.URL.Path == "/flush" && r.Method != http.MethodPost) {
			http.Error(w, "method not allowed", 405)
			return
		}
		var flushErr error
		if r.URL.Path == "/flush" {
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			flushErr = DrainBillingForMigration(ctx)
		}
		output := map[string]interface{}{"batch": model.GetBatchQuotaState(), "refunds": GetRefundDrainState(), "cutover_ready": false}
		if flushErr != nil {
			output["error"] = flushErr.Error()
		}
		data, err := common.Marshal(output)
		if err != nil {
			http.Error(w, "serialization error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if flushErr != nil {
			w.WriteHeader(http.StatusConflict)
		}
		w.Write(data)
	})
	go func() {
		server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			common.SysLog("migration control stopped: " + err.Error())
		}
	}()
	return nil
}

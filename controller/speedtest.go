package controller

import (
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

const speedtestMaxUploadBytes int64 = 128 << 20

func SpeedtestPing(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Status(http.StatusNoContent)
}

func SpeedtestUpload(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")

	maxBytes := int64(constant.MaxRequestBodyMB) << 20
	if maxBytes <= 0 || maxBytes > speedtestMaxUploadBytes {
		maxBytes = speedtestMaxUploadBytes
	}
	defer c.Request.Body.Close()
	started := time.Now()
	readBytes, err := io.Copy(io.Discard, io.LimitReader(c.Request.Body, maxBytes+1))
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"message": "speedtest upload is too large"}})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "speedtest upload could not be read"}})
		return
	}
	if readBytes > maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"message": "speedtest upload is too large"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"received_bytes":     readBytes,
		"server_read_ms":     time.Since(started).Seconds() * 1000,
		"server_received_at": time.Now().UTC().Format(time.RFC3339Nano),
		"request_id":         c.GetString(common.RequestIdKey),
	})
}

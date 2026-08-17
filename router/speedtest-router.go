package router

import (
	"embed"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetSpeedtestRouter(router *gin.Engine, assets ThemeAssets) {
	speedtest := router.Group("/speedtest")
	speedtest.Use(middleware.RouteTag("speedtest"))
	{
		speedtest.GET("/", func(c *gin.Context) {
			serveSpeedtestAsset(c, assets.SpeedtestBuildFS, "speedtest/index.html", "text/html; charset=utf-8")
		})
		speedtest.GET("/assets/:name", func(c *gin.Context) {
			name := c.Param("name")
			contentType := map[string]string{
				"app.js":     "text/javascript; charset=utf-8",
				"styles.css": "text/css; charset=utf-8",
			}[name]
			if contentType == "" {
				c.Status(http.StatusNotFound)
				return
			}
			serveSpeedtestAsset(c, assets.SpeedtestBuildFS, "speedtest/"+name, contentType)
		})
		speedtest.GET("/ping", controller.SpeedtestPing)
		speedtest.POST("/upload", middleware.TokenAuth(), middleware.UploadRateLimit(), controller.SpeedtestUpload)
	}
}

func serveSpeedtestAsset(c *gin.Context, buildFS embed.FS, name, contentType string) {
	data, err := buildFS.ReadFile(name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	if strings.HasSuffix(name, "index.html") {
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; connect-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	}
	c.Data(http.StatusOK, contentType, data)
}

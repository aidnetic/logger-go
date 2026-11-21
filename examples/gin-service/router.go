package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/aidnetic/logger-go"
)

// newRouter wires the middleware, the API routes and, if adminToken is set,
// the admin endpoints. It is separate from main so tests can build the same
// router with a recording logger.
func newRouter(log *logger.Logger, levels *RouteLevels, adminToken string) *gin.Engine {
	r := gin.New()

	// Don't take client IPs from X-Forwarded-For sent by arbitrary clients.
	// Behind a load balancer, list its addresses here instead.
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatal("trusted proxies:", err)
	}

	// RequestLogger must come first: it creates the request logger that
	// Recovery and the handlers use, and it writes the access log last.
	r.Use(RequestLogger(log, levels, adminToken), Recovery(log))

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	users := newUserStore()
	api := r.Group("/api")
	api.GET("/users/:id", users.get)
	api.POST("/orders", createOrder)
	api.GET("/panic", func(*gin.Context) { panic("something went badly wrong") }) // demo only

	if adminToken == "" {
		log.Notice("admin endpoints disabled; set ADMIN_TOKEN to enable them")
	} else {
		registerAdmin(r.Group("/admin", requireToken(adminToken)), r, log, levels)
	}
	return r
}

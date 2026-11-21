package main

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/aidnetic/logger-go"
)

// registerAdmin adds endpoints to inspect and change log levels at run time:
//
//	GET    /admin/log-levels                  the default level and every route's level
//	PUT    /admin/log-levels/default          {"level":"debug"}
//	PUT    /admin/log-levels/routes           {"route":"GET /api/users/:id","level":"debug"}
//	DELETE /admin/log-levels/routes?route=…   the route follows the default again
//
// Changes are written to the log at NOTICE, whatever the current level, so
// there is always a record of who changed what.
func registerAdmin(g *gin.RouterGroup, engine *gin.Engine, base *logger.Logger, levels *RouteLevels) {
	audit := func(c *gin.Context) *logger.Logger {
		return L(c).WithLevel(logger.NOTICE).With(logger.F("client_ip", c.ClientIP()))
	}

	g.GET("/log-levels", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"default": base.Level(), "routes": levels.All()})
	})

	g.PUT("/log-levels/default", func(c *gin.Context) {
		var req struct {
			Level *logger.Level `json:"level"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Level == nil {
			badRequest(c, `body must be {"level":"trace|debug|info|notice|warn|error|off"}`, err)
			return
		}
		old := base.Level()
		base.SetLevel(*req.Level)
		audit(c).Notice("default log level changed", logger.F("from", old), logger.F("to", *req.Level))
		c.JSON(http.StatusOK, gin.H{"default": *req.Level})
	})

	g.PUT("/log-levels/routes", func(c *gin.Context) {
		var req struct {
			Route string        `json:"route"`
			Level *logger.Level `json:"level"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Route == "" || req.Level == nil {
			badRequest(c, `body must be {"route":"GET /api/users/:id","level":"debug"}`, err)
			return
		}
		known := routeList(engine)
		if !contains(known, req.Route) {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown route", "routes": known})
			return
		}
		levels.Set(req.Route, *req.Level)
		audit(c).Notice("route log level changed", logger.F("route", req.Route), logger.F("to", *req.Level))
		c.JSON(http.StatusOK, gin.H{"route": req.Route, "level": *req.Level})
	})

	g.DELETE("/log-levels/routes", func(c *gin.Context) {
		route := c.Query("route")
		if route == "" {
			badRequest(c, "the route query parameter is required", nil)
			return
		}
		levels.Reset(route)
		audit(c).Notice("route log level reset", logger.F("route", route))
		c.Status(http.StatusNoContent)
	})
}

func badRequest(c *gin.Context, msg string, err error) {
	body := gin.H{"error": msg}
	if err != nil {
		body["detail"] = err.Error()
	}
	c.JSON(http.StatusBadRequest, body)
}

// routeList returns every registered route as "METHOD /pattern".
func routeList(engine *gin.Engine) []string {
	var out []string
	for _, r := range engine.Routes() {
		out = append(out, r.Method+" "+r.Path)
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

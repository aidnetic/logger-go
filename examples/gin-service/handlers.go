package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/aidnetic/logger-go"
)

var (
	errNotFound  = errors.New("not found")
	errDBTimeout = errors.New("database timeout")
)

type User struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

type userStore struct{ users map[string]User }

func newUserStore() *userStore {
	return &userStore{users: map[string]User{
		"1": {ID: "1", Name: "Ada Lovelace", Email: "ada@example.com", Roles: []string{"admin"}},
		"2": {ID: "2", Name: "Alan Turing", Email: "alan@example.com", Roles: []string{"billing"}},
	}}
}

// get handles GET /api/users/:id. User 13 simulates a database failure.
func (s *userStore) get(c *gin.Context) {
	log := L(c)
	id := c.Param("id")
	log.Debug("looking up user", logger.F("user_id", id))

	user, err := s.find(c.Request.Context(), id)
	switch {
	case errors.Is(err, errNotFound):
		log.Info("user not found", logger.F("user_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
	case err != nil:
		log.Error("user lookup failed", logger.Err(err), logger.F("user_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		log.Debug("user loaded", logger.F("user", user))
		c.JSON(http.StatusOK, user)
	}
}

// find stands in for a repository method. It only has a context, and still
// gets the request logger: same request ID, same route level.
func (s *userStore) find(ctx context.Context, id string) (User, error) {
	log := logger.FromContext(ctx)
	log.Trace("querying users table", logger.F("user_id", id))
	if id == "13" {
		return User{}, fmt.Errorf("query users: %w", errDBTimeout)
	}
	u, ok := s.users[id]
	if !ok {
		return User{}, errNotFound
	}
	return u, nil
}

type orderRequest struct {
	Item       string `json:"item" binding:"required"`
	Quantity   int    `json:"quantity" binding:"required,min=1"`
	CardNumber string `json:"card_number" binding:"required"`
}

// createOrder handles POST /api/orders.
func createOrder(c *gin.Context) {
	log := L(c)
	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("invalid order request", logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order", "detail": err.Error()})
		return
	}
	// card_number is passed as a field, so RedactKeys hides it in every format.
	log.Debug("order request",
		logger.F("item", req.Item),
		logger.F("quantity", req.Quantity),
		logger.F("card_number", req.CardNumber),
	)

	orderID := "ord_" + newRequestID()[:8]
	log.Notice("order created", logger.F("order_id", orderID), logger.F("item", req.Item), logger.F("quantity", req.Quantity))
	c.JSON(http.StatusCreated, gin.H{"order_id": orderID})
}

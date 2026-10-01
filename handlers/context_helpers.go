package handlers

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errNoUserID = errors.New("no authenticated user")

// requireUserID returns the authenticated user's ID, or an error if the
// request has none (caller must have gone through JWTAuthMiddleware).
func requireUserID(c *gin.Context) (uuid.UUID, error) {
	if id := optionalUserID(c); id != nil {
		return *id, nil
	}
	return uuid.Nil, errNoUserID
}

// optionalUserID returns the authenticated user's ID if OptionalAuthMiddleware
// (or JWTAuthMiddleware) set one on the context, or nil for an unauthenticated request.
func optionalUserID(c *gin.Context) *uuid.UUID {
	val, exists := c.Get("userID")
	if !exists {
		return nil
	}
	str, ok := val.(string)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(str)
	if err != nil {
		return nil
	}
	return &id
}

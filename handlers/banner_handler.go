package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type BannerHandler struct {
	service *services.BannerService
}

func NewBannerHandler(s *services.BannerService) *BannerHandler {
	return &BannerHandler{service: s}
}

func (h *BannerHandler) GetActiveBanners(c *gin.Context) {
	banners, err := h.service.GetActiveBanners()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": banners})
}

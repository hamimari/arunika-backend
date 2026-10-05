package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"arunika_backend/services"

	"github.com/gin-gonic/gin"
)

// maxUploadBody caps the multipart request; the per-kind limits (500 KB
// image, 300 KB audio) are enforced by the service on the file itself.
const maxUploadBody = 1 << 20

// AdminAssetHandler serves POST /admin/assets.
type AdminAssetHandler struct {
	assets *services.AssetService
}

func NewAdminAssetHandler(assets *services.AssetService) *AdminAssetHandler {
	return &AdminAssetHandler{assets: assets}
}

// Upload handles POST /admin/assets (multipart: file, kind=image|audio).
// 201 for a new asset, 200 when the same bytes were uploaded before.
func (h *AdminAssetHandler) Upload(c *gin.Context) {
	adminID, ok := adminIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBody)
	kind := c.PostForm("kind")
	header, err := c.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "file too large", "code": "INVALID_ASSET", "reason": services.AssetTooLarge})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	f, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read file"})
		return
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read file"})
		return
	}

	view, created, err := h.assets.Upload(c.Request.Context(), adminID, kind, header.Filename, data)
	var ae *services.AssetError
	switch {
	case errors.As(err, &ae):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "INVALID_ASSET", "reason": ae.Reason})
	case err != nil:
		slog.Error("asset upload failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
	case created:
		c.JSON(http.StatusCreated, gin.H{"data": view})
	default:
		c.JSON(http.StatusOK, gin.H{"data": view})
	}
}

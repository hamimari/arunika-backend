package handlers

import (
	"arunika_backend/models"
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
	"gorm.io/gorm"
)

type PrintableCardHandler struct {
	db *gorm.DB
}

func NewPrintableCardHandler(db *gorm.DB) *PrintableCardHandler {
	return &PrintableCardHandler{db: db}
}

// GetPrintablePDF generates an A4 PDF with AR card images for all cards in a category.
// Cards are laid out 2 columns × 2 rows per page, each cell 75×110 mm with 2 mm gutters.
func (h *PrintableCardHandler) GetPrintablePDF(c *gin.Context) {
	categoryID := c.Query("category_id")
	if categoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category_id is required"})
		return
	}

	cards, err := models.FindAllCards(h.db, categoryID, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(cards) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no cards found for this category"})
		return
	}

	// A4: 210×297 mm
	// Card cell: 75×110 mm
	// 2 cols, 2 rows per page
	// Gutters: 2 mm
	// Total width used: 2*75 + 3*2 = 156 mm → left margin = (210-156)/2 = 27 mm
	// Total height used: 2*110 + 3*2 = 226 mm → top margin = (297-226)/2 = 35.5 mm

	const (
		cardW   = 75.0
		cardH   = 110.0
		gutter  = 2.0
		cols    = 2
		rows    = 2
		marginL = (210.0 - float64(cols)*cardW - float64(cols+1)*gutter) / 2.0
		marginT = (297.0 - float64(rows)*cardH - float64(rows+1)*gutter) / 2.0
		titleH  = 10.0 // reserved at bottom of each card for the title
		imageH  = cardH - titleH - 4.0
	)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Helvetica", "", 8)

	cardsPerPage := cols * rows
	for i, card := range cards {
		if i%cardsPerPage == 0 {
			pdf.AddPage()
		}
		slot := i % cardsPerPage
		col := slot % cols
		row := slot / cols

		x := marginL + float64(col)*(cardW+gutter) + gutter
		y := marginT + float64(row)*(cardH+gutter) + gutter

		// Card border
		pdf.SetDrawColor(200, 200, 200)
		pdf.SetFillColor(248, 248, 248)
		pdf.RoundedRect(x, y, cardW, cardH, 3, "1234", "FD")

		// Try to fetch and embed the printable image.
		// PrintableImg takes priority; falls back to ImageURL when not set.
		imgURL := card.PrintableImg
		if imgURL == "" {
			imgURL = card.ImageURL
		}
		if imgURL != "" {
			imgData := fetchImageBytes(imgURL)
			if imgData != nil {
				imgOpt := fpdf.ImageOptions{ImageType: detectImageType(imgData), ReadDpi: false}
				imgReader := bytes.NewReader(imgData)
				imgName := fmt.Sprintf("card_%s", card.ID)
				pdf.RegisterImageOptionsReader(imgName, imgOpt, imgReader)
				imgAreaH := imageH
				imgAreaW := cardW - 4.0
				pdf.ImageOptions(imgName, x+2, y+2, imgAreaW, imgAreaH, false, imgOpt, 0, "")
			} else {
				// Grey placeholder
				pdf.SetFillColor(200, 200, 200)
				pdf.Rect(x+2, y+2, cardW-4, imageH, "F")
			}
		} else {
			pdf.SetFillColor(220, 220, 220)
			pdf.Rect(x+2, y+2, cardW-4, imageH, "F")
		}

		// Title
		pdf.SetXY(x+2, y+cardH-titleH+1)
		pdf.SetTextColor(50, 50, 50)
		pdf.SetFont("Helvetica", "B", 7)
		pdf.CellFormat(cardW-4, 8, truncate(card.Title, 30), "", 0, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate PDF"})
		return
	}

	c.Header("Content-Disposition", `attachment; filename="kartu-ar.pdf"`)
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

// fetchImageBytes fetches image bytes from a URL with a 5 s timeout.
// Returns nil on failure.
func fetchImageBytes(url string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil
	}
	b := buf.Bytes()
	// Validate that it is a decodable image
	_, _, err = image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	return b
}

// detectImageType sniffs the image type from its bytes.
func detectImageType(b []byte) string {
	if len(b) >= 4 && b[0] == 0x89 && b[1] == 0x50 {
		return "PNG"
	}
	return "JPG"
}

// truncate truncates a string to max n runes.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n-1]) + "…"
	}
	return s
}

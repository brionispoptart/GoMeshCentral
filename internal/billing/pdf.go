package billing

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/png"
	"os"

	"gomeshcentral/internal/storage"

	"github.com/jung-kurt/gofpdf"
)

// GenerateInvoicePDF creates a professional PDF invoice using company branding.
// It returns the PDF bytes as a buffer.
func GenerateInvoicePDF(org *storage.Organization, client *storage.Client, invoice *storage.Invoice, branding *storage.Branding) (*bytes.Buffer, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	if pdf.Err() {
		return nil, fmt.Errorf("failed to create PDF")
	}

	pdf.AddPage()
	pdf.SetFont("Arial", "", 11)

	// Get company name from branding if available
	companyName := org.Name
	if branding != nil && branding.CompanyName != "" {
		companyName = branding.CompanyName
	}
	if companyName == "" {
		companyName = "GoMeshCentral"
	}

	// Top section: Logo + Company Info (no colored band)
	currentY := 5.0

	// Logo (if available)
	logoWidth := 50.0
	if branding != nil && branding.Logo != "" {
		// Decode base64 logo and embed it
		logoData := branding.Logo

		// Handle data URL format (e.g., "data:image/png;base64,...")
		if len(logoData) > 23 && logoData[:23] == "data:image/png;base64," {
			logoData = logoData[23:]
		} else if len(logoData) > 22 {
			// Check for other image formats
			if idx := len("data:image/"); idx < len(logoData) {
				for i := idx; i < len(logoData); i++ {
					if logoData[i] == ',' {
						logoData = logoData[i+1:]
						break
					}
				}
			}
		}

		logoBytes, err := base64.StdEncoding.DecodeString(logoData)
		if err == nil && len(logoBytes) > 0 {
			// Get image dimensions to calculate proper aspect ratio
			img, _, err := image.DecodeConfig(bytes.NewReader(logoBytes))
			if err == nil && img.Width > 0 && img.Height > 0 {
				// Calculate proportional height
				aspectRatio := float64(img.Height) / float64(img.Width)
				logoHeight := logoWidth * aspectRatio

				// Create a temporary file for the logo
				tempFile, err := os.CreateTemp("", "logo-*.png")
				if err == nil {
					defer os.Remove(tempFile.Name()) // Clean up after PDF generation

					// Write decoded image data to temp file
					if _, err := tempFile.Write(logoBytes); err == nil {
						tempFile.Close()

						// Embed the image in the PDF with calculated proportional dimensions
						// Center the logo horizontally (A4 width = 210mm)
						logoX := (210.0 - logoWidth) / 2.0
						if !pdf.Err() {
							pdf.Image(tempFile.Name(), logoX, currentY, logoWidth, logoHeight, false, "", 0, "")
							currentY += logoHeight + 2
						}
					} else {
						tempFile.Close()
					}
				}
			}
		}
	}

	// Company name and contact info
	pdf.SetFont("Arial", "B", 13)
	pdf.SetTextColor(50, 50, 50)
	pdf.SetXY(10, currentY)
	pdf.Cell(0, 6, companyName)
	currentY += 6

	// Company contact information
	if branding != nil {
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(100, 100, 100)

		if branding.PhoneNumber != "" {
			pdf.SetXY(10, currentY)
			pdf.Cell(0, 3, branding.PhoneNumber)
			currentY += 3
		}
		if branding.Email != "" {
			pdf.SetXY(10, currentY)
			pdf.Cell(0, 3, branding.Email)
			currentY += 3
		}
		if branding.Website != "" {
			pdf.SetXY(10, currentY)
			pdf.Cell(0, 3, branding.Website)
			currentY += 3
		}
	}

	currentY += 4

	// Invoice label and info on the right
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(50, 50, 50)
	pdf.SetXY(130, currentY-3)
	pdf.Cell(0, 5, "Invoice")

	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(130, currentY+2)
	pdf.Cell(20, 3, "Invoice #")
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(0, 3, invoice.InvoiceNumber)
	pdf.Ln(3)

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(100, 100, 100)
	pdf.SetXY(130, pdf.GetY())
	pdf.Cell(0, 3, invoice.IssueDate.Format("Jan 02, 2006"))
	pdf.Ln(3)

	// Move to main content
	pdf.SetY(currentY + 8)

	// "From:" section on the left
	pdf.SetXY(10, pdf.GetY())
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(50, 50, 50)
	pdf.Cell(0, 4, "From:")
	pdf.Ln(4)

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(50, 50, 50)
	pdf.SetXY(10, pdf.GetY())
	pdf.Cell(0, 4, companyName)
	pdf.Ln(4)

	// Company contact in From section
	if branding != nil && branding.Email != "" {
		pdf.SetXY(10, pdf.GetY())
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(100, 100, 100)
		pdf.Cell(0, 3, branding.Email)
		pdf.Ln(3)
	}

	if branding != nil && branding.PhoneNumber != "" {
		pdf.SetXY(10, pdf.GetY())
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(100, 100, 100)
		pdf.Cell(0, 3, branding.PhoneNumber)
		pdf.Ln(3)
	}

	// "To:" section on the right - at the same level as From
	fromY := currentY + 8.0
	pdf.SetXY(120, fromY)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(50, 50, 50)
	pdf.Cell(0, 4, "To:")
	pdf.Ln(4)

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(50, 50, 50)

	if client != nil {
		if client.Name != "" {
			pdf.SetXY(120, pdf.GetY())
			pdf.Cell(0, 4, client.Name)
			pdf.Ln(4)
		}
		if client.Address != "" {
			pdf.SetXY(120, pdf.GetY())
			pdf.Cell(0, 4, client.Address)
			pdf.Ln(4)
		}
		if client.ContactEmail != "" {
			pdf.SetXY(120, pdf.GetY())
			pdf.Cell(0, 4, client.ContactEmail)
			pdf.Ln(4)
		}
		if client.ContactPhone != "" {
			pdf.SetXY(120, pdf.GetY())
			pdf.Cell(0, 4, client.ContactPhone)
			pdf.Ln(4)
		}
	}

	// Add spacing before line items
	pdf.SetY(pdf.GetY() + 4)

	// Line items table header
	pdf.SetFillColor(245, 245, 245)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(50, 50, 50)

	// Table headers: #, Category, Product/Rate name, Qty, Rate, Subtotal, Tax name, Tax rate
	pdf.SetXY(10, pdf.GetY())
	pdf.Cell(4, 5, "#")
	pdf.Cell(15, 5, "Category")
	pdf.Cell(55, 5, "Product/Rate name")
	pdf.Cell(12, 5, "Qty")
	pdf.Cell(15, 5, "Rate")
	pdf.Cell(20, 5, "Subtotal")
	pdf.Cell(30, 5, "Tax name")
	pdf.Cell(14, 5, "Tax rate")
	pdf.Ln(5)

	// Line items
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(33, 33, 33)
	itemNum := 1
	total := 0.0

	for _, item := range invoice.LineItems {
		pdf.SetXY(10, pdf.GetY())
		pdf.Cell(4, 5, fmt.Sprintf("%d", itemNum))
		pdf.Cell(15, 5, "Labor")

		desc := item.Description
		if len(desc) > 40 {
			desc = desc[:40]
		}
		pdf.Cell(55, 5, desc)

		qtyStr := formatQuantity(item.Quantity)
		pdf.Cell(12, 5, qtyStr)

		pdf.Cell(15, 5, fmt.Sprintf("$%.2f", item.UnitPrice))

		subtotal := item.Quantity * item.UnitPrice
		pdf.Cell(20, 5, fmt.Sprintf("$%.2f", subtotal))

		pdf.Cell(30, 5, "-")
		pdf.Cell(14, 5, "-")

		pdf.Ln(5)
		total += subtotal
		itemNum++
	}

	// Spacing before totals
	pdf.SetY(pdf.GetY() + 2)

	// Totals section (right-aligned)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(50, 50, 50)

	// Subtotal
	pdf.SetX(120)
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(30, 5, "Subtotal:")
	pdf.Cell(0, 5, fmt.Sprintf("$%.2f", total))
	pdf.Ln(5)

	// Tax
	pdf.SetX(120)
	pdf.Cell(30, 5, "Tax:")
	pdf.Cell(0, 5, fmt.Sprintf("$%.2f", invoice.Tax))
	pdf.Ln(5)

	// Total (emphasized)
	pdf.SetX(120)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(30, 6, "Total:")
	pdf.Cell(0, 6, fmt.Sprintf("$%.2f", invoice.Total))
	pdf.Ln(6)

	// Generate PDF to buffer
	buf := new(bytes.Buffer)
	err := pdf.Output(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return buf, nil
}

// formatQuantity formats a quantity value, removing trailing zeros and decimal point if not needed
func formatQuantity(q float64) string {
	if q == float64(int64(q)) {
		return fmt.Sprintf("%.0f", q)
	}
	return fmt.Sprintf("%.2f", q)
}

// sumWidths calculates the cumulative width up to a given column index
func sumWidths(widths []float64, upToIndex int) float64 {
	sum := 0.0
	for i := 0; i < upToIndex && i < len(widths); i++ {
		sum += widths[i]
	}
	return sum
}

// GenerateInvoiceFileName creates a filename for the invoice in Atera's format.
// Format: {InvoiceNumber}__Invoice__{timestamp}.pdf
func GenerateInvoiceFileName(invoice *storage.Invoice) string {
	timestamp := invoice.IssueDate.Format("2006_01_02_15_04_05")
	return fmt.Sprintf("%s__Invoice__%s_async.pdf", invoice.InvoiceNumber, timestamp)
}

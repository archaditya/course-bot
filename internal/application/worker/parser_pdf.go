package worker

import (
	"encoding/json"
	"fmt"

	"archadilm/internal/domain/entities"
	"archadilm/internal/infrastructure/llm"
)

// parsePDF sends raw PDF bytes to the AI Service's /extract-pdf endpoint
// and converts the response into a NormalizedDocument.
//
// The AI Service uses PyMuPDF (fitz) to extract text page-by-page. This
// keeps the Go worker free of cgo/poppler dependencies while still getting
// production-quality extraction including tables and OCR fallback.
func parsePDF(rawData []byte, doc *entities.Document, aiClient *llm.Client) (*entities.NormalizedDocument, error) {
	// Call AI Service to extract text from PDF
	pages, err := aiClient.ExtractPDF(rawData)
	if err != nil {
		return nil, fmt.Errorf("pdf: ai service extraction: %w", err)
	}

	nd := &entities.NormalizedDocument{
		Language:             "en",
		SourceRef:            doc.StoragePath,
		Timeline:             false,
		NormalizationVersion: NormalizationVersion,
	}
	nd.Metadata.SourceType = doc.SourceType
	nd.Metadata.OriginalFilename = doc.OriginalFilename
	nd.Metadata.Checksum = doc.Checksum

	for i, page := range pages {
		if page.Text == "" {
			continue
		}
		pageNum := i + 1
		nd.Segments = append(nd.Segments, entities.Segment{
			SegmentID: fmt.Sprintf("page-%d", pageNum),
			Text:      page.Text,
			Page:      &pageNum,
		})
	}

	if len(nd.Segments) == 0 {
		return nil, fmt.Errorf("pdf: no text extracted; file may be image-only or corrupt")
	}

	return nd, nil
}

// PDFPage is one page of extracted text from the AI Service.
type PDFPage struct {
	PageNumber int    `json:"page_number"`
	Text       string `json:"text"`
}

// parsePDFPages is a helper that unmarshals the AI Service response.
func parsePDFPages(data []byte) ([]PDFPage, error) {
	var pages []PDFPage
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("pdf: unmarshal pages: %w", err)
	}
	return pages, nil
}

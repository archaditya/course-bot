package worker

import (
	"context"
	"fmt"

	"archadilm/internal/domain/entities"
	"archadilm/internal/infrastructure/llm"
)

// parseURL fetches and extracts readable text content from a web URL.
// Delegates to the AI Service's /extract-url endpoint which uses
// BeautifulSoup + readability extraction.
func parseURL(doc *entities.Document, aiClient *llm.Client, allowedDomains []string) (*entities.NormalizedDocument, error) {
    if doc.SourceURL == "" {
        return nil, fmt.Errorf("url: document has no source URL")
    }
 
    extracted, err := aiClient.ExtractURL(context.Background(), doc.SourceURL, allowedDomains)
    if err != nil {
        return nil, fmt.Errorf("url: extraction: %w", err)
    }

	nd := &entities.NormalizedDocument{
		Language:             "en",
		SourceRef:            doc.SourceURL,
		Timeline:             false,
		NormalizationVersion: NormalizationVersion,
	}
	nd.Metadata.SourceType = doc.SourceType
	nd.Metadata.OriginalFilename = doc.OriginalFilename
	nd.Metadata.Checksum = doc.Checksum

	// Split extracted content into segments by paragraph/section.
	// The AI Service returns pre-segmented content.
	for i, section := range extracted.Sections {
		if section.Text == "" {
			continue
		}
		nd.Segments = append(nd.Segments, entities.Segment{
			SegmentID: fmt.Sprintf("section-%d", i+1),
			Text:      section.Text,
		})
	}

	if len(nd.Segments) == 0 {
		return nil, fmt.Errorf("url: no text extracted from %s", doc.SourceURL)
	}

	return nd, nil
}

package worker

import (
	"fmt"
	"strings"

	"archadilm/internal/domain/entities"
)

// parseText creates a NormalizedDocument from raw text content.
// Segments are split by double newlines (paragraphs). This handles the
// "paste raw text" source type from the UI.
func parseText(data []byte, doc *entities.Document) (*entities.NormalizedDocument, error) {
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, fmt.Errorf("text: empty content")
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

	// Split by double newlines to create paragraph-level segments.
	// Single newlines within a paragraph are preserved.
	paragraphs := strings.Split(content, "\n\n")
	segIdx := 0
	for _, para := range paragraphs {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" {
			continue
		}
		segIdx++
		nd.Segments = append(nd.Segments, entities.Segment{
			SegmentID: fmt.Sprintf("para-%d", segIdx),
			Text:      trimmed,
		})
	}

	if len(nd.Segments) == 0 {
		return nil, fmt.Errorf("text: no segments after splitting")
	}

	return nd, nil
}

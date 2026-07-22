package worker

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"archadilm/internal/domain/entities"
)

// parseVTT parses a WebVTT (.vtt) subtitle file into a NormalizedDocument.
// VTT is similar to SRT but has a WEBVTT header and uses '.' instead of ','
// for milliseconds. Cue settings (position, alignment) are ignored.
func parseVTT(data []byte, doc *entities.Document) (*entities.NormalizedDocument, error) {
	nd := &entities.NormalizedDocument{
		Language:             "en",
		SourceRef:            doc.StoragePath,
		Timeline:             true,
		NormalizationVersion: NormalizationVersion,
	}
	nd.Metadata.SourceType = doc.SourceType
	nd.Metadata.OriginalFilename = doc.OriginalFilename
	nd.Metadata.Checksum = doc.Checksum

	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Skip the WEBVTT header and any metadata lines before the first cue.
	headerParsed := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !headerParsed {
			if strings.HasPrefix(line, "WEBVTT") {
				headerParsed = true
			}
			continue
		}
		// Skip blank lines and NOTE blocks between header and first cue
		if line == "" || strings.HasPrefix(line, "NOTE") {
			continue
		}
		// We've hit the first cue — break and process below
		break
	}

	if !headerParsed {
		return nil, fmt.Errorf("vtt: missing WEBVTT header")
	}

	var (
		segIdx  int
		state   int // 0=expecting timing, 1=reading text
		seg     entities.Segment
		textBuf []string
	)

	flush := func() {
		if len(textBuf) > 0 {
			seg.Text = strings.TrimSpace(strings.Join(textBuf, " "))
			nd.Segments = append(nd.Segments, seg)
			textBuf = nil
		}
	}

	// Re-process from current scanner position (after header skip).
	// The scanner already consumed the first non-header line above.
	// We need to check if the current line is a timing line.
	currentLine := strings.TrimSpace(scanner.Text())
	if strings.Contains(currentLine, "-->") {
		// It's a timing line; process it
		segIdx++
		seg = entities.Segment{SegmentID: fmt.Sprintf("seg-%d", segIdx)}
		start, end, err := parseVTTTimeline(currentLine)
		if err != nil {
			return nil, fmt.Errorf("vtt: timing at segment %d: %w", segIdx, err)
		}
		seg.StartTS = &start
		seg.EndTS = &end
		state = 1
	}
	// Otherwise it might be a cue identifier (number or string), skip it
	// and look for the next timing line.

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch state {
		case 0: // expecting timing or cue id
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "NOTE") {
				// Skip NOTE blocks
				for scanner.Scan() {
					if strings.TrimSpace(scanner.Text()) == "" {
						break
					}
				}
				continue
			}
			if strings.Contains(line, "-->") {
				segIdx++
				seg = entities.Segment{SegmentID: fmt.Sprintf("seg-%d", segIdx)}
				start, end, err := parseVTTTimeline(line)
				if err != nil {
					return nil, fmt.Errorf("vtt: timing at segment %d: %w", segIdx, err)
				}
				seg.StartTS = &start
				seg.EndTS = &end
				state = 1
			}
			// else: it's a cue identifier, ignore

		case 1: // reading text
			if line == "" {
				flush()
				state = 0
				continue
			}
			// Strip VTT formatting tags like <b>, <i>, <v Speaker>
			cleaned := stripVTTTags(line)
			textBuf = append(textBuf, cleaned)
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("vtt: scan: %w", err)
	}
	if len(nd.Segments) == 0 {
		return nil, fmt.Errorf("vtt: no segments found; file may be empty or corrupt")
	}
	return nd, nil
}

// parseVTTTimeline parses "00:01:23.456 --> 00:01:25.789" or "01:23.456 --> 01:25.789"
func parseVTTTimeline(line string) (int, int, error) {
	// Remove cue settings (everything after the end time that isn't a timestamp)
	parts := strings.Split(line, "-->")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected timing line, got %q", line)
	}
	startSec, err := parseVTTTime(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("start: %w", err)
	}
	// End time may have cue settings appended; take only the timestamp part
	endPart := strings.TrimSpace(parts[1])
	endFields := strings.Fields(endPart)
	endSec, err := parseVTTTime(endFields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("end: %w", err)
	}
	return startSec, endSec, nil
}

// parseVTTTime handles both "HH:MM:SS.mmm" and "MM:SS.mmm" formats.
func parseVTTTime(s string) (int, error) {
	s = strings.TrimSpace(s)
	colonParts := strings.SplitN(s, ":", 3)

	switch len(colonParts) {
	case 3: // HH:MM:SS.mmm
		h, _ := strconv.Atoi(colonParts[0])
		m, _ := strconv.Atoi(colonParts[1])
		secParts := strings.SplitN(colonParts[2], ".", 2)
		sec, _ := strconv.Atoi(secParts[0])
		return h*3600 + m*60 + sec, nil

	case 2: // MM:SS.mmm
		m, _ := strconv.Atoi(colonParts[0])
		secParts := strings.SplitN(colonParts[1], ".", 2)
		sec, _ := strconv.Atoi(secParts[0])
		return m*60 + sec, nil

	default:
		return 0, fmt.Errorf("unexpected time format: %q", s)
	}
}

// stripVTTTags removes VTT/HTML-like tags from cue text.
func stripVTTTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

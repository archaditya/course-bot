package worker

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"archadilm/internal/domain/entities"
)

// parseSRT parses a SubRip (.srt) transcript into a NormalizedDocument.
func parseSRT(data []byte, doc *entities.Document) (*entities.NormalizedDocument, error) {
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
	var (
		segIdx  int
		state   int
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

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch state {
		case 0:
			if line == "" {
				continue
			}
			idx, err := strconv.Atoi(line)
			if err != nil {
				continue
			}
			segIdx = idx
			seg = entities.Segment{SegmentID: fmt.Sprintf("seg-%d", segIdx)}
			state = 1

		case 1:
			if line == "" {
				state = 0
				continue
			}
			parts := strings.Split(line, " --> ")
			if len(parts) != 2 {
				return nil, fmt.Errorf("srt: malformed timing at segment %d: %q", segIdx, line)
			}
			startSec, err := parseSRTTime(parts[0])
			if err != nil {
				return nil, fmt.Errorf("srt: start time at %d: %w", segIdx, err)
			}
			endSec, err := parseSRTTime(parts[1])
			if err != nil {
				return nil, fmt.Errorf("srt: end time at %d: %w", segIdx, err)
			}
			seg.StartTS = &startSec
			seg.EndTS = &endSec
			state = 2

		case 2:
			if line == "" {
				flush()
				state = 0
				continue
			}
			textBuf = append(textBuf, line)
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("srt: scan: %w", err)
	}
	if len(nd.Segments) == 0 {
		return nil, fmt.Errorf("srt: no segments found; file may be empty or corrupt")
	}
	return nd, nil
}

func parseSRTTime(s string) (int, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return 0, fmt.Errorf("expected HH:MM:SS,mmm got %q", s)
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	secParts := strings.SplitN(parts[2], ".", 2)
	sec, _ := strconv.Atoi(secParts[0])
	return h*3600 + m*60 + sec, nil
}
package server

import (
	"fmt"
	"strconv"
	"strings"
)

// HTTPRange represents a byte range for partial content
type HTTPRange struct {
	Start  int64
	End    int64
	Length int64
}

// ParseRange parses HTTP Range header
// Format: "bytes=start-end" or "bytes=start-" or "bytes=-suffix"
func ParseRange(rangeHeader string, contentLength int64) (*HTTPRange, error) {
	if rangeHeader == "" {
		// No range header, return full content
		return &HTTPRange{
			Start:  0,
			End:    contentLength - 1,
			Length: contentLength,
		}, nil
	}

	// Remove "bytes=" prefix
	const bytesPrefix = "bytes="
	if !strings.HasPrefix(rangeHeader, bytesPrefix) {
		return nil, fmt.Errorf("invalid range header format")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, bytesPrefix)

	// Split on comma to handle multiple ranges (we'll only support single range for now)
	ranges := strings.Split(rangeSpec, ",")
	if len(ranges) > 1 {
		return nil, fmt.Errorf("multiple ranges not supported")
	}

	// Parse start-end
	parts := strings.Split(strings.TrimSpace(ranges[0]), "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format")
	}

	var start, end int64
	var err error

	if parts[0] == "" {
		// Suffix range: "-500" means last 500 bytes
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid suffix range: %w", err)
		}
		start = contentLength - suffix
		if start < 0 {
			start = 0
		}
		end = contentLength - 1
	} else if parts[1] == "" {
		// Open-ended range: "500-" means from 500 to end
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid start range: %w", err)
		}
		end = contentLength - 1
	} else {
		// Both start and end specified: "500-999"
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid start range: %w", err)
		}
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid end range: %w", err)
		}
	}

	// Validate range
	if start < 0 || end < start || end >= contentLength {
		return nil, fmt.Errorf("range not satisfiable")
	}

	return &HTTPRange{
		Start:  start,
		End:    end,
		Length: end - start + 1,
	}, nil
}

// ContentRange returns the Content-Range header value
func (r *HTTPRange) ContentRange(totalSize int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, totalSize)
}

package lsp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadMessage reads a single LSP/JSON-RPC message framed with Content-Length headers.
// It returns the raw JSON payload bytes.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	var contentLen int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", v, err)
			}
			contentLen = n
		}
	}
	if contentLen <= 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// WriteMessage writes a single LSP/JSON-RPC message framed with Content-Length headers.
func WriteMessage(w io.Writer, payload []byte) error {
	var hdr bytes.Buffer
	_, _ = fmt.Fprintf(&hdr, "Content-Length: %d\r\n\r\n", len(payload))
	if _, err := w.Write(hdr.Bytes()); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

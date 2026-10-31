package proxy

import (
	"bytes"
	"io"

	"github.com/rejisterjack/fuse/internal/provider"
)

// teeReadCloser tees reads into a buffer while passing through to the client.
type teeReadCloser struct {
	r   io.ReadCloser
	buf *bytes.Buffer
}

func (t *teeReadCloser) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		_, _ = t.buf.Write(p[:n])
	}
	return n, err
}

func (t *teeReadCloser) Close() error {
	return t.r.Close()
}

// parseStreamUsage extracts usage via the given provider from buffered SSE bytes.
func parseStreamUsage(p provider.Provider, buf []byte) (provider.Usage, bool) {
	return p.ParseStreamUsage(buf)
}

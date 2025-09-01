package request

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chunkReader simulates a network stream that only returns
// numBytesPerRead bytes at a time (useful for testing incremental parsing).
type chunkReader struct {
	data            string
	numBytesPerRead int
	pos             int
}

func (cr *chunkReader) Read(p []byte) (int, error) {
	if cr.pos >= len(cr.data) {
		return 0, io.EOF
	}

	// how many bytes to read this time
	n := cr.numBytesPerRead
	if len(p) < n {
		n = len(p)
	}
	if cr.pos+n > len(cr.data) {
		n = len(cr.data) - cr.pos
	}

	copy(p, cr.data[cr.pos:cr.pos+n])
	cr.pos += n
	return n, nil
}

// ----------------------------
// 🔥 TESTS START HERE
// ----------------------------

// ✅ 1. Request with no body
func TestRequest_NoBody(t *testing.T) {
	reader := &chunkReader{
		data: "GET /coffee HTTP/1.1\r\n" +
			"Host: example.com\r\n" +
			"X-Custom: value1\r\n" +
			"X-Custom: value2\r\n" +
			"\r\n", // no body
		numBytesPerRead: 1,
	}

	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Full check
	assert.Equal(t, "GET", r.RequestLine.Method)
	assert.Equal(t, "/coffee", r.RequestLine.RequestTarget)
	assert.Equal(t, "1.1", r.RequestLine.HttpVersion)

	assert.Equal(t, []string{"example.com"}, r.Headers["Host"])
	assert.Equal(t, []string{"value1", "value2"}, r.Headers["X-Custom"])

	// No body expected
	assert.Empty(t, r.Body)

	// Pretty print
	fmt.Println("\n=== Parsed Request (No Body) ===")
	fmt.Printf("Method: %s, Path: %s, Version: %s\n", r.RequestLine.Method, r.RequestLine.RequestTarget, r.RequestLine.HttpVersion)
	for k, v := range r.Headers {
		fmt.Printf("  %s: %v\n", k, v)
	}
	fmt.Printf("Body: %q\n", string(r.Body))
}

// ✅ 2. Request with Content-Length body
func TestRequest_ContentLengthBody(t *testing.T) {
	reader := &chunkReader{
		data: "POST /submit HTTP/1.1\r\n" +
			"Host: example.com\r\n" +
			"Content-Length: 11\r\n" +
			"\r\n" +
			"hello=world",
		numBytesPerRead: 2,
	}

	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Full check
	assert.Equal(t, "POST", r.RequestLine.Method)
	assert.Equal(t, "/submit", r.RequestLine.RequestTarget)

	assert.Equal(t, []string{"example.com"}, r.Headers["Host"])
	assert.Equal(t, "hello=world", string(r.Body))

	// Pretty print
	fmt.Println("\n=== Parsed Request (Content-Length) ===")
	fmt.Printf("Method: %s, Path: %s\n", r.RequestLine.Method, r.RequestLine.RequestTarget)
	for k, v := range r.Headers {
		fmt.Printf("  %s: %v\n", k, v)
	}
	fmt.Printf("Body: %q\n", string(r.Body))
}

// ✅ 3. Request with Chunked Encoding body
func TestRequest_ChunkedBody(t *testing.T) {
	reader := &chunkReader{
		data: "POST /data HTTP/1.1\r\n" +
			"Host: example.com\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"5\r\nhello\r\n" +
			"6\r\n world\r\n" +
			"0\r\n\r\n",
		numBytesPerRead: 3,
	}

	r, err := RequestFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Full check
	assert.Equal(t, "POST", r.RequestLine.Method)
	assert.Equal(t, "/data", r.RequestLine.RequestTarget)

	assert.Equal(t, []string{"example.com"}, r.Headers["Host"])
	assert.Equal(t, "hello world", string(r.Body))

	// Pretty print
	fmt.Println("\n=== Parsed Request (Chunked) ===")
	fmt.Printf("Method: %s, Path: %s\n", r.RequestLine.Method, r.RequestLine.RequestTarget)
	for k, v := range r.Headers {
		fmt.Printf("  %s: %v\n", k, v)
	}
	fmt.Printf("Body: %q\n", string(r.Body))
}

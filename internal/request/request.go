package request

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type parseState int
type chunkBody int

type Request struct {
	RequestLine RequestLine
	Headers     Headers
	Body        Body
	state       parseState
}

type ChunkDecoder struct {
	chunkSize    int64
	crlfRead     bool
	dataRead     bool
	lastByteRead int
	data         []byte
}

type Headers map[string][]string
type Body []byte

type RequestLine struct {
	HttpVersion   string
	RequestTarget string
	Method        string
}

const (
	intState parseState = iota
	stateRequestLine
	stateHeaders
	stateDone
)

func (r *RequestLine) validHTTP() bool {
	if r.HttpVersion == "HTTP/1.1" {
		return true
	}
	return false
}

var ERROR_BAD_START_LINE = fmt.Errorf("bad start line")
var ERROR_BAD_HTTP_VERISON = fmt.Errorf("malformed http version")
var ERROR_BAD_HEADER = fmt.Errorf("malformed http header")
var SEPARATOR = "\r\n"

func parseRequestLine(b string, req *Request) (*RequestLine, int, error) {
	idx := strings.Index(b, SEPARATOR)
	if idx == -1 {
		return nil, 0, nil // incomplete line
	}

	firstLine := b[:idx]

	data := strings.Split(firstLine, " ")
	if len(data) != 3 {
		return nil, 0, ERROR_BAD_START_LINE
	}

	httprq := strings.Split(data[2], "/")
	if len(httprq) != 2 || httprq[0] != "HTTP" || httprq[1] != "1.1" {
		return nil, 0, ERROR_BAD_HTTP_VERISON
	}

	req.state = stateRequestLine
	reqsline := RequestLine{
		Method:        data[0],
		RequestTarget: data[1],
		HttpVersion:   httprq[1],
	}

	return &reqsline, len(firstLine) + len(SEPARATOR), nil
}
func parseHeader(b string) (Headers, int, error) {

	idx := strings.Index(b, "\r\n\r\n")
	if idx == -1 {
		return nil, 0, nil

	}
	headerStr := b[:idx]
	splitted := strings.Split(headerStr, "\r\n")
	h := make(Headers)

	for i := 0; i < len(splitted); i++ {
		parts := strings.SplitN(splitted[i], ":", 2)
		if len(parts) != 2 {
			return nil, 0, ERROR_BAD_HEADER

		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		h[key] = append(h[key], value)

	}
	return h, idx + 4, nil

}

func ByteBody(data []byte, length int) (*Body, int, error) {
	if len(data) != length {
		return nil, 0, nil
	}

	body := make([]byte, length)
	copy(body, data)

	b := Body(body)
	return &b, length, nil
}

func NewChunkDecoder() *ChunkDecoder {
	return &ChunkDecoder{
		crlfRead: true, // first we expect size line
	}
}

func (d *ChunkDecoder) Feed(body []byte) ([]byte, int, error) {
	fmt.Println("body:", body)
	for {
		if d.crlfRead {
			sliced := body[d.lastByteRead:]
			if len(sliced) == 0 {
				return nil, d.lastByteRead, nil
			}

			idx := bytes.Index(sliced, []byte("\r\n"))
			if idx == -1 {
				return nil, d.lastByteRead, nil
			}

			sizeStr := string(sliced[:idx])
			size, err := strconv.ParseInt(sizeStr, 16, 64)
			if err != nil {
				return nil, d.lastByteRead, fmt.Errorf("invalid chunk size: %v", err)
			}
			d.chunkSize = size

			if d.chunkSize == 0 {
				if len(sliced) < idx+4 { // "0\r\n\r\n"
					return nil, d.lastByteRead, nil // wait for more
				}
				return d.data, d.lastByteRead + idx + 4, nil
			}

			d.lastByteRead += idx + 2
			d.crlfRead = false
			d.dataRead = true
		}

		if d.dataRead {
			if len(body[d.lastByteRead:]) < int(d.chunkSize)+2 {
				return nil, d.lastByteRead, nil
			}

			parts := body[d.lastByteRead : d.lastByteRead+int(d.chunkSize)]
			d.data = append(d.data, parts...)

			d.lastByteRead += int(d.chunkSize) + 2

			d.crlfRead = true
			d.dataRead = false
		}
	}
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	req := &Request{state: intState}
	decoder := NewChunkDecoder()
	buff := make([]byte, 0, 8) // main buffer
	tem := make([]byte, 4)     // temp buffer for reading chunks

	for {
		fmt.Println("entring loop")
		n, err := reader.Read(tem)
		if n > 0 {
			fmt.Printf("Read chunk: %q\n", tem[:n])
			buff = append(buff, tem[:n]...)
		}

		if req.state == intState {
			reqline, consumed, perr := parseRequestLine(string(buff), req)
			if perr != nil {
				return nil, perr
			}
			if consumed > 0 {
				// trim consumed bytes from buffer
				buff = buff[consumed:]

				req.RequestLine = *reqline
			}
		}

		if req.state == stateRequestLine {

			headers, consumed, herr := parseHeader(string(buff))
			if herr != nil {
				return nil, herr
			}

			if consumed > 0 {
				// trim consumed bytes from buffer
				buff = buff[consumed:]

				req.Headers = headers
				req.state = stateHeaders
			}
		}
		if req.state == stateHeaders {
			if vals, ok := req.Headers["Content-Length"]; ok && len(vals) > 0 {
				// --- Case 1: Content-Length ---
				length, err := strconv.Atoi(vals[0])
				if err != nil {
					return nil, fmt.Errorf("invalid Content-Length: %v", err)
				}

				if len(buff) < length {
					// need more data, continue reading
					continue
				}

				req.Body = buff[:length]
				buff = buff[length:]
				req.state = stateDone

			} else if vals, ok := req.Headers["Transfer-Encoding"]; ok && len(vals) > 0 {
				// --- Case 2: Transfer-Encoding: chunked ---
				if strings.ToLower(vals[0]) == "chunked" {
					out, _, err := decoder.Feed(buff)
					if err != nil {
						return nil, err
					}

					if out == nil {
						// not enough data yet → need more bytes
						continue
					}

					req.Body = out
					req.state = stateDone
				}

			} else {
				// --- Case 3: No body (e.g. GET request) ---
				req.Body = nil
				req.state = stateDone
			}

		}

		if req.state == stateDone {
			return req, nil
		}

		if req.state == stateDone {

			return req, nil

		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("the request object", *req)
	return nil, fmt.Errorf("incomplete request")
}

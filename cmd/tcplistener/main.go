package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"vipul/httpformtcp/internal/request"
)

func getLinesChannel(f io.ReadCloser) <-chan string {
	ch := make(chan string)

	go func() {
		defer f.Close()
		defer close(ch)

		var str []byte

		for {
			data := make([]byte, 8)
			n, err := f.Read(data)
			if err != nil {
				break
			}

			for i := 0; i < n; i++ {
				str = append(str, data[i])

				if data[i] == '\n' {
					if len(str) > 1 && str[0] != '\n' {
						line := string(str)
						ch <- line
					}
					str = str[:0]
				}
			}
		}
	}()

	return ch
}

func main() {
	l, err := net.Listen("tcp", ":42069")
	if err != nil {
		log.Fatal("error creating listener:", err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("error accepting connection:", err)
			continue
		}

		log.Println("accepted connection from", conn.RemoteAddr())

		r, err := request.RequestFromReader(conn)
		if err != nil {
			log.Println("something went wrong with http parser", err)
			return

		}
		// Pretty print
		fmt.Println("\n=== Parsed Request (Chunked) ===")
		fmt.Printf("Method: %s, Path: %s\n", r.RequestLine.Method, r.RequestLine.RequestTarget)
		for k, v := range r.Headers {
			fmt.Printf("  %s: %v\n", k, v)
		}
		fmt.Printf("Body: %q\n", string(r.Body))

		log.Println("connection closed:", conn.RemoteAddr())
	}
}

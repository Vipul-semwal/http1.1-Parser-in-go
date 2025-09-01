package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	// 1) Turn "localhost:42069" into a UDP address struct
	addr, err := net.ResolveUDPAddr("udp", "localhost:42069")
	if err != nil {
		log.Fatal("resolve error:", err)
	}

	// 2) "Dial" a UDP connection (no handshake—just a socket with a default remote addr)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Fatal("dial error:", err)
	}
	defer conn.Close()

	// 3) Reader for stdin (so we can read lines easily)
	reader := bufio.NewReader(os.Stdin)

	for {
		// 4) Prompt the user
		fmt.Print("> ")

		// 5) Read a line (includes the trailing '\n')
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF (Ctrl+D) or read error — just exit the program
			log.Println("stdin read error (exiting):", err)
			return
		}

		// 6) Send the line as a UDP packet
		_, err = conn.Write([]byte(line))
		if err != nil {
			// With UDP this usually doesn't error even if nobody is listening.
			log.Println("udp write error:", err)
		}
	}
}

package alert

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSMTPSend(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- ""
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 1024)
		// Minimal SMTP server dialogue.
		_, _ = conn.Write([]byte("220 localhost\r\n"))
		for {
			n, err := conn.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				s := string(buf)
				switch {
				case strings.Contains(s, "EHLO") || strings.Contains(s, "HELO"):
					_, _ = conn.Write([]byte("250 OK\r\n"))
					buf = buf[:0]
				case strings.HasPrefix(strings.TrimSpace(s), "MAIL"):
					_, _ = conn.Write([]byte("250 OK\r\n"))
					buf = buf[:0]
				case strings.HasPrefix(strings.TrimSpace(s), "RCPT"):
					_, _ = conn.Write([]byte("250 OK\r\n"))
					buf = buf[:0]
				case strings.HasPrefix(strings.TrimSpace(s), "DATA"):
					_, _ = conn.Write([]byte("354 End data\r\n"))
					buf = buf[:0]
					data := make([]byte, 0, 4096)
					for {
						n, err := conn.Read(tmp)
						if n > 0 {
							data = append(data, tmp[:n]...)
							if strings.Contains(string(data), "\r\n.\r\n") {
								_, _ = conn.Write([]byte("250 OK\r\n"))
								done <- string(data)
								_, _ = conn.Write([]byte("221 bye\r\n"))
								return
							}
						}
						if err != nil {
							done <- string(data)
							return
						}
					}
				case strings.HasPrefix(strings.TrimSpace(s), "QUIT"):
					_, _ = conn.Write([]byte("221 bye\r\n"))
					done <- string(buf)
					return
				}
			}
			if err != nil {
				done <- ""
				return
			}
		}
	}()

	n := &Notifier{
		SMTPAddr: ln.Addr().String(),
		SMTPFrom: "fuse@localhost",
		SMTPTo:   "ops@localhost",
	}
	n.SendSMTPForTest("hard budget limit", "agent=demo limit=$2.00")

	select {
	case msg := <-done:
		if !strings.Contains(msg, "agent=demo") {
			t.Fatalf("smtp body missing agent: %q", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for smtp")
	}
}

func TestSMTPNoopWhenUnconfigured(t *testing.T) {
	n := &Notifier{}
	n.SendSMTPForTest("x", "y")
	_, _ = io.Discard.Write(nil)
}

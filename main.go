package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Session struct {
	id     uuid.UUID
	conn   net.Conn
	config *Config
}

type Request struct {
	raw  string
	Host string
}

func NewRequest(
	raw string,
) (*Request, error) {
	lineArr := strings.Split(raw, "\r\n")

	hostHeaderIdx, err := func() (*int, error) {
		for i := 1; i < len(lineArr); i++ {
			if len(lineArr[i]) > 6 && lineArr[i][:6] == "Host: " {
				return &i, nil
			}
		}

		return nil, fmt.Errorf("Host header missing")
	}()
	if err != nil {
		return nil, err
	}

	return &Request{
		raw:  raw,
		Host: lineArr[*hostHeaderIdx][6:],
	}, nil
}

func NewSession(
	conn net.Conn,
	config *Config,
) *Session {
	return &Session{
		id:     uuid.New(),
		conn:   conn,
		config: config,
	}
}

func (s *Session) listen() {
	defer func() {
		s.conn.Close()
	}()
	/*
	 * if your headers are more than 65536 bytes
	 * then you should really rethink what you're doing
	 * */

	size := 2 << 16
	buffer := make([]byte, size)

	i := 0

	CRLFCount := 0

	r := bufio.NewReader(s.conn)
	for {
		b, err := r.ReadByte()
		if err != nil {
			fmt.Printf("error reading: %v\n", err)
			break
		}

		if i > 0 && b == byte('\n') && buffer[i-1] == byte('\r') {
			CRLFCount++
		} else if b != byte('\n') && b != byte('\r') {
			// we found a non-crlf char
			CRLFCount = 0
		}

		// we dont need the body
		if CRLFCount == 2 {
			buffer[i] = '\n'
			i++
			break
		}

		buffer[i] = b

		i++

		if i >= size {
			fmt.Printf("error header was larger than %v", size)
			return
		}
	}

	req, err := NewRequest(string(buffer[:i]))
	if err != nil {
		fmt.Printf("error building request: %v", err)
		return
	}
	s.handleRequest(*req, r)
}

func (s *Session) handleRequest(
	request Request,
	reader io.Reader,
) {
	server, ok := s.config.GetServerForHost(request.Host)
	if !ok {
		fmt.Printf("host not found: %v", request.Host)
		return
	}
	server.NewConnection(NewConnectionRequest(request.raw, &reader, &s.conn))
}

func main() {
	cfg, err := NewConfig()
	if err != nil {
		fmt.Printf("error making config: %v", err)
		return
	}

	go cfg.Start()

	connections := make(map[uuid.UUID]*Session)
	var connectionsLock sync.Mutex

	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("error starting listener: %v\n", err)
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("error accepting connection: %v\n", err)
			continue
		}

		session := NewSession(conn, cfg)
		go func() {
			connectionsLock.Lock()
			connections[session.id] = session
			connectionsLock.Unlock()
			defer func() {
				connectionsLock.Lock()
				delete(connections, session.id)
				connectionsLock.Unlock()
			}()
			session.listen()
		}()
	}
}

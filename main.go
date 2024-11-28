package main

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

type Server struct {
	connections map[uuid.UUID]*Session
}

func NewServer() *Server {
	return &Server{
		connections: make(map[uuid.UUID]*Session),
	}
}

type Session struct {
	id   uuid.UUID
	conn net.Conn
}

func NewSession(conn net.Conn) *Session {
	return &Session{
		id:   uuid.New(),
		conn: conn,
	}
}

func (s *Session) listen() {
	defer func() {
		s.conn.Close()
	}()

	time.Sleep(3 * time.Second)
}

func (s *Server) Start() {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("error starting listener: %v", err)
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("error accepting connection: %v", err)
			continue
		}

		session := NewSession(conn)
		go session.listen()
	}
}

func main() {
	server := NewServer()
	server.Start()
}

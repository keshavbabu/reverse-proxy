package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
)

type Server struct {
	connections     map[uuid.UUID]*Session
	connectionsLock sync.Mutex
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

	buffer := make([]byte, 1024)

	r := bufio.NewReader(s.conn)
	n, err := r.Read(buffer)
	if err != nil {
		fmt.Printf("error reading: %v\n", err)
		return
	}

	m := binary.LittleEndian.Uint16(buffer[:2])

	fmt.Printf("[%v] read %v bytes: %v\n", m, n, string(buffer[:n]))
}

func (s *Server) Start() {
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

		session := NewSession(conn)
		go func() {
			s.connectionsLock.Lock()
			s.connections[session.id] = session
			s.connectionsLock.Unlock()
			defer func() {
				s.connectionsLock.Lock()
				delete(s.connections, session.id)
				s.connectionsLock.Unlock()
			}()
			session.listen()
		}()
	}
}

func main() {
	server := NewServer()
	server.Start()
}

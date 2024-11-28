package main

import (
	"bufio"
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
	/*
	 * if you're sending a packet with more than 65536 bytes
	 * then you should reallt rethink what you're doing
	 * */
	buffer := make([]byte, 2<<16)

	r := bufio.NewReader(s.conn)
	tmp := make([]byte, 2<<12)
	for {
		n, err := r.Read(tmp)
		if err != nil {
			fmt.Printf("error reading: %v\n", err)
			break
		}

		buffer = append(buffer, tmp[:n]...)

		// if we ever read anything less than the max break
		if n < 2<<12 {
			break
		}
	}

	st := string(buffer)
	fmt.Printf("last: %v\n", len(st))
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

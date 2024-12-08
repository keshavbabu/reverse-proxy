package main

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type ConnectionRequest struct {
	Request    string
	Reader     *io.Reader
	Connection *net.Conn
}

func NewConnectionRequest(
	request string,
	reader *io.Reader,
	connection *net.Conn,
) ConnectionRequest {
	return ConnectionRequest{
		Request:    request,
		Reader:     reader,
		Connection: connection,
	}
}

type Server struct {
	Host    string
	Address string
	Channel chan ConnectionRequest
}

func NewServer(
	host string,
	address string,
) *Server {
	return &Server{
		Host:    host,
		Address: address,
		Channel: make(chan ConnectionRequest),
	}
}

func (s *Server) NewConnection(req ConnectionRequest) {
	conn, err := net.Dial("tcp", s.Address)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer conn.Close()
	conn.Write([]byte(req.Request))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		_, err := io.Copy(conn, *req.Reader)
		if err != nil {
			fmt.Printf("request copy error: %v\n", err)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		_, err := io.Copy(*req.Connection, conn)
		if err != nil {
			fmt.Printf("response copy error: %v\n", err)
		}
		wg.Done()
	}()
	wg.Wait()
}

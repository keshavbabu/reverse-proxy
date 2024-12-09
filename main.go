package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

type Session struct {
	id      uuid.UUID
	conn    net.Conn
	servers map[string]*Server
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
	servers map[string]*Server,
) *Session {
	return &Session{
		id:      uuid.New(),
		conn:    conn,
		servers: servers,
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
	server, ok := s.servers[request.Host]
	if !ok {
		fmt.Printf("host not found: %v", request.Host)
		return
	}
	server.NewConnection(NewConnectionRequest(request.raw, &reader, &s.conn))
}

type ServerConfig struct {
	Host          string `toml:"host"`
	DownstreamURL string `toml:"downstream-url"`
}

type Config struct {
	Servers map[string]ServerConfig `toml:"servers"`
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func ReadConfig(configfp string) (*map[string]*Server, error) {
	d, err := os.ReadFile(configfp)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}
	cfg := Config{}
	_, err = toml.Decode(string(d), &cfg)
	if err != nil {
		fmt.Println("error decoding toml:", err)
		return nil, fmt.Errorf("error decoding toml: %v", err)
	}
	newServers := make(map[string]*Server)
	for _, server := range cfg.Servers {
		newServers[server.Host] = NewServer(server.Host, server.DownstreamURL)
	}
	return &newServers, nil
}

func main() {
	home, ok := os.LookupEnv("HOME")
	if !ok {
		fmt.Println("env var HOME not set")
		return
	}

	configfp := fmt.Sprintf("%s/.config/reverse-proxy", home)
	err := os.MkdirAll(configfp, os.ModePerm)
	if err != nil {
		fmt.Printf("error making config dir: %v", err)
		return
	}

	servers := make(map[string]*Server)
	ex, err := exists(configfp + "/config.toml")
	if err == nil && ex {
		s, err := ReadConfig(configfp + "/config.toml")
		if err == nil {
			servers = *s
		} else {
			fmt.Println("error reading config:", err)
		}
	}

	go func() {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			fmt.Println("error making fs watcher", err)
			return
		}
		defer w.Close()

		err = w.Add(configfp)
		if err != nil {
			fmt.Println("error adding config path:", err)
			return
		}

		for {
			select {
			case e, ok := <-w.Events:
				if !ok {
					break
				}

				file := e.Name[len(configfp):]
				if file == "/config.toml" && e.Op.Has(fsnotify.Chmod) {
					// run the reload here
					fmt.Println("reloading config.toml")
					s, err := ReadConfig(e.Name)
					if err != nil {
						fmt.Println("error reading config:", err)
						continue
					}
					servers = *s
				}
			case err, ok := <-w.Errors:
				if !ok {
					break
				}
				fmt.Println("[fs-error]", err)
			}
		}
	}()

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

		session := NewSession(conn, servers)
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

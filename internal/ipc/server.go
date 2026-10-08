package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// Handler interface implemented by an oml instance (TUI or MCP) to answer IPC requests.
type Handler interface {
	GetStatus() map[string]interface{}
	GetRecentLogs(count int, minLevel, module string) []record.Record
	SearchLogs(query string, isRegex bool, maxResults int) []record.Record
	SendSerialCommand(command string, waitMS int, ending string) (int, []record.Record, error)
	HandoverPort() error
}

// Server hosts an IPC endpoint servicing incoming client queries.
type Server struct {
	listener net.Listener
	handler  Handler
	stopCh   chan struct{}
	wg       sync.WaitGroup
	addr     string
}

// NewServer starts an IPC server listening on addr with the provided request handler.
func NewServer(addr string, handler Handler) (*Server, error) {
	ln, err := Listen(addr)
	if err != nil {
		return nil, err
	}

	s := &Server{
		listener: ln,
		handler:  handler,
		stopCh:   make(chan struct{}),
		addr:     addr,
	}

	s.wg.Add(1)
	go s.acceptLoop()

	return s, nil
}

// Close stops the IPC server and removes the socket file.
func (s *Server) Close() error {
	close(s.stopCh)
	err := s.listener.Close()
	s.wg.Wait()
	return err
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = encoder.Encode(Response{Success: false, Error: "invalid request payload"})
			return
		}

		resp := s.dispatch(&req)
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}

func (s *Server) dispatch(req *Request) Response {
	switch req.Action {
	case "ping":
		data, _ := json.Marshal("pong")
		return Response{Success: true, Data: data}

	case "status":
		stat := s.handler.GetStatus()
		data, _ := json.Marshal(stat)
		return Response{Success: true, Data: data}

	case "logs":
		var p struct {
			Count    int    `json:"count"`
			MinLevel string `json:"min_level"`
			Module   string `json:"module"`
		}
		if len(req.Data) > 0 {
			_ = json.Unmarshal(req.Data, &p)
		}
		logs := s.handler.GetRecentLogs(p.Count, p.MinLevel, p.Module)
		data, _ := json.Marshal(logs)
		return Response{Success: true, Data: data}

	case "search":
		var p struct {
			Query      string `json:"query"`
			IsRegex    bool   `json:"is_regex"`
			MaxResults int    `json:"max_results"`
		}
		if len(req.Data) > 0 {
			_ = json.Unmarshal(req.Data, &p)
		}
		matches := s.handler.SearchLogs(p.Query, p.IsRegex, p.MaxResults)
		data, _ := json.Marshal(matches)
		return Response{Success: true, Data: data}

	case "send_cmd":
		var p struct {
			Command string `json:"command"`
			WaitMS  int    `json:"wait_ms"`
			Ending  string `json:"ending"`
		}
		if len(req.Data) > 0 {
			_ = json.Unmarshal(req.Data, &p)
		}
		n, resLogs, err := s.handler.SendSerialCommand(p.Command, p.WaitMS, p.Ending)
		if err != nil {
			return Response{Success: false, Error: err.Error()}
		}
		data, _ := json.Marshal(map[string]interface{}{
			"bytes":     n,
			"responses": resLogs,
		})
		return Response{Success: true, Data: data}

	case "handover":
		err := s.handler.HandoverPort()
		if err != nil {
			return Response{Success: false, Error: err.Error()}
		}
		data, _ := json.Marshal("handover acknowledged")
		return Response{Success: true, Data: data}

	default:
		return Response{Success: false, Error: fmt.Sprintf("unknown action %q", req.Action)}
	}
}

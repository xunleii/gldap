// Copyright (c) Jim Lambert
// SPDX-License-Identifier: MIT

package gldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// Server is an ldap server that you can add a mux (multiplexer) router to and
// then run it to accept and process requests.
type Server struct {
	mu             sync.RWMutex
	logger         hclog.Logger
	connWg         sync.WaitGroup
	listener       net.Listener
	listenerReady  bool
	router         *Mux
	tlsConfig      *tls.Config
	readTimeout    time.Duration
	writeTimeout   time.Duration
	onCloseHandler OnCloseHandler

	disablePanicRecovery bool
	shutdownCancel       context.CancelFunc
	shutdownCtx          context.Context
}

// NewServer creates a new ldap server
//
// Options supported:
// - WithLogger allows you pass a logger with whatever hclog.Level you wish including hclog.Off to turn off all logging
// - WithReadTimeout will set a read time out per connection
// - WithWriteTimeout will set a write time out per connection
// - WithOnClose will define a callback the server will call every time a connection is closed
func NewServer(opt ...Option) (*Server, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	opts := getConfigOpts(opt...)

	if opts.withLogger == nil {
		opts.withLogger = hclog.New(&hclog.LoggerOptions{
			Name:  "Server-logger",
			Level: hclog.Error,
		})
	}

	return &Server{
		router:               &Mux{}, // TODO: a better default router
		logger:               opts.withLogger,
		shutdownCancel:       cancel,
		shutdownCtx:          cancelCtx,
		writeTimeout:         opts.withWriteTimeout,
		readTimeout:          opts.withReadTimeout,
		disablePanicRecovery: opts.withDisablePanicRecovery,
		onCloseHandler:       opts.withOnClose,
	}, nil
}

// Run will run the server which will listen and serve requests.
//
// Options supported: WithTLSConfig
func (s *Server) Run(addr string, opt ...Option) error {
	const op = "gldap.(Server).Run"
	opts := getConfigOpts(opt...)

	var err error
	s.mu.Lock()
	s.listener, err = net.Listen("tcp", addr)
	s.listenerReady = true
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("%s: unable to listen to addr %s: %w", op, addr, err)
	}
	if opts.withTLSConfig != nil {
		s.logger.Debug("setting up TLS listener", "op", op)
		s.tlsConfig = opts.withTLSConfig
		s.mu.Lock()
		s.listener = tls.NewListener(s.listener, s.tlsConfig)
		s.mu.Unlock()
	}
	s.logger.Info("listening", "op", op, "addr", s.listener.Addr())

	connID := 0
	for {
		connID++
		select {
		case <-s.shutdownCtx.Done():
			return nil
		default:
			// need a default to fall through to rest of loop...
		}
		c, err := s.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				s.logger.Debug("accept on closed conn")
				return nil
			}
			return fmt.Errorf("%s: error accepting conn: %w", op, err)
		}
		s.logger.Debug("new connection accepted", "op", op, "conn", connID)
		conn, err := newConn(s.shutdownCtx, connID, c, s.logger, s.router)
		if err != nil {
			return fmt.Errorf("%s: unable to create in-memory conn: %w", op, err)
		}
		localConnID := connID
		s.connWg.Add(1)
		go func() {
			defer func() {
				s.logger.Debug("connWg done", "op", op, "conn", localConnID)
				s.connWg.Done()
				err := conn.close()
				if err != nil {
					s.logger.Error("error closing conn", "op", op, "conn", localConnID, "conn/req", "err", err)
					// we are intentionally not returning here; since we still
					// need to call the onCloseHandler if it's not nil
				}
				if s.onCloseHandler != nil {
					s.onCloseHandler(localConnID)
				}
			}()

			if !s.disablePanicRecovery {
				// catch and report panics - we don't want it to crash the server if
				// handling a single conn causes a panic
				defer func() {
					if r := recover(); r != nil {
						s.logger.Error("Caught panic while serving request", "op", op, "conn", localConnID, "conn/req", fmt.Sprintf("%+v: %+v", c, r))
					}
				}()
			}
			if s.readTimeout != 0 {
				if err := c.SetReadDeadline(time.Now().Add(s.readTimeout)); err != nil {
					s.logger.Error("unable to set read deadline", "op", op, "err", err.Error())
					return
				}
			}
			if s.writeTimeout != 0 {
				if err := c.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil {
					s.logger.Error("unable to set write deadline", "op", op, "err", err.Error())
					return
				}
			}
			if err := conn.serveRequests(); err != nil {
				s.logger.Error("error handling conn", "op", op, "conn", localConnID, "err", err.Error())
			}
		}()
	}
}

// Ready will return true when the server is ready to accept connection
func (s *Server) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listenerReady
}

// Stop a running ldap server
func (s *Server) Stop() error {
	const op = "gldap.(Server).Stop"
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.logger.Debug("shutting down")
	if s.listener == nil && s.shutdownCancel == nil {
		s.logger.Debug("nothing to do for shutdown")
		return nil
	}

	if s.listener != nil {
		s.logger.Debug("closing listener")
		if err := s.listener.Close(); err != nil {
			switch {
			case !strings.Contains(err.Error(), "use of closed network connection"):
				return fmt.Errorf("%s: %w", op, err)
			default:
				s.logger.Debug("listener already closed")
			}
		}
	}
	if s.shutdownCancel != nil {
		s.logger.Debug("shutdown cancel func")
		s.shutdownCancel()
	}
	s.logger.Debug("waiting on connections to close")
	s.connWg.Wait()
	s.logger.Debug("stopped")
	return nil
}

// Router sets the mux (multiplexer) router for matching inbound requests
// to handlers.
func (s *Server) Router(r *Mux) error {
	const op = "gldap.(Server).HandleRoutes"
	if r == nil {
		return fmt.Errorf("%s: missing router: %w", op, ErrInvalidParameter)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.router = r
	return nil
}

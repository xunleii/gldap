package gldap

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_Stop(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		server          *Server
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "missing-listener",
			server: func() *Server {
				s, err := NewServer()
				require.NoError(t, err)
				s.mu.Lock()
				defer s.mu.Unlock()
				s.listener = nil
				return s
			}(),
			wantErr:         true,
			wantErrContains: "no listener",
		},
		{
			name: "missing-cancel",
			server: func() *Server {
				p := freePort(t)
				l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
				require.NoError(t, err)
				s, err := NewServer()
				require.NoError(t, err)
				s.mu.Lock()
				defer s.mu.Unlock()
				s.listener = l
				s.shutdownCancel = nil
				return s
			}(),
			wantErr:         true,
			wantErrContains: "no shutdown context cancel func",
		},
		{
			name: "listener-closed",
			server: func() *Server {
				_, cancel := context.WithCancel(context.Background())
				p := freePort(t)
				l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
				require.NoError(t, err)
				s, err := NewServer()
				require.NoError(t, err)
				s.mu.Lock()
				defer s.mu.Unlock()
				s.listener = l
				s.shutdownCancel = cancel
				l.Close()
				return s
			}(),
			wantErr:         true,
			wantErrContains: "use of closed network connection",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert, require := assert.New(t), require.New(t)
			err := tc.server.Stop()
			if tc.wantErr {
				require.Error(err)
				if tc.wantErrContains != "" {
					assert.Contains(err.Error(), tc.wantErrContains)
				}
				return
			}
			require.NoError(err)
		})
	}
}

package main

import (
	"testing"
	"time"

	"payment-sandbox/app/config"
	sagaSvc "payment-sandbox/app/modules/saga/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Branch analysis for newApp():
// └── single code path: assigns cfg, router, orchestrator to App fields → returns *App

func TestNewApp(t *testing.T) {
	// not parallel: gin.SetMode writes to a global
	gin.SetMode(gin.TestMode)

	type args struct {
		cfg          config.Config
		router       *gin.Engine
		orchestrator *sagaSvc.Orchestrator
	}
	type wants struct {
		cfg          config.Config
		router       *gin.Engine
		orchestrator *sagaSvc.Orchestrator
		notNil       bool
	}

	router := gin.New()
	orchestrator := &sagaSvc.Orchestrator{}
	cfg := config.Config{
		AppPort:     "8080",
		JWTSecret:   "secret",
		ShutdownTTL: 5 * time.Second,
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. valid cfg, router, orchestrator -> App fields assigned correctly",
			args: args{
				cfg:          cfg,
				router:       router,
				orchestrator: orchestrator,
			},
			wants: wants{
				cfg:          cfg,
				router:       router,
				orchestrator: orchestrator,
				notNil:       true,
			},
		},
		{
			name: "2. nil router and nil orchestrator -> App created with nil fields",
			args: args{
				cfg:          config.Config{AppPort: "9090"},
				router:       nil,
				orchestrator: nil,
			},
			wants: wants{
				cfg:          config.Config{AppPort: "9090"},
				router:       nil,
				orchestrator: nil,
				notNil:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: gin global state

			got := newApp(tt.args.cfg, tt.args.router, tt.args.orchestrator)

			assert.NotNil(t, got, "app must not be nil")
			assert.Equal(t, tt.wants.cfg, got.Config, "config")
			assert.Equal(t, tt.wants.router, got.Router, "router")
			assert.Equal(t, tt.wants.orchestrator, got.Orchestrator, "orchestrator")
		})
	}
}

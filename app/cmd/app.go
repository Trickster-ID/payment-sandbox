package main

import (
	"payment-sandbox/app/config"
	sagaSvc "payment-sandbox/app/modules/saga/services"

	"github.com/gin-gonic/gin"
)

type App struct {
	Config       config.Config
	Router       *gin.Engine
	Orchestrator *sagaSvc.Orchestrator
}

func newApp(cfg config.Config, router *gin.Engine, orchestrator *sagaSvc.Orchestrator) *App {
	return &App{
		Config:       cfg,
		Router:       router,
		Orchestrator: orchestrator,
	}
}

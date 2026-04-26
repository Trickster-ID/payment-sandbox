package main

import (
	"payment-sandbox/app/config"

	"github.com/gin-gonic/gin"
)

type App struct {
	Config config.Config
	Router *gin.Engine
}

func newApp(cfg config.Config, router *gin.Engine) *App {
	return &App{
		Config: cfg,
		Router: router,
	}
}

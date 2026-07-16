package api

import (
	"payment-sandbox/app/modules/oauth2/handlers"
	"github.com/gin-gonic/gin"
)

func RegisterPublicRoutes(router *gin.RouterGroup, handler *handlers.OAuth2Handler) {
	oauth2 := router.Group("/oauth2")
	{
		oauth2.POST("/token", handler.Token)
		oauth2.POST("/introspect", handler.Introspect)
		oauth2.POST("/revoke", handler.Revoke)

		oauth2.GET("/authorize", handler.Authorize)
	}
}

// RegisterSecuredRoutes registers routes requiring an authenticated user
// (Bearer access token via AuthMiddleware).
func RegisterSecuredRoutes(router *gin.RouterGroup, handler *handlers.OAuth2Handler) {
	oauth2 := router.Group("/oauth2")
	{
		oauth2.GET("/userinfo", handler.UserInfo)
		// POST /authorize (ApproveAuthorize) requires middleware.MustUserID,
		// so it must live under secured routes, not public.
		oauth2.POST("/authorize", handler.ApproveAuthorize)
	}
}

func RegisterMerchantRoutes(router *gin.RouterGroup, handler *handlers.OAuth2Handler) {
	clients := router.Group("/clients")
	{
		clients.POST("", handler.RegisterClient)
		clients.GET("", handler.ListClients)
		clients.DELETE("/:id", handler.DeleteClient)
	}
}

package main

import (
	"time"

	"payment-sandbox/app/config"
	"payment-sandbox/app/middleware"
	adminAPI "payment-sandbox/app/modules/admin/api"
	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	usersAPI "payment-sandbox/app/modules/users/api"
	usersHandlers "payment-sandbox/app/modules/users/handlers"
	"payment-sandbox/app/modules/users/models/entity"
	invoiceAPI "payment-sandbox/app/modules/invoice/api"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	oauth2API "payment-sandbox/app/modules/oauth2/api"
	oauth2Handlers "payment-sandbox/app/modules/oauth2/handlers"
	paymentAPI "payment-sandbox/app/modules/payment/api"
	paymentHandlers "payment-sandbox/app/modules/payment/handlers"
	refundAPI "payment-sandbox/app/modules/refund/api"
	refundHandlers "payment-sandbox/app/modules/refund/handlers"
	walletAPI "payment-sandbox/app/modules/wallet/api"
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"
	"payment-sandbox/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func newRouter(
	cfg config.Config,
	usersHandler *usersHandlers.UserHandler,
	adminHandler *adminHandlers.AdminHandler,
	walletHandler *walletHandlers.WalletHandler,
	invoiceHandler *invoiceHandlers.InvoiceHandler,
	paymentHandler *paymentHandlers.PaymentHandler,
	refundHandler *refundHandlers.RefundHandler,
	oauth2Handler *oauth2Handlers.OAuth2Handler,
) *gin.Engine {
	docs.SwaggerInfo.Host = "localhost:" + cfg.AppPort
	docs.SwaggerInfo.BasePath = "/api/v1"
	docs.SwaggerInfo.Schemes = []string{"http"}

	router := gin.New()
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(gin.Logger())
	router.Use(gin.Recovery(), gin.LoggerWithFormatter(func(params gin.LogFormatterParams) string {
		return params.TimeStamp.Format(time.RFC3339) + " " + params.Method + " " + params.Path + " " + params.ClientIP + " " + params.ErrorMessage + "\n"
	}))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := router.Group("/api/v1")
	{
		v1.GET("/ping", adminHandler.Healthz)
		usersAPI.RegisterPublicRoutes(v1, usersHandler)
		paymentAPI.RegisterPublicRoutes(v1, paymentHandler)
		oauth2API.RegisterPublicRoutes(v1, oauth2Handler)
	}

	secured := v1.Group("")
	secured.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		oauth2API.RegisterSecuredRoutes(secured, oauth2Handler)

		merchant := secured.Group("/merchant")
		merchant.Use(middleware.RequireRoles(entity.RoleMerchant))
		{
			walletAPI.RegisterMerchantRoutes(merchant, walletHandler)
			invoiceAPI.RegisterMerchantRoutes(merchant, invoiceHandler)
			refundAPI.RegisterMerchantRoutes(merchant, refundHandler)
			oauth2API.RegisterMerchantRoutes(merchant, oauth2Handler)
		}

		admin := secured.Group("/admin")
		admin.Use(middleware.RequireRoles(entity.RoleAdmin))
		{
			walletAPI.RegisterAdminRoutes(admin, walletHandler)
			paymentAPI.RegisterAdminRoutes(admin, paymentHandler)
			refundAPI.RegisterAdminRoutes(admin, refundHandler)
			adminAPI.RegisterAdminRoutes(admin, adminHandler)
		}
	}

	return router
}

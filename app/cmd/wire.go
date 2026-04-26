//go:build wireinject
// +build wireinject

package main

import (
	"payment-sandbox/app/config"
	"payment-sandbox/app/middleware"
	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	adminRepo "payment-sandbox/app/modules/admin/repositories"
	adminSvc "payment-sandbox/app/modules/admin/services"
	authHandlers "payment-sandbox/app/modules/auth/handlers"
	authRepo "payment-sandbox/app/modules/auth/repositories"
	authSvc "payment-sandbox/app/modules/auth/services"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	invoiceRepo "payment-sandbox/app/modules/invoice/repositories"
	invoiceSvc "payment-sandbox/app/modules/invoice/services"
	paymentHandlers "payment-sandbox/app/modules/payment/handlers"
	paymentRepo "payment-sandbox/app/modules/payment/repositories"
	paymentSvc "payment-sandbox/app/modules/payment/services"
	refundHandlers "payment-sandbox/app/modules/refund/handlers"
	refundRepo "payment-sandbox/app/modules/refund/repositories"
	refundSvc "payment-sandbox/app/modules/refund/services"
	walletHandlers "payment-sandbox/app/modules/wallet/handlers"
	walletRepo "payment-sandbox/app/modules/wallet/repositories"
	walletSvc "payment-sandbox/app/modules/wallet/services"
	"payment-sandbox/app/shared/database"

	"github.com/google/wire"
)

func initApp() (*App, error) {
	wire.Build(
		config.Load,
		database.New,
		middleware.NewJWTService,
		provideAuthRepository,
		provideJourneyLogger,
		wire.Bind(new(authRepo.AuthRepository), new(*authRepo.SQLAuthRepository)),
		adminRepo.NewAdminRepository,
		wire.Bind(new(adminRepo.AdminRepository), new(*adminRepo.SQLAdminRepository)),
		walletRepo.NewWalletRepository,
		wire.Bind(new(walletRepo.WalletRepository), new(*walletRepo.SQLWalletRepository)),
		invoiceRepo.NewInvoiceRepository,
		wire.Bind(new(invoiceRepo.InvoiceRepository), new(*invoiceRepo.SQLInvoiceRepository)),
		paymentRepo.NewPaymentRepository,
		wire.Bind(new(paymentRepo.PaymentRepository), new(*paymentRepo.SQLPaymentRepository)),
		refundRepo.NewRefundRepository,
		wire.Bind(new(refundRepo.RefundRepository), new(*refundRepo.SQLRefundRepository)),
		authSvc.NewAuthService,
		authHandlers.NewAuthHandler,
		adminSvc.NewAdminService,
		adminHandlers.NewAdminHandler,
		walletSvc.NewWalletService,
		walletHandlers.NewWalletHandler,
		invoiceSvc.NewInvoiceService,
		invoiceHandlers.NewInvoiceHandler,
		paymentSvc.NewPaymentService,
		paymentHandlers.NewPaymentHandler,
		refundSvc.NewRefundService,
		refundHandlers.NewRefundHandler,
		newRouter,
		newApp,
	)
	return &App{}, nil
}

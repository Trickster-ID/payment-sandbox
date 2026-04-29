//go:build wireinject
// +build wireinject

package main

import (
	"payment-sandbox/app/config"
	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	adminRepo "payment-sandbox/app/modules/admin/repositories"
	adminSvc "payment-sandbox/app/modules/admin/services"
	usersHandlers "payment-sandbox/app/modules/users/handlers"
	usersRepo "payment-sandbox/app/modules/users/repositories"
	usersSvc "payment-sandbox/app/modules/users/services"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	invoiceRepo "payment-sandbox/app/modules/invoice/repositories"
	invoiceSvc "payment-sandbox/app/modules/invoice/services"
	oauth2Handlers "payment-sandbox/app/modules/oauth2/handlers"
	oauth2Repo "payment-sandbox/app/modules/oauth2/repositories"
	oauth2Svc "payment-sandbox/app/modules/oauth2/services"
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
		provideUserRepository,
		provideJourneyLogger,
		wire.Bind(new(usersRepo.IUserRepository), new(*usersRepo.UserRepository)),
		adminRepo.NewAdminRepository,
		wire.Bind(new(adminRepo.IAdminRepository), new(*adminRepo.AdminRepository)),
		walletRepo.NewWalletRepository,
		wire.Bind(new(walletRepo.IWalletRepository), new(*walletRepo.WalletRepository)),
		invoiceRepo.NewInvoiceRepository,
		wire.Bind(new(invoiceRepo.IInvoiceRepository), new(*invoiceRepo.InvoiceRepository)),
		paymentRepo.NewPaymentRepository,
		wire.Bind(new(paymentRepo.IPaymentRepository), new(*paymentRepo.PaymentRepository)),
		refundRepo.NewRefundRepository,
		wire.Bind(new(refundRepo.IRefundRepository), new(*refundRepo.RefundRepository)),
		oauth2Repo.NewOAuth2Repository,
		wire.Bind(new(oauth2Repo.IOAuth2Repository), new(*oauth2Repo.OAuth2Repository)),
		usersSvc.NewUserService,
		wire.Bind(new(usersSvc.IUserService), new(*usersSvc.UserService)),
		usersHandlers.NewUserHandler,
		adminSvc.NewAdminService,
		wire.Bind(new(adminSvc.IAdminService), new(*adminSvc.AdminService)),
		adminHandlers.NewAdminHandler,
		walletSvc.NewWalletService,
		wire.Bind(new(walletSvc.IWalletService), new(*walletSvc.WalletService)),
		walletHandlers.NewWalletHandler,
		invoiceSvc.NewInvoiceService,
		wire.Bind(new(invoiceSvc.IInvoiceService), new(*invoiceSvc.InvoiceService)),
		invoiceHandlers.NewInvoiceHandler,
		paymentSvc.NewPaymentService,
		wire.Bind(new(paymentSvc.IPaymentService), new(*paymentSvc.PaymentService)),
		paymentHandlers.NewPaymentHandler,
		refundSvc.NewRefundService,
		wire.Bind(new(refundSvc.IRefundService), new(*refundSvc.RefundService)),
		refundHandlers.NewRefundHandler,
		oauth2Svc.NewOAuth2Service,
		wire.Bind(new(oauth2Svc.IOAuth2Service), new(*oauth2Svc.OAuth2Service)),
		oauth2Handlers.NewOAuth2Handler,
		newRouter,
		newApp,
	)
	return &App{}, nil
}

//go:build wireinject
// +build wireinject

package main

import (
	"payment-sandbox/app/config"
	adminHandlers "payment-sandbox/app/modules/admin/handlers"
	adminRepo "payment-sandbox/app/modules/admin/repositories"
	adminSvc "payment-sandbox/app/modules/admin/services"
	merchantHandlers "payment-sandbox/app/modules/merchants/handlers"
	merchantRepo "payment-sandbox/app/modules/merchants/repositories"
	merchantSvc "payment-sandbox/app/modules/merchants/services"
	usersHandlers "payment-sandbox/app/modules/users/handlers"
	usersRepo "payment-sandbox/app/modules/users/repositories"
	usersSvc "payment-sandbox/app/modules/users/services"
	invoiceHandlers "payment-sandbox/app/modules/invoice/handlers"
	invoiceRepo "payment-sandbox/app/modules/invoice/repositories"
	invoiceSvc "payment-sandbox/app/modules/invoice/services"
	ledgerHandlers "payment-sandbox/app/modules/ledger/handlers"
	ledgerRepo "payment-sandbox/app/modules/ledger/repositories"
	oauth2Handlers "payment-sandbox/app/modules/oauth2/handlers"
	sagaSvc "payment-sandbox/app/modules/saga/services"
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
		database.NewMongoDB,
		database.NewRedis,
		provideAuditLogger,
		provideIdempotencyMiddleware,
		provideUserRepository,
		wire.Bind(new(usersRepo.IUserRepository), new(*usersRepo.UserRepository)),
		adminRepo.NewAdminRepository,
		wire.Bind(new(adminRepo.IAdminRepository), new(*adminRepo.AdminRepository)),
		merchantRepo.NewMerchantsRepository,
		wire.Bind(new(merchantRepo.IMerchantsRepository), new(*merchantRepo.MerchantsRepository)),
		ledgerRepo.NewRepository,
		wire.Bind(new(ledgerRepo.IRepository), new(*ledgerRepo.Repository)),
		walletRepo.NewWalletRepository,
		wire.Bind(new(walletRepo.IWalletRepository), new(*walletRepo.WalletRepository)),
		invoiceRepo.NewInvoiceRepository,
		wire.Bind(new(invoiceRepo.IInvoiceRepository), new(*invoiceRepo.InvoiceRepository)),
		sagaSvc.NewOrchestrator,
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
		merchantSvc.NewMerchantsService,
		wire.Bind(new(merchantSvc.IMerchantsService), new(*merchantSvc.MerchantsService)),
		merchantHandlers.NewMerchantsHandler,
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
		ledgerHandlers.NewLedgerHandler,
		newRouter,
		newApp,
	)
	return &App{}, nil
}

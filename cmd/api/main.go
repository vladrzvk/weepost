package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"github.com/vladrzvk/weepost/internal/application"
	"github.com/vladrzvk/weepost/internal/http/handlers"
	"github.com/vladrzvk/weepost/internal/http/middleware"
	"github.com/vladrzvk/weepost/internal/infrastructure/config"
	"github.com/vladrzvk/weepost/internal/infrastructure/postgres"
	"github.com/vladrzvk/weepost/internal/infrastructure/stub"
	brandUC    "github.com/vladrzvk/weepost/internal/usecases/brand"
	channelUC  "github.com/vladrzvk/weepost/internal/usecases/channel"
	mediaUC    "github.com/vladrzvk/weepost/internal/usecases/media"
	securityUC "github.com/vladrzvk/weepost/internal/usecases/security"
	workspaceUC "github.com/vladrzvk/weepost/internal/usecases/workspace"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// ── Postgres ──────────────────────────────────────────────────────────────
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(context.Background(), pool, "migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	// ── Services infra ────────────────────────────────────────────────────────
	cryptoSvc, err := stub.NewCryptoService(cfg.CryptoKey)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}

	jwtSvc := stub.NewJWTService(
		cfg.JWTSecret,
		cfg.JWTRefreshSecret,
		cfg.JWTAccessExpiry,
		cfg.JWTRefreshExpiry,
	)

	emailSvc := stub.NewEmailService()
	eventBus := stub.NewNoOpEventBus()

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo        := postgres.NewPostgresUserRepo(pool)
	workspaceRepo   := postgres.NewPostgresWorkspaceRepo(pool)
	brandRepo       := postgres.NewPostgresBrandRepo(pool)
	postRepo        := postgres.NewPostgresPostRepo(pool)
	channelRepo     := postgres.NewPostgresChannelRepo(pool)
	sessionRepo     := postgres.NewPostgresSessionRepo(pool)
	invitationRepo  := postgres.NewPostgresInvitationRepo(pool)
	auditRepo       := postgres.NewPostgresAuditRepo(pool)
	approvalRepo    := postgres.NewPostgresApprovalRequestRepo(pool)
	mediaRepo       := postgres.NewPostgresMediaAssetRepo(pool)
	pwdResetRepo    := postgres.NewPostgresPasswordResetTokenRepo(pool)
	keyRotRepo      := postgres.NewPostgresKeyRotationRepo(pool)

	_ = auditRepo; _ = approvalRepo; _ = pwdResetRepo; _ = keyRotRepo

	// ── Application services ──────────────────────────────────────────────────
	permChecker := application.NewPermissionChecker(workspaceRepo, brandRepo)

	// ── Plan checkers & channel cred repo (no-op local) ───────────────────────
	wsPlanChecker      := stub.NoOpWorkspacePlanChecker{}
	brandPlanChecker   := stub.NoOpBrandPlanChecker{}
	channelPlanChecker := stub.NoOpChannelPlanChecker{}
	channelCredRepo    := stub.NoOpChannelCredentialRepo{}
	planFeatureSvc     := stub.NoOpPlanFeatureService{}

	// ── Security use cases ────────────────────────────────────────────────────
	recordFailedLoginSvc  := securityUC.NewRecordFailedLoginService(userRepo, sessionRepo, eventBus)
	generateJWTSvc        := securityUC.NewGenerateJWTTokenService(sessionRepo, jwtSvc)
	validateCredentialsSvc := securityUC.NewValidateCredentialsService(
		userRepo, sessionRepo, recordFailedLoginSvc, generateJWTSvc, jwtSvc, eventBus)
	validate2FASvc  := securityUC.NewValidate2FAService(userRepo, sessionRepo, generateJWTSvc, jwtSvc, planFeatureSvc)
	revokeSessionSvc := securityUC.NewRevokeJWTTokenService(sessionRepo)

	_ = emailSvc

	// ── Workspace use cases ───────────────────────────────────────────────────
	createWorkspaceUC  := workspaceUC.NewCreateWorkspaceUseCase(workspaceRepo, eventBus)
	updateWorkspaceUC  := workspaceUC.NewUpdateWorkspaceUseCase(workspaceRepo, eventBus)
	deleteWorkspaceUC  := workspaceUC.NewDeleteWorkspaceUseCase(workspaceRepo, brandRepo, sessionRepo, eventBus)
	inviteMemberUC     := workspaceUC.NewInviteMemberUseCase(workspaceRepo, invitationRepo, userRepo, eventBus, wsPlanChecker)
	_ = workspaceUC.NewAcceptInvitationUseCase(workspaceRepo, invitationRepo, userRepo, eventBus, wsPlanChecker) // stub used instead

	// ── Brand use cases ───────────────────────────────────────────────────────
	createBrandUC       := brandUC.NewCreateBrandUseCase(workspaceRepo, brandRepo, userRepo, eventBus, brandPlanChecker)
	updateBrandUC       := brandUC.NewUpdateBrandUseCase(workspaceRepo, brandRepo, eventBus)
	assignBrandMemberUC := brandUC.NewAssignMemberToBrandUseCase(workspaceRepo, brandRepo, eventBus)
	_ = brandUC.NewRevokeBrandAccessUseCase(workspaceRepo, brandRepo, eventBus) // stub used instead

	// ── Channel use cases ─────────────────────────────────────────────────────
	connectChannelUC := channelUC.NewConnectChannelUseCase(
		channelRepo, channelCredRepo, brandRepo, workspaceRepo, cryptoSvc, channelPlanChecker, eventBus)

	// ── Media use cases ───────────────────────────────────────────────────────
	uploadMediaUC := mediaUC.NewUploadMediaAssetUseCase(mediaRepo, postRepo, brandRepo, workspaceRepo, eventBus)

	// ── Post stubs (type mismatches avec handler interfaces — Phase D) ─────────
	createPostStub    := stub.CreatePostStub{}
	submitPostStub    := stub.SubmitPostStub{}
	validatePostStub  := stub.ValidatePostStub{}
	rejectPostStub    := stub.RejectPostStub{}
	schedulePostStub  := stub.SchedulePostStub{}
	publishPostStub   := stub.PublishPostStub{}
	cancelSchedStub   := stub.CancelScheduleStub{}

	// ── Channel disconnect stub ───────────────────────────────────────────────
	disconnectChannelStub    := stub.DisconnectChannelStub{}
	acceptInvitationStub     := stub.AcceptInvitationStub{}
	revokeBrandAccessStub    := stub.RevokeBrandAccessStub{}

	// ── Security admin stubs ──────────────────────────────────────────────────
	enable2FAStub        := stub.Enable2FAStub{}
	disable2FAStub       := stub.Disable2FAStub{}
	sendPwdResetStub     := stub.SendPasswordResetStub{}
	validatePwdResetStub := stub.ValidatePasswordResetStub{}
	unlockUserStub       := stub.UnlockUserStub{}
	rotateKeysStub       := stub.RotateKeysStub{}

	// ── Handlers ──────────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(
		validateCredentialsSvc,
		validate2FASvc,
		revokeSessionSvc,
		enable2FAStub,
		disable2FAStub,
		sendPwdResetStub,
		validatePwdResetStub,
	)
	workspaceHandler := handlers.NewWorkspaceHandler(
		createWorkspaceUC,
		updateWorkspaceUC,
		deleteWorkspaceUC,
		inviteMemberUC,
		acceptInvitationStub,
		createBrandUC,
	)
	brandHandler := handlers.NewBrandHandler(
		updateBrandUC,
		assignBrandMemberUC,
		revokeBrandAccessStub,
	)
	channelHandler := handlers.NewChannelHandler(connectChannelUC, disconnectChannelStub)
	postHandler := handlers.NewPostHandler(
		createPostStub,
		submitPostStub,
		validatePostStub,
		rejectPostStub,
		schedulePostStub,
		publishPostStub,
		cancelSchedStub,
	)
	mediaHandler    := handlers.NewMediaHandler(uploadMediaUC)
	securityHandler := handlers.NewSecurityHandler(unlockUserStub, rotateKeysStub)

	// ── Fiber ─────────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      "WeePost API",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PATCH, DELETE",
	}))

	// ── Middleware factories ───────────────────────────────────────────────────
	rateLimiter := middleware.NewInMemoryRateLimiter()
	jwtMw := middleware.AuthJWTMiddleware(jwtSvc, sessionRepo)
	permMw := func(perm string) fiber.Handler {
		return middleware.PermissionMiddleware(perm, permChecker)
	}
	rateMw := func(max int, window time.Duration) fiber.Handler {
		return middleware.RateLimitMiddleware(max, window, rateLimiter)
	}

	// ── Routes ────────────────────────────────────────────────────────────────
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	authHandler.RegisterRoutes(app, jwtMw, permMw, rateMw)
	workspaceHandler.RegisterRoutes(app, jwtMw, permMw)
	brandHandler.RegisterRoutes(app, jwtMw, permMw)
	channelHandler.RegisterRoutes(app, jwtMw, permMw)
	postHandler.RegisterRoutes(app, jwtMw, permMw)
	mediaHandler.RegisterRoutes(app, jwtMw, permMw)
	securityHandler.RegisterRoutes(app, jwtMw, permMw)

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	port := cfg.AppPort
	go func() {
		log.Printf("WeePost API listening on :%s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

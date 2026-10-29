package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/admin"
	"github.com/asb131/local-cloud/internal/audit"
	"github.com/asb131/local-cloud/internal/auth"
	"github.com/asb131/local-cloud/internal/config"
	"github.com/asb131/local-cloud/internal/netx"
	"github.com/asb131/local-cloud/internal/files"
	"github.com/asb131/local-cloud/internal/static"
	"github.com/asb131/local-cloud/internal/storage"
	"github.com/asb131/local-cloud/internal/ws"
)

type Server struct {
	log             *slog.Logger
	cfg             config.Config
	pool            *pgxpool.Pool
	layout          *storage.Layout
	audit           *audit.Writer
	tokens          *auth.TokenIssuer
	auth            *auth.Service
	adminUsers      *admin.Service
	adminInvites    *admin.InviteService
	files           *files.Store
	hub             *ws.Hub
	trustedProxies  []*net.IPNet
	apiLimiter      *auth.IPLimiter
	router          chi.Router
}

func New(log *slog.Logger, cfg config.Config, pool *pgxpool.Pool, layout *storage.Layout) *Server {
	tokens := auth.NewTokenIssuer(cfg.JWTSecret)
	auditWriter := audit.NewWriter(pool)
	hub := ws.NewHub()
	loginLimiter := auth.NewLoginIPLimiter()
	apiLimiter := auth.NewIPLimiter(800, time.Minute)
	authSvc := auth.NewService(pool, tokens, auditWriter, loginLimiter, layout)
	adminSvc := admin.NewService(pool, auditWriter, layout)
	inviteSvc := admin.NewInviteService(pool, auditWriter)
	fileStore := &files.Store{
		Pool:   pool,
		Layout: layout,
		Audit:  auditWriter,
		Hub:    hub,
		Tokens: tokens,
	}

	trustedProxies, err := netx.ParseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)
	if err != nil {
		log.Error("invalid LOCAL_CLOUD_TRUSTED_PROXY_CIDRS", "err", err)
		trustedProxies = nil
	}

	s := &Server{
		log:            log,
		cfg:            cfg,
		pool:           pool,
		layout:         layout,
		audit:          auditWriter,
		tokens:         tokens,
		auth:           authSvc,
		adminUsers:     adminSvc,
		adminInvites:   inviteSvc,
		files:          fileStore,
		hub:            hub,
		trustedProxies: trustedProxies,
		apiLimiter:     apiLimiter,
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.securityHeaders)
	r.Use(s.requireLANClient)
	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(s.rateLimitAPI)
		api.Post("/auth/login", s.postLogin)
		api.Post("/auth/register", s.postRegister)
		api.Post("/auth/refresh", s.postRefresh)
		api.Get("/files/{id}/content", s.getFileContent)
		api.Get("/files/{id}/office-preview", s.getFileOfficePreview)
		api.Get("/files/{id}/versions/{version_id}/content", s.getVersionContent)
		api.Get("/files/{id}/thumbnail", s.getFileThumbnail)
		api.Get("/ws", s.getWS)

		api.Group(func(protected chi.Router) {
			protected.Use(s.requireAuth)
			protected.Post("/auth/logout", s.postLogout)
			protected.Get("/auth/me", s.getMe)
			protected.Get("/client/features", s.getClientFeatures)

			protected.Get("/folders/{id}", s.getFolder)
			protected.Get("/folders/{id}/children", s.getFolderChildren)
			protected.Post("/folders", s.postFolder)
			protected.Patch("/folders/{id}", s.patchFolder)
			protected.Delete("/folders/{id}", s.deleteFolder)
			protected.Post("/folders/{id}/copy", s.postFolderCopy)

			protected.Post("/uploads", s.postUpload)
			protected.Put("/uploads/{id}/content", s.putUploadContent)
			protected.Post("/uploads/{id}/complete", s.postUploadComplete)
			protected.Delete("/uploads/{id}", s.deleteUpload)

			protected.Get("/files/{id}", s.getFile)
			protected.Patch("/files/{id}", s.patchFile)
			protected.Delete("/files/{id}", s.deleteFile)
			protected.Post("/files/{id}/copy", s.postFileCopy)
			protected.Post("/files/{id}/media-token", s.postMediaToken)
			protected.Get("/files/{id}/onlyoffice-config", s.getOnlyOfficeConfig)
			protected.Post("/downloads/archive", s.postArchive)

			protected.Get("/trash", s.getTrash)
			protected.Post("/trash/files/{id}/restore", s.postTrashRestoreFile)
			protected.Post("/trash/folders/{id}/restore", s.postTrashRestoreFolder)
			protected.Delete("/trash/files/{id}", s.deleteTrashFile)
			protected.Delete("/trash/folders/{id}", s.deleteTrashFolder)
			protected.Get("/files/{id}/versions", s.getFileVersions)
			protected.Post("/files/{id}/versions/{version_id}/media-token", s.postVersionMediaToken)
			protected.Post("/files/{id}/versions/{version_id}/restore", s.postRestoreVersion)
			protected.Get("/favorites", s.getFavorites)
			protected.Post("/favorites", s.postFavorite)
			protected.Delete("/favorites/{id}", s.deleteFavorite)
			protected.Get("/search", s.getSearch)
			protected.Get("/file-categories", s.getFileCategories)
			protected.Get("/file-categories/{category}/files", s.getFileCategoryFiles)

			protected.Route("/admin", func(adm chi.Router) {
				adm.Use(s.requireAdmin)
				adm.Get("/users", s.getAdminUsers)
				adm.Patch("/users/{id}", s.patchAdminUser)
				adm.Delete("/users/{id}", s.deleteAdminUser)
				adm.Post("/invites", s.postAdminInvite)
				adm.Get("/invites", s.getAdminInvites)
				adm.Delete("/invites/{id}", s.deleteAdminInvite)
				adm.Get("/settings", s.getAdminSettings)
				adm.Patch("/settings", s.patchAdminSettings)
				adm.Get("/stats", s.getAdminStats)
				adm.Get("/logs", s.getAdminLogs)
				adm.Post("/rebuild", s.postAdminRebuild)
				adm.Get("/rebuild", s.getAdminRebuild)
			})
		})
	})

	r.Get("/*", static.Handler().ServeHTTP)

	s.router = r
	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) Hub() *ws.Hub {
	return s.hub
}

func (s *Server) FilesStore() *files.Store {
	return s.files
}

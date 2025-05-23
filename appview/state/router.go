package state

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"tangled.sh/tangled.sh/core/appview/middleware"
	oauthhandler "tangled.sh/tangled.sh/core/appview/oauth/handler"
	"tangled.sh/tangled.sh/core/appview/settings"
	"tangled.sh/tangled.sh/core/appview/state/userutil"
)

func (s *State) Router() http.Handler {
	router := chi.NewRouter()

	router.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		pat := chi.URLParam(r, "*")
		if strings.HasPrefix(pat, "did:") || strings.HasPrefix(pat, "@") {
			s.UserRouter().ServeHTTP(w, r)
		} else {
			// Check if the first path element is a valid handle without '@' or a flattened DID
			pathParts := strings.SplitN(pat, "/", 2)
			if len(pathParts) > 0 {
				if userutil.IsHandleNoAt(pathParts[0]) {
					// Redirect to the same path but with '@' prefixed to the handle
					redirectPath := "@" + pat
					http.Redirect(w, r, "/"+redirectPath, http.StatusFound)
					return
				} else if userutil.IsFlattenedDid(pathParts[0]) {
					// Redirect to the unflattened DID version
					unflattenedDid := userutil.UnflattenDid(pathParts[0])
					var redirectPath string
					if len(pathParts) > 1 {
						redirectPath = unflattenedDid + "/" + pathParts[1]
					} else {
						redirectPath = unflattenedDid
					}
					http.Redirect(w, r, "/"+redirectPath, http.StatusFound)
					return
				}
			}
			s.StandardRouter().ServeHTTP(w, r)
		}
	})

	return router
}

func (s *State) UserRouter() http.Handler {
	r := chi.NewRouter()

	// strip @ from user
	r.Use(StripLeadingAt)

	r.With(ResolveIdent(s)).Route("/{user}", func(r chi.Router) {
		r.Get("/", s.Profile)

		r.With(ResolveRepo(s)).Route("/{repo}", func(r chi.Router) {
			r.Use(GoImport(s))

			r.Get("/", s.RepoIndex)
			r.Get("/commits/{ref}", s.RepoLog)
			r.Route("/tree/{ref}", func(r chi.Router) {
				r.Get("/", s.RepoIndex)
				r.Get("/*", s.RepoTree)
			})
			r.Get("/commit/{ref}", s.RepoCommit)
			r.Get("/branches", s.RepoBranches)
			r.Route("/tags", func(r chi.Router) {
				r.Get("/", s.RepoTags)
				r.Route("/{tag}", func(r chi.Router) {
					r.Use(middleware.AuthMiddleware(s.oauth))
					// require auth to download for now
					r.Get("/download/{file}", s.DownloadArtifact)

					// require repo:push to upload or delete artifacts
					//
					// additionally: only the uploader can truly delete an artifact
					// (record+blob will live on their pds)
					r.Group(func(r chi.Router) {
						r.With(RepoPermissionMiddleware(s, "repo:push"))
						r.Post("/upload", s.AttachArtifact)
						r.Delete("/{file}", s.DeleteArtifact)
					})
				})
			})
			r.Get("/blob/{ref}/*", s.RepoBlob)
			r.Get("/raw/{ref}/*", s.RepoBlobRaw)

			r.Route("/issues", func(r chi.Router) {
				r.With(middleware.Paginate).Get("/", s.RepoIssues)
				r.Get("/{issue}", s.RepoSingleIssue)

				r.Group(func(r chi.Router) {
					r.Use(middleware.AuthMiddleware(s.oauth))
					r.Get("/new", s.NewIssue)
					r.Post("/new", s.NewIssue)
					r.Post("/{issue}/comment", s.NewIssueComment)
					r.Route("/{issue}/comment/{comment_id}/", func(r chi.Router) {
						r.Get("/", s.IssueComment)
						r.Delete("/", s.DeleteIssueComment)
						r.Get("/edit", s.EditIssueComment)
						r.Post("/edit", s.EditIssueComment)
					})
					r.Post("/{issue}/close", s.CloseIssue)
					r.Post("/{issue}/reopen", s.ReopenIssue)
				})
			})

			r.Route("/fork", func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(s.oauth))
				r.Get("/", s.ForkRepo)
				r.Post("/", s.ForkRepo)
				r.With(RepoPermissionMiddleware(s, "repo:owner")).Route("/sync", func(r chi.Router) {
					r.Post("/", s.SyncRepoFork)
				})
			})

			r.Route("/pulls", func(r chi.Router) {
				r.Get("/", s.RepoPulls)
				r.With(middleware.AuthMiddleware(s.oauth)).Route("/new", func(r chi.Router) {
					r.Get("/", s.NewPull)
					r.Get("/patch-upload", s.PatchUploadFragment)
					r.Post("/validate-patch", s.ValidatePatch)
					r.Get("/compare-branches", s.CompareBranchesFragment)
					r.Get("/compare-forks", s.CompareForksFragment)
					r.Get("/fork-branches", s.CompareForksBranchesFragment)
					r.Post("/", s.NewPull)
				})

				r.Route("/{pull}", func(r chi.Router) {
					r.Use(ResolvePull(s))
					r.Get("/", s.RepoSinglePull)

					r.Route("/round/{round}", func(r chi.Router) {
						r.Get("/", s.RepoPullPatch)
						r.Get("/interdiff", s.RepoPullInterdiff)
						r.Get("/actions", s.PullActions)
						r.With(middleware.AuthMiddleware(s.oauth)).Route("/comment", func(r chi.Router) {
							r.Get("/", s.PullComment)
							r.Post("/", s.PullComment)
						})
					})

					r.Route("/round/{round}.patch", func(r chi.Router) {
						r.Get("/", s.RepoPullPatchRaw)
					})

					r.Group(func(r chi.Router) {
						r.Use(middleware.AuthMiddleware(s.oauth))
						r.Route("/resubmit", func(r chi.Router) {
							r.Get("/", s.ResubmitPull)
							r.Post("/", s.ResubmitPull)
						})
						r.Post("/close", s.ClosePull)
						r.Post("/reopen", s.ReopenPull)
						// collaborators only
						r.Group(func(r chi.Router) {
							r.Use(RepoPermissionMiddleware(s, "repo:push"))
							r.Post("/merge", s.MergePull)
							// maybe lock, etc.
						})
					})
				})
			})

			// These routes get proxied to the knot
			r.Get("/info/refs", s.InfoRefs)
			r.Post("/git-upload-pack", s.UploadPack)
			r.Post("/git-receive-pack", s.ReceivePack)

			// settings routes, needs auth
			r.Group(func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(s.oauth))
				// repo description can only be edited by owner
				r.With(RepoPermissionMiddleware(s, "repo:owner")).Route("/description", func(r chi.Router) {
					r.Put("/", s.RepoDescription)
					r.Get("/", s.RepoDescription)
					r.Get("/edit", s.RepoDescriptionEdit)
				})
				r.With(RepoPermissionMiddleware(s, "repo:settings")).Route("/settings", func(r chi.Router) {
					r.Get("/", s.RepoSettings)
					r.With(RepoPermissionMiddleware(s, "repo:invite")).Put("/collaborator", s.AddCollaborator)
					r.With(RepoPermissionMiddleware(s, "repo:delete")).Delete("/delete", s.DeleteRepo)
					r.Put("/branches/default", s.SetDefaultBranch)
				})
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.pages.Error404(w)
	})

	return r
}

func (s *State) StandardRouter() http.Handler {
	r := chi.NewRouter()

	r.Handle("/static/*", s.pages.Static())

	r.Get("/", s.Timeline)

	r.With(middleware.AuthMiddleware(s.oauth)).Post("/logout", s.Logout)

	r.Route("/knots", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(s.oauth))
		r.Get("/", s.Knots)
		r.Post("/key", s.RegistrationKey)

		r.Route("/{domain}", func(r chi.Router) {
			r.Post("/init", s.InitKnotServer)
			r.Get("/", s.KnotServerInfo)
			r.Route("/member", func(r chi.Router) {
				r.Use(KnotOwner(s))
				r.Get("/", s.ListMembers)
				r.Put("/", s.AddMember)
				r.Delete("/", s.RemoveMember)
			})
		})
	})

	r.Route("/repo", func(r chi.Router) {
		r.Route("/new", func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(s.oauth))
			r.Get("/", s.NewRepo)
			r.Post("/", s.NewRepo)
		})
		// r.Post("/import", s.ImportRepo)
	})

	r.With(middleware.AuthMiddleware(s.oauth)).Route("/follow", func(r chi.Router) {
		r.Post("/", s.Follow)
		r.Delete("/", s.Follow)
	})

	r.With(middleware.AuthMiddleware(s.oauth)).Route("/star", func(r chi.Router) {
		r.Post("/", s.Star)
		r.Delete("/", s.Star)
	})

	r.Route("/profile", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(s.oauth))
		r.Get("/edit-bio", s.EditBioFragment)
		r.Get("/edit-pins", s.EditPinsFragment)
		r.Post("/bio", s.UpdateProfileBio)
		r.Post("/pins", s.UpdateProfilePins)
	})

	r.Mount("/settings", s.SettingsRouter())
	r.Mount("/", s.OAuthRouter())
	r.Get("/keys/{user}", s.Keys)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.pages.Error404(w)
	})
	return r
}

func (s *State) OAuthRouter() http.Handler {
	oauth := &oauthhandler.OAuthHandler{
		Config:   s.config,
		Pages:    s.pages,
		Resolver: s.resolver,
		Db:       s.db,
		Store:    sessions.NewCookieStore([]byte(s.config.Core.CookieSecret)),
		OAuth:    s.oauth,
		Enforcer: s.enforcer,
		Posthog:  s.posthog,
	}

	return oauth.Router()
}

func (s *State) SettingsRouter() http.Handler {
	settings := &settings.Settings{
		Db:     s.db,
		OAuth:  s.oauth,
		Pages:  s.pages,
		Config: s.config,
	}

	return settings.Router()
}

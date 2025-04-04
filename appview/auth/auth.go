package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/gorilla/sessions"
	"tangled.sh/tangled.sh/core/appview"
)

type Auth struct {
	Store *sessions.CookieStore
}

type AtSessionCreate struct {
	comatproto.ServerCreateSession_Output
	PDSEndpoint string
}

type AtSessionRefresh struct {
	comatproto.ServerRefreshSession_Output
	PDSEndpoint string
}

func Make(secret string) (*Auth, error) {
	store := sessions.NewCookieStore([]byte(secret))
	return &Auth{store}, nil
}

func (a *Auth) CreateInitialSession(ctx context.Context, resolved *identity.Identity, appPassword string) (*comatproto.ServerCreateSession_Output, error) {

	pdsUrl := resolved.PDSEndpoint()
	client := xrpc.Client{
		Host: pdsUrl,
	}

	atSession, err := comatproto.ServerCreateSession(ctx, &client, &comatproto.ServerCreateSession_Input{
		Identifier: resolved.DID.String(),
		Password:   appPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid app password")
	}

	return atSession, nil
}

// Sessionish is an interface that provides access to the common fields of both types.
type Sessionish interface {
	GetAccessJwt() string
	GetActive() *bool
	GetDid() string
	GetDidDoc() *interface{}
	GetHandle() string
	GetRefreshJwt() string
	GetStatus() *string
}

// Create a wrapper type for ServerRefreshSession_Output
type RefreshSessionWrapper struct {
	*comatproto.ServerRefreshSession_Output
}

func (s *RefreshSessionWrapper) GetAccessJwt() string {
	return s.AccessJwt
}

func (s *RefreshSessionWrapper) GetActive() *bool {
	return s.Active
}

func (s *RefreshSessionWrapper) GetDid() string {
	return s.Did
}

func (s *RefreshSessionWrapper) GetDidDoc() *interface{} {
	return s.DidDoc
}

func (s *RefreshSessionWrapper) GetHandle() string {
	return s.Handle
}

func (s *RefreshSessionWrapper) GetRefreshJwt() string {
	return s.RefreshJwt
}

func (s *RefreshSessionWrapper) GetStatus() *string {
	return s.Status
}

// Create a wrapper type for ServerRefreshSession_Output
type CreateSessionWrapper struct {
	*comatproto.ServerCreateSession_Output
}

func (s *CreateSessionWrapper) GetAccessJwt() string {
	return s.AccessJwt
}

func (s *CreateSessionWrapper) GetActive() *bool {
	return s.Active
}

func (s *CreateSessionWrapper) GetDid() string {
	return s.Did
}

func (s *CreateSessionWrapper) GetDidDoc() *interface{} {
	return s.DidDoc
}

func (s *CreateSessionWrapper) GetHandle() string {
	return s.Handle
}

func (s *CreateSessionWrapper) GetRefreshJwt() string {
	return s.RefreshJwt
}

func (s *CreateSessionWrapper) GetStatus() *string {
	return s.Status
}

func (a *Auth) ClearSession(r *http.Request, w http.ResponseWriter) error {
	clientSession, err := a.Store.Get(r, appview.SessionName)
	if err != nil {
		return fmt.Errorf("invalid session", err)
	}
	if clientSession.IsNew {
		return fmt.Errorf("invalid session")
	}
	clientSession.Options.MaxAge = -1
	return clientSession.Save(r, w)
}

func (a *Auth) StoreSession(r *http.Request, w http.ResponseWriter, atSessionish Sessionish, pdsEndpoint string) error {
	clientSession, _ := a.Store.Get(r, appview.SessionName)
	clientSession.Values[appview.SessionHandle] = atSessionish.GetHandle()
	clientSession.Values[appview.SessionDid] = atSessionish.GetDid()
	clientSession.Values[appview.SessionPds] = pdsEndpoint
	clientSession.Values[appview.SessionAccessJwt] = atSessionish.GetAccessJwt()
	clientSession.Values[appview.SessionRefreshJwt] = atSessionish.GetRefreshJwt()
	clientSession.Values[appview.SessionExpiry] = time.Now().Add(time.Minute * 15).Format(time.RFC3339)
	clientSession.Values[appview.SessionAuthenticated] = true
	return clientSession.Save(r, w)
}

func (a *Auth) AuthorizedClient(r *http.Request) (*xrpc.Client, error) {
	clientSession, err := a.Store.Get(r, "appview-session")
	if err != nil || clientSession.IsNew {
		return nil, err
	}

	did := clientSession.Values["did"].(string)
	pdsUrl := clientSession.Values["pds"].(string)
	accessJwt := clientSession.Values["accessJwt"].(string)
	refreshJwt := clientSession.Values["refreshJwt"].(string)

	client := &xrpc.Client{
		Host: pdsUrl,
		Auth: &xrpc.AuthInfo{
			AccessJwt:  accessJwt,
			RefreshJwt: refreshJwt,
			Did:        did,
		},
	}

	return client, nil
}

func (a *Auth) GetSession(r *http.Request) (*sessions.Session, error) {
	return a.Store.Get(r, appview.SessionName)
}

func (a *Auth) GetDid(r *http.Request) string {
	clientSession, err := a.Store.Get(r, appview.SessionName)
	if err != nil || clientSession.IsNew {
		return ""
	}

	return clientSession.Values[appview.SessionDid].(string)
}

func (a *Auth) GetHandle(r *http.Request) string {
	clientSession, err := a.Store.Get(r, appview.SessionName)
	if err != nil || clientSession.IsNew {
		return ""
	}

	return clientSession.Values[appview.SessionHandle].(string)
}

type User struct {
	Handle string
	Did    string
	Pds    string
}

func (a *Auth) GetUser(r *http.Request) *User {
	clientSession, err := a.Store.Get(r, appview.SessionName)

	if err != nil || clientSession.IsNew {
		return nil
	}

	return &User{
		Handle: clientSession.Values[appview.SessionHandle].(string),
		Did:    clientSession.Values[appview.SessionDid].(string),
		Pds:    clientSession.Values[appview.SessionPds].(string),
	}
}

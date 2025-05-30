package repo

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/ipfs/go-cid"
	"tangled.sh/tangled.sh/core/api/tangled"
	"tangled.sh/tangled.sh/core/appview"
	"tangled.sh/tangled.sh/core/appview/db"
	"tangled.sh/tangled.sh/core/appview/pages"
	"tangled.sh/tangled.sh/core/appview/reporesolver"
	"tangled.sh/tangled.sh/core/knotclient"
	"tangled.sh/tangled.sh/core/types"
)

// TODO: proper statuses here on early exit
func (rp *Repo) AttachArtifact(w http.ResponseWriter, r *http.Request) {
	user := rp.oauth.GetUser(r)
	tagParam := chi.URLParam(r, "tag")
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		log.Println("failed to get repo and knot", err)
		rp.pages.Notice(w, "upload", "failed to upload artifact, error in repo resolution")
		return
	}

	tag, err := rp.resolveTag(f, tagParam)
	if err != nil {
		log.Println("failed to resolve tag", err)
		rp.pages.Notice(w, "upload", "failed to upload artifact, error in tag resolution")
		return
	}

	file, handler, err := r.FormFile("artifact")
	if err != nil {
		log.Println("failed to upload artifact", err)
		rp.pages.Notice(w, "upload", "failed to upload artifact")
		return
	}
	defer file.Close()

	client, err := rp.oauth.AuthorizedClient(r)
	if err != nil {
		log.Println("failed to get authorized client", err)
		rp.pages.Notice(w, "upload", "failed to get authorized client")
		return
	}

	uploadBlobResp, err := client.RepoUploadBlob(r.Context(), file)
	if err != nil {
		log.Println("failed to upload blob", err)
		rp.pages.Notice(w, "upload", "Failed to upload blob to your PDS. Try again later.")
		return
	}

	log.Println("uploaded blob", humanize.Bytes(uint64(uploadBlobResp.Blob.Size)), uploadBlobResp.Blob.Ref.String())

	rkey := appview.TID()
	createdAt := time.Now()

	putRecordResp, err := client.RepoPutRecord(r.Context(), &comatproto.RepoPutRecord_Input{
		Collection: tangled.RepoArtifactNSID,
		Repo:       user.Did,
		Rkey:       rkey,
		Record: &lexutil.LexiconTypeDecoder{
			Val: &tangled.RepoArtifact{
				Artifact:  uploadBlobResp.Blob,
				CreatedAt: createdAt.Format(time.RFC3339),
				Name:      handler.Filename,
				Repo:      f.RepoAt.String(),
				Tag:       tag.Tag.Hash[:],
			},
		},
	})
	if err != nil {
		log.Println("failed to create record", err)
		rp.pages.Notice(w, "upload", "Failed to create artifact record. Try again later.")
		return
	}

	log.Println(putRecordResp.Uri)

	tx, err := rp.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("failed to start tx")
		rp.pages.Notice(w, "upload", "Failed to create artifact. Try again later.")
		return
	}
	defer tx.Rollback()

	artifact := db.Artifact{
		Did:       user.Did,
		Rkey:      rkey,
		RepoAt:    f.RepoAt,
		Tag:       tag.Tag.Hash,
		CreatedAt: createdAt,
		BlobCid:   cid.Cid(uploadBlobResp.Blob.Ref),
		Name:      handler.Filename,
		Size:      uint64(uploadBlobResp.Blob.Size),
		MimeType:  uploadBlobResp.Blob.MimeType,
	}

	err = db.AddArtifact(tx, artifact)
	if err != nil {
		log.Println("failed to add artifact record to db", err)
		rp.pages.Notice(w, "upload", "Failed to create artifact. Try again later.")
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Println("failed to add artifact record to db")
		rp.pages.Notice(w, "upload", "Failed to create artifact. Try again later.")
		return
	}

	rp.pages.RepoArtifactFragment(w, pages.RepoArtifactParams{
		LoggedInUser: user,
		RepoInfo:     f.RepoInfo(user),
		Artifact:     artifact,
	})
}

// TODO: proper statuses here on early exit
func (rp *Repo) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	tagParam := chi.URLParam(r, "tag")
	filename := chi.URLParam(r, "file")
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		log.Println("failed to get repo and knot", err)
		return
	}

	tag, err := rp.resolveTag(f, tagParam)
	if err != nil {
		log.Println("failed to resolve tag", err)
		rp.pages.Notice(w, "upload", "failed to upload artifact, error in tag resolution")
		return
	}

	client, err := rp.oauth.AuthorizedClient(r)
	if err != nil {
		log.Println("failed to get authorized client", err)
		return
	}

	artifacts, err := db.GetArtifact(
		rp.db,
		db.FilterEq("repo_at", f.RepoAt),
		db.FilterEq("tag", tag.Tag.Hash[:]),
		db.FilterEq("name", filename),
	)
	if err != nil {
		log.Println("failed to get artifacts", err)
		return
	}
	if len(artifacts) != 1 {
		log.Printf("too many or too little artifacts found")
		return
	}

	artifact := artifacts[0]

	getBlobResp, err := client.SyncGetBlob(r.Context(), artifact.BlobCid.String(), artifact.Did)
	if err != nil {
		log.Println("failed to get blob from pds", err)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(getBlobResp)
}

// TODO: proper statuses here on early exit
func (rp *Repo) DeleteArtifact(w http.ResponseWriter, r *http.Request) {
	user := rp.oauth.GetUser(r)
	tagParam := chi.URLParam(r, "tag")
	filename := chi.URLParam(r, "file")
	f, err := rp.repoResolver.Resolve(r)
	if err != nil {
		log.Println("failed to get repo and knot", err)
		return
	}

	client, _ := rp.oauth.AuthorizedClient(r)

	tag := plumbing.NewHash(tagParam)

	artifacts, err := db.GetArtifact(
		rp.db,
		db.FilterEq("repo_at", f.RepoAt),
		db.FilterEq("tag", tag[:]),
		db.FilterEq("name", filename),
	)
	if err != nil {
		log.Println("failed to get artifacts", err)
		rp.pages.Notice(w, "remove", "Failed to delete artifact. Try again later.")
		return
	}
	if len(artifacts) != 1 {
		rp.pages.Notice(w, "remove", "Unable to find artifact.")
		return
	}

	artifact := artifacts[0]

	if user.Did != artifact.Did {
		log.Println("user not authorized to delete artifact", err)
		rp.pages.Notice(w, "remove", "Unauthorized deletion of artifact.")
		return
	}

	_, err = client.RepoDeleteRecord(r.Context(), &comatproto.RepoDeleteRecord_Input{
		Collection: tangled.RepoArtifactNSID,
		Repo:       user.Did,
		Rkey:       artifact.Rkey,
	})
	if err != nil {
		log.Println("failed to get blob from pds", err)
		rp.pages.Notice(w, "remove", "Failed to remove blob from PDS.")
		return
	}

	tx, err := rp.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("failed to start tx")
		rp.pages.Notice(w, "remove", "Failed to delete artifact. Try again later.")
		return
	}
	defer tx.Rollback()

	err = db.DeleteArtifact(tx,
		db.FilterEq("repo_at", f.RepoAt),
		db.FilterEq("tag", artifact.Tag[:]),
		db.FilterEq("name", filename),
	)
	if err != nil {
		log.Println("failed to remove artifact record from db", err)
		rp.pages.Notice(w, "remove", "Failed to delete artifact. Try again later.")
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Println("failed to remove artifact record from db")
		rp.pages.Notice(w, "remove", "Failed to delete artifact. Try again later.")
		return
	}

	w.Write([]byte{})
}

func (rp *Repo) resolveTag(f *reporesolver.ResolvedRepo, tagParam string) (*types.TagReference, error) {
	tagParam, err := url.QueryUnescape(tagParam)
	if err != nil {
		return nil, err
	}

	us, err := knotclient.NewUnsignedClient(f.Knot, rp.config.Core.Dev)
	if err != nil {
		return nil, err
	}

	result, err := us.Tags(f.OwnerDid(), f.RepoName)
	if err != nil {
		log.Println("failed to reach knotserver", err)
		return nil, err
	}

	var tag *types.TagReference
	for _, t := range result.Tags {
		if t.Tag != nil {
			if t.Reference.Name == tagParam || t.Reference.Hash == tagParam {
				tag = t
			}
		}
	}

	if tag == nil {
		return nil, fmt.Errorf("invalid tag, only annotated tags are supported for artifacts")
	}

	if tag.Tag.Target.IsZero() {
		return nil, fmt.Errorf("invalid tag, only annotated tags are supported for artifacts")
	}

	return tag, nil
}

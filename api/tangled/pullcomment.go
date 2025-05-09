// Code generated by cmd/lexgen (see Makefile's lexgen); DO NOT EDIT.

package tangled

// schema: sh.tangled.repo.pull.comment

import (
	"github.com/bluesky-social/indigo/lex/util"
)

const (
	RepoPullCommentNSID = "sh.tangled.repo.pull.comment"
)

func init() {
	util.RegisterType("sh.tangled.repo.pull.comment", &RepoPullComment{})
} //
// RECORDTYPE: RepoPullComment
type RepoPullComment struct {
	LexiconTypeID string  `json:"$type,const=sh.tangled.repo.pull.comment" cborgen:"$type,const=sh.tangled.repo.pull.comment"`
	Body          string  `json:"body" cborgen:"body"`
	CommentId     *int64  `json:"commentId,omitempty" cborgen:"commentId,omitempty"`
	CreatedAt     string  `json:"createdAt" cborgen:"createdAt"`
	Owner         *string `json:"owner,omitempty" cborgen:"owner,omitempty"`
	Pull          string  `json:"pull" cborgen:"pull"`
	Repo          *string `json:"repo,omitempty" cborgen:"repo,omitempty"`
}

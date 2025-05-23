package git

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

func Fork(repoPath, source string) error {
	_, err := git.PlainClone(repoPath, true, &git.CloneOptions{
		URL:          source,
		SingleBranch: false,
	})

	if err != nil {
		return fmt.Errorf("failed to bare clone repository: %w", err)
	}

	err = exec.Command("git", "-C", repoPath, "config", "receive.hideRefs", "refs/hidden").Run()
	if err != nil {
		return fmt.Errorf("failed to configure hidden refs: %w", err)
	}

	return nil
}

func (g *GitRepo) Sync(branch string) error {
	fetchOpts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/heads/%s", branch, branch)),
		},
	}

	err := g.r.Fetch(fetchOpts)
	if errors.Is(git.NoErrAlreadyUpToDate, err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to fetch origin branch: %s: %w", branch, err)
	}
	return nil
}

// TrackHiddenRemoteRef tracks a hidden remote in the repository. For example,
// if the feature branch on the fork (forkRef) is feature-1, and the remoteRef,
// i.e. the branch we want to merge into, is main, this will result in a refspec:
//
//	+refs/heads/main:refs/hidden/feature-1/main
func (g *GitRepo) TrackHiddenRemoteRef(forkRef, remoteRef string) error {
	fetchOpts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/hidden/%s/%s", remoteRef, forkRef, remoteRef)),
		},
		RemoteName: "origin",
	}

	err := g.r.Fetch(fetchOpts)
	if errors.Is(git.NoErrAlreadyUpToDate, err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to fetch hidden remote: %s: %w", forkRef, err)
	}
	return nil
}

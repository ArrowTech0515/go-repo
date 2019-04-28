package project

import (
	"fmt"
	"strings"

	"code.alibaba-inc.com/force/git-repo/config"
	"github.com/jiangxin/multi-log"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// RemoteTrackBranch gets remote tracking branch
func (v Repository) RemoteTrackBranch(branch string) string {
	if branch == "" {
		branch = v.GetHead()
	}
	if branch == "" {
		return ""
	}
	branch = strings.TrimPrefix(branch, config.RefsHeads)

	cfg := v.Config()
	return cfg.Get("branch." + branch + ".merge")
}

// DeleteBranch deletes a branch
func (v Repository) DeleteBranch(branch string) error {
	// TODO: go-git fail to delete a branch
	// TODO: return v.Raw().DeleteBranch(branch)
	if IsHead(branch) {
		branch = strings.TrimPrefix(branch, config.RefsHeads)
	}
	cmdArgs := []string{
		GIT,
		"branch",
		"-D",
		branch,
		"--",
	}
	return executeCommandIn(v.Path, cmdArgs)
}

// GetHead returns head branch.
func (v Project) GetHead() string {
	return v.WorkRepository.GetHead()
}

// RemoteTracking returns name of current remote tracking branch
func (v Project) RemoteTracking(rev string) string {
	if rev == "" || IsSha(rev) {
		return ""
	}
	if IsHead(rev) {
		rev = strings.TrimPrefix(rev, config.RefsHeads)
	}
	if IsRef(rev) {
		return ""
	}
	return v.Config().Get("branch." + rev + ".merge")
}

// ResolveRevision checks and resolves reference to revid
func (v Project) ResolveRevision(rev string) (string, error) {
	if rev == "" {
		return "", nil
	}

	raw := v.WorkRepository.Raw()
	if raw == nil {
		return "", fmt.Errorf("repository for %s is missing, fail to parse %s", v.Name, rev)
	}

	if rev == "" {
		log.Errorf("empty revision to resolve for proejct '%s'", v.Name)
	}

	revid, err := raw.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return "", err
	}
	return revid.String(), nil
}

// ResolveRemoteTracking returns revision id of current remote tracking branch
func (v Project) ResolveRemoteTracking(rev string) (string, error) {
	raw := v.WorkRepository.Raw()
	if raw == nil {
		return "", fmt.Errorf("repository for %s is missing, fail to parse %s", v.Name, v.Revision)
	}

	if rev == "" {
		log.Errorf("empty Revision for proejct '%s'", v.Name)
	}
	if !IsSha(rev) {
		if IsHead(rev) {
			rev = strings.TrimPrefix(rev, config.RefsHeads)
		}
		if !strings.HasPrefix(rev, config.Refs) {
			rev = fmt.Sprintf("%s%s/%s",
				config.RefsRemotes,
				v.RemoteName,
				rev)
		}
	}
	revid, err := raw.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return "", fmt.Errorf("revision %s in %s not found", rev, v.Name)
	}
	return revid.String(), nil
}

// DeleteBranch deletes a branch
func (v Project) DeleteBranch(branch string) error {
	return v.WorkRepository.DeleteBranch(branch)
}

// StartBranch creates new branch
func (v Project) StartBranch(branch, track string) error {
	var err error

	if track == "" {
		track = v.Revision
	}
	if IsHead(branch) {
		branch = strings.TrimPrefix(branch, config.RefsHeads)
	}

	// Branch is already the current branch
	head := v.GetHead()
	if head == config.RefsHeads+branch {
		return nil
	}

	// Checkout if branch is already exist in repository
	if v.RevisionIsValid(config.RefsHeads + branch) {
		cmdArgs := []string{
			GIT,
			"checkout",
			branch,
			"--",
		}
		return executeCommandIn(v.WorkDir, cmdArgs)
	}

	// Get revid from already fetched tracking for v.Revision
	revid, err := v.ResolveRemoteTracking(v.Revision)
	remote := v.RemoteName
	if remote == "" {
		remote = "origin"
	}

	// Create a new branch
	cmdArgs := []string{
		GIT,
		"checkout",
		"-b",
		branch,
	}
	if revid != "" {
		cmdArgs = append(cmdArgs, revid)
	}
	cmdArgs = append(cmdArgs, "--")
	err = executeCommandIn(v.WorkDir, cmdArgs)
	if err != nil {
		return err
	}

	// Create remote tracking
	v.UpdateBranchTracking(branch, remote, track)
	return nil
}

// GetUploadableBranch returns branch if has commits not upload
func (v Project) GetUploadableBranch(branch string) string {
	// TODO: for subcommand upload
	if branch == "" {
		branch = v.GetHead()
		if branch == "" {
			return ""
		}
	}
	return ""
}

// RemoteTrackBranch gets remote tracking branch
func (v Project) RemoteTrackBranch(branch string) string {
	return v.WorkRepository.RemoteTrackBranch(branch)
}

// UpdateBranchTracking updates branch tracking info.
func (v Project) UpdateBranchTracking(branch, remote, track string) {
	cfg := v.Config()
	if track == "" ||
		IsSha(track) ||
		(IsRef(track) && !IsHead(track)) {
		cfg.Unset("branch." + branch + ".merge")
		cfg.Unset("branch." + branch + ".remote")
		v.SaveConfig(cfg)
		return
	}

	if !IsHead(track) {
		track = config.RefsHeads + track
	}

	cfg.Set("branch."+branch+".merge", track)
	if remote != "" {
		cfg.Set("branch."+branch+".remote", remote)
	}

	v.SaveConfig(cfg)
}

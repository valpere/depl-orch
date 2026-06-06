package agent

import (
	"context"
	"os/exec"
	"strings"
)

// gitSnapshot captures the current working-tree state and returns a tree-ish
// (commit SHA) to restore to later. It does NOT modify the working tree.
func gitSnapshot(ctx context.Context, root string) (string, error) {
	// `git stash create` writes a commit object capturing tracked changes without
	// touching the working tree. Empty output means there were no changes, so the
	// snapshot is simply HEAD.
	out, err := gitOut(ctx, root, "stash", "create", "depl-orch fix-test snapshot")
	if err != nil {
		return "", err
	}
	if sha := strings.TrimSpace(out); sha != "" {
		return sha, nil
	}
	head, err := gitOut(ctx, root, "rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}

// gitRollback restores tracked files in the working tree to the snapshot,
// undoing edits an agent made to existing files.
//
// Deliberate trade-off: it does NOT delete files the agent newly created — that
// would require `git clean -fd`, which could also wipe legitimate untracked files
// in the target repo (.env, build output). fix-test is prompted to edit existing
// source only, so a stray new file on a failed attempt is the lesser evil. Revisit
// when M4 adds steps that legitimately create files.
func gitRollback(ctx context.Context, root, snapshot string) error {
	_, err := gitOut(ctx, root, "checkout", strings.TrimSpace(snapshot), "--", ".")
	return err
}

func gitOut(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	b, err := cmd.CombinedOutput()
	return string(b), err
}

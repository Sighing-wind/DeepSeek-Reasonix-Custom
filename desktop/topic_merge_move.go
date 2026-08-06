package main

// Custom desktop extensions: merge two topics into one and move a topic
// between the Global section and a project workspace. Both operations are
// guarded so they never race a live writer:
//   - topics with active runtime work (running turn / pending prompt /
//     background jobs) are refused;
//   - topics open in a live tab are refused for move/merge-target because a
//     stale in-memory session could later overwrite the relocated transcript.
//
// Locking notes: sessionRemovalMu is non-reentrant and TrashTopic acquires it
// (plus the runtime mutation barrier) itself, so MergeTopics never calls
// TrashTopic while holding sessionRemovalMu. MoveTopicToProject holds
// sessionRemovalMu for its whole body and only calls lock-free helpers.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"reasonix/internal/agent"
	"reasonix/internal/provider"
)

// topicHasOpenTab reports whether any live tab (not a detached runtime) is
// bound to the topic.
func (a *App) topicHasOpenTab(topicID string) bool {
	if strings.TrimSpace(topicID) == "" {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, tab := range a.tabs {
		if tab != nil && tab.TopicID == topicID {
			return true
		}
	}
	return false
}

// MergeTopics appends every message of the source topic to the end of the
// target topic, then moves the source topic into the session trash (its
// records stay recoverable). Both topics must be idle; the target must not be
// open in a tab (the source may be open — TrashTopic detaches it).
//
// Ordering makes the operation retry-safe: the source is trashed BEFORE the
// target write, so a failed merge never leaves a state where retrying would
// append the source messages a second time. If the target write fails after
// the trash, the source stays recoverable in the session trash.
func (a *App) MergeTopics(sourceTopicID, targetTopicID string) error {
	sourceTopicID = strings.TrimSpace(sourceTopicID)
	targetTopicID = strings.TrimSpace(targetTopicID)
	if sourceTopicID == "" || targetTopicID == "" {
		return fmt.Errorf("source and target topic IDs are required")
	}
	if sourceTopicID == targetTopicID {
		return fmt.Errorf("source and target topic must differ")
	}
	if a.topicHasActiveRuntimeWork(sourceTopicID) || a.topicHasActiveRuntimeWork(targetTopicID) {
		return errTopicHasActiveWork
	}
	if a.topicHasOpenTab(targetTopicID) {
		return fmt.Errorf("close the target conversation tab before merging")
	}

	// Read the source transcript into memory (and pin the session paths)
	// while holding sessionRemovalMu, then release it before TrashTopic.
	a.sessionRemovalMu.Lock()
	srcMsgs, tgtPath, err := a.mergeTopicsLoad(sourceTopicID, targetTopicID)
	a.sessionRemovalMu.Unlock()
	if err != nil {
		return err
	}

	if err := a.trashTopic(sourceTopicID); err != nil {
		return err
	}

	a.sessionRemovalMu.Lock()
	defer a.sessionRemovalMu.Unlock()
	tgt, err := agent.LoadSession(tgtPath)
	if err != nil {
		return fmt.Errorf("read target session: %w", err)
	}
	merged := append(append([]provider.Message(nil), tgt.Snapshot()...), srcMsgs...)
	tgt.Rewrite(merged)
	if err := tgt.Save(tgtPath); err != nil {
		return fmt.Errorf("save merged session: %w", err)
	}
	invalidateTopicSessionIndexForPath(tgtPath)
	return nil
}

// mergeTopicsLoad resolves the source/target session paths and returns the
// source messages plus the target path. Caller must hold sessionRemovalMu.
func (a *App) mergeTopicsLoad(sourceTopicID, targetTopicID string) ([]provider.Message, string, error) {
	srcTargets, err := a.topicTrashTargets(sourceTopicID)
	if err != nil {
		return nil, "", err
	}
	tgtTargets, err := a.topicTrashTargets(targetTopicID)
	if err != nil {
		return nil, "", err
	}
	if len(srcTargets) == 0 || len(tgtTargets) == 0 {
		return nil, "", fmt.Errorf("one of the topics has no session records")
	}
	srcPath := srcTargets[0].sessionPath
	tgtPath := tgtTargets[0].sessionPath
	if sameDesktopPath(srcPath, tgtPath) {
		return nil, "", fmt.Errorf("source and target share the same session")
	}
	src, err := agent.LoadSession(srcPath)
	if err != nil {
		return nil, "", fmt.Errorf("read source session: %w", err)
	}
	return src.Snapshot(), tgtPath, nil
}

// MoveTopicToProject relocates an idle, unopened topic (and its saved session
// records) between the Global section and a project workspace. An empty
// targetWorkspaceRoot moves the topic to Global.
//
// The mutation is ordered least-risky first so a failure never leaves a
// half-migrated topic: desktop-projects.json (idempotent, retry-safe) →
// title/source/createdAt maps → the session file family (with rollback) →
// branch meta (with rollback). Every later failure rolls the earlier steps
// back.
func (a *App) MoveTopicToProject(topicID, targetWorkspaceRoot string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topicID is required")
	}
	targetWorkspaceRoot = normalizeProjectRoot(targetWorkspaceRoot)
	if a.topicHasActiveRuntimeWork(topicID) {
		return errTopicHasActiveWork
	}
	if a.topicHasOpenTab(topicID) {
		return fmt.Errorf("close the conversation tab before moving it")
	}

	a.sessionRemovalMu.Lock()
	defer a.sessionRemovalMu.Unlock()

	targets, err := a.topicTrashTargets(topicID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("topic has no session records")
	}
	srcPath := targets[0].sessionPath
	srcDir := targets[0].dir

	var dstDir string
	if targetWorkspaceRoot == "" {
		dstDir = desktopSessionDir(globalWorkspaceRoot())
	} else {
		dstDir = desktopSessionDir(targetWorkspaceRoot)
	}
	if sameDesktopPath(srcDir, dstDir) {
		return nil // already in the destination scope
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create target session dir: %w", err)
	}

	// Capture the source branch meta before the files move.
	oldTitleRoot := ""
	var oldMeta agent.BranchMeta
	hasMeta := false
	if meta, ok, metaErr := agent.LoadBranchMeta(srcPath); metaErr == nil && ok {
		hasMeta = true
		oldMeta = meta
		if meta.DefaultScope() == "project" {
			oldTitleRoot = normalizeProjectRoot(meta.WorkspaceRoot)
		}
	}

	// rollback reverses the projects-file and title-map steps. It is safe to
	// call more than once (idempotent map entry moves, tombstone-clear prepend).
	rollback := func() {
		_ = moveTopicTitleState(targetWorkspaceRoot, oldTitleRoot, topicID)
		_ = removeTopicFromProjectsFile(topicID)
		_ = prependTopicInProjectsFile(oldTitleRoot, topicID, true)
	}

	// 1) Re-home the topic in desktop-projects.json first. remove adds a
	// tombstone; prepend clears it when the topic lands in its new home.
	if err := removeTopicFromProjectsFile(topicID); err != nil {
		return err
	}
	if err := prependTopicInProjectsFile(targetWorkspaceRoot, topicID, true); err != nil {
		return err
	}

	// 2) Move the topic's title/source/createdAt state between scopes.
	if err := moveTopicTitleState(oldTitleRoot, targetWorkspaceRoot, topicID); err != nil {
		rollback()
		return err
	}

	// 3) Move the whole session file family (jsonl, event log, meta, event
	// index, lease and lock sidecars). Roll back every moved file on failure.
	base := filepath.Base(srcPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dstPath := filepath.Join(dstDir, base)
	var moved []string
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		rollback()
		return fmt.Errorf("read source session dir: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != base && !strings.HasPrefix(name, stem+".") {
			continue
		}
		from := filepath.Join(srcDir, name)
		to := filepath.Join(dstDir, name)
		if err := os.Rename(from, to); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(filepath.Join(dstDir, filepath.Base(moved[i])), moved[i])
			}
			rollback()
			return fmt.Errorf("move session file %s: %w", name, err)
		}
		moved = append(moved, from)
	}

	// 4) Re-home the branch meta (scope + workspace root).
	if hasMeta {
		next := oldMeta
		if targetWorkspaceRoot == "" {
			next.Scope = "global"
			next.WorkspaceRoot = ""
		} else {
			next.Scope = "project"
			next.WorkspaceRoot = targetWorkspaceRoot
		}
		if err := agent.SaveBranchMetaPreserveUpdated(dstPath, next); err != nil {
			// Move the files back, restore the original meta, then roll back
			// the projects/title steps.
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(filepath.Join(dstDir, filepath.Base(moved[i])), moved[i])
			}
			_ = agent.SaveBranchMetaPreserveUpdated(srcPath, oldMeta)
			rollback()
			return fmt.Errorf("update branch meta: %w", err)
		}
	}

	invalidateTopicSessionIndex(srcDir)
	invalidateTopicSessionIndex(dstDir)
	return nil
}

// moveTopicTitleState relocates the per-topic title/source/createdAt entries
// between the source scope root and the destination scope root. A scope root
// of "" is the Global section; otherwise it is a project workspace root.
func moveTopicTitleState(srcRoot, dstRoot, topicID string) error {
	if err := moveTopicTitleEntry(srcRoot, dstRoot, topicID, loadTopicTitles, saveTopicTitles); err != nil {
		return fmt.Errorf("move topic title: %w", err)
	}
	if err := moveTopicTitleEntry(srcRoot, dstRoot, topicID, loadTopicTitleSources, saveTopicTitleSources); err != nil {
		return fmt.Errorf("move topic title source: %w", err)
	}
	if err := moveTopicTitleEntry(srcRoot, dstRoot, topicID, loadTopicCreatedAts, saveTopicCreatedAts); err != nil {
		return fmt.Errorf("move topic created-at: %w", err)
	}
	return nil
}

// moveTopicTitleEntry removes topicID from the source scope's map and inserts
// it into the destination scope's map. Moving in the reverse direction is the
// rollback for the forward move.
func moveTopicTitleEntry[V any](srcRoot, dstRoot, topicID string, load func(string) map[string]V, save func(string, map[string]V) error) error {
	src := load(srcRoot)
	value, ok := src[topicID]
	if !ok {
		return nil // nothing to move
	}
	delete(src, topicID)
	if err := save(srcRoot, src); err != nil {
		return err
	}
	dst := load(dstRoot)
	dst[topicID] = value
	return save(dstRoot, dst)
}

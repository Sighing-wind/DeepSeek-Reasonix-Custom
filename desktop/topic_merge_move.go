package main

// Custom desktop extensions: merge two topics into one and move a topic
// between the Global section and a project workspace. Both operations are
// guarded so they never race a live writer:
//   - topics with active runtime work (running turn / pending prompt /
//     background jobs) are refused;
//   - merge refuses a target that is open in a live tab because a stale
//     in-memory session could later overwrite the merged transcript.
//
// Locking notes: sessionRemovalMu is non-reentrant and TrashTopic acquires it
// (plus the runtime mutation barrier) itself, so MergeTopics never calls
// TrashTopic while holding sessionRemovalMu. MoveTopicToProject mirrors the
// TrashTopic pattern: it holds both locks and tears down every runtime of the
// topic before relocating files.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"reasonix/internal/agent"
	"reasonix/internal/control"
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

	// Read the source transcript into memory (and pin the session paths)
	// while holding sessionRemovalMu, then release it before TrashTopic.
	a.sessionRemovalMu.Lock()
	srcMsgs, srcPath, tgtPath, err := a.mergeTopicsLoad(sourceTopicID, targetTopicID)
	a.sessionRemovalMu.Unlock()
	if err != nil {
		return err
	}

	// Snapshot both session families + desktop-projects.json before any
	// mutation so UndoLastOperation (Ctrl+Z / toast action) can revert this merge.
	if err := a.createMergeUndoSnapshot(sourceTopicID, targetTopicID, srcPath, tgtPath); err != nil {
		return fmt.Errorf("create merge undo snapshot: %w", err)
	}

	// Tear down every runtime bound to the TARGET topic (visible tabs and
	// detached sessions) so no in-memory session copy can later overwrite the
	// merged transcript. This lets a merge proceed while the target
	// conversation is open — its tab is closed automatically and the frontend
	// refreshes; reopening shows the merged transcript. The source topic's
	// runtimes are torn down by TrashTopic below.
	var removedTgt []removedSessionRuntime
	var prepareErr error
	func() {
		defer a.lockRuntimeMutation("merge-topics-target")()
		a.sessionRemovalMu.Lock()
		defer a.sessionRemovalMu.Unlock()
		removedTgt, _ = a.removeTopicRuntimeBindings(targetTopicID)
		if len(removedTgt) == 0 {
			return
		}
		closedRemoved := map[control.SessionAPI]bool{}
		defer a.closeRemainingRemovedSessionRuntimesAdmissionHeld(removedTgt, closedRemoved)
		if prepareErr = a.prepareRemovedSessionRuntimes(removedTgt); prepareErr != nil {
			return
		}
		destroys := a.destroyHandlesForSession(filepath.Dir(tgtPath), tgtPath, removedTgt)
		waitDestroyHandles(destroys)
		a.closeRemovedSessionRuntimesForSessionAfterDestroyAdmissionHeld(removedTgt, filepath.Dir(tgtPath), tgtPath, closedRemoved)
	}()
	if prepareErr != nil {
		return prepareErr
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
// source messages, source session path, and target path. Caller must hold
// sessionRemovalMu.
func (a *App) mergeTopicsLoad(sourceTopicID, targetTopicID string) ([]provider.Message, string, string, error) {
	srcTargets, err := a.topicTrashTargets(sourceTopicID)
	if err != nil {
		return nil, "", "", err
	}
	tgtTargets, err := a.topicTrashTargets(targetTopicID)
	if err != nil {
		return nil, "", "", err
	}
	if len(srcTargets) == 0 || len(tgtTargets) == 0 {
		return nil, "", "", fmt.Errorf("one of the topics has no session records")
	}
	srcPath := srcTargets[0].sessionPath
	tgtPath := tgtTargets[0].sessionPath
	if sameDesktopPath(srcPath, tgtPath) {
		return nil, "", "", fmt.Errorf("source and target share the same session")
	}
	src, err := agent.LoadSession(srcPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("read source session: %w", err)
	}
	return src.Snapshot(), srcPath, tgtPath, nil
}

// MoveTopicToProject relocates an idle topic (and its saved session records)
// between the Global section and a project workspace. An empty
// targetWorkspaceRoot moves the topic to Global.
//
// The topic's own conversation view is left open in the destination scope:
// after the move, opening the topic from the sidebar starts the agent with the
// new workspace root (from the re-homed branch meta), so the agent "knows"
// which project it belongs to.
//
// Mutation order (mirrors TrashTopic's runtime handling, then least-risky
// state first):
//  1. unbind + tear down every runtime of the topic (visible tabs AND
//     detached runtimes) so nothing can keep writing to the old project path;
//  2. desktop-projects.json (idempotent, retry-safe);
//  3. title/source/createdAt maps;
//  4. the session file family (with rollback);
//  5. branch meta scope/workspace (with rollback).
//
// Every later failure rolls the earlier steps back.
func (a *App) MoveTopicToProject(topicID, targetWorkspaceRoot string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topicID is required")
	}
	targetWorkspaceRoot = normalizeProjectRoot(targetWorkspaceRoot)
	if a.topicHasActiveRuntimeWork(topicID) {
		return errTopicHasActiveWork
	}

	var fallback fallbackRuntimeTarget
	runMove := func() error {
		defer a.lockRuntimeMutation("move-topic")()
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

		// 1) Unbind and tear down every runtime of this topic (visible tabs +
		// detached). The per-session close below runs BEFORE the file move so
		// no controller can recreate the old-path files afterwards.
		removed, nextFallback := a.removeTopicRuntimeBindings(topicID)
		fallback = nextFallback
		closedRemoved := map[control.SessionAPI]bool{}
		defer a.closeRemainingRemovedSessionRuntimesAdmissionHeld(removed, closedRemoved)
		if err := a.prepareRemovedSessionRuntimes(removed); err != nil {
			return err
		}
		destroys := a.destroyHandlesForSession(srcDir, srcPath, removed)
		waitDestroyHandles(destroys)
		a.closeRemovedSessionRuntimesForSessionAfterDestroyAdmissionHeld(removed, srcDir, srcPath, closedRemoved)

		// rollback reverses the projects-file and title-map steps. It is safe
		// to call more than once (idempotent map entry moves, tombstone-clear
		// prepend).
		rollback := func() {
			_ = moveTopicTitleState(targetWorkspaceRoot, oldTitleRoot, topicID)
			_ = removeTopicFromProjectsFile(topicID)
			_ = prependTopicInProjectsFile(oldTitleRoot, topicID, true)
		}

		// 2) Re-home the topic in desktop-projects.json first. remove adds a
		// tombstone; prepend clears it when the topic lands in its new home.
		if err := removeTopicFromProjectsFile(topicID); err != nil {
			return err
		}
		if err := prependTopicInProjectsFile(targetWorkspaceRoot, topicID, true); err != nil {
			return err
		}

		// 3) Move the topic's title/source/createdAt state between scopes.
		if err := moveTopicTitleState(oldTitleRoot, targetWorkspaceRoot, topicID); err != nil {
			rollback()
			return err
		}

		// 4) Move the whole session file family (jsonl, event log, meta, event
		// index, lease and lock sidecars). Roll back every moved file on
		// failure.
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
			// Skip process-held lock/lease sidecars: they belong to a live
			// runtime in the source directory and are meaningless after the
			// move (Windows refuses to rename an open file, and a fresh lease
			// is re-acquired at the destination on next use).
			if strings.HasSuffix(name, ".lease.lock") || strings.HasSuffix(name, ".lease.json") || strings.HasSuffix(name, ".lock") {
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

		// 5) Re-home the branch meta (scope + workspace root).
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
				// Move the files back, restore the original meta, then roll
				// back the projects/title steps.
				for i := len(moved) - 1; i >= 0; i-- {
					_ = os.Rename(filepath.Join(dstDir, filepath.Base(moved[i])), moved[i])
				}
				_ = agent.SaveBranchMetaPreserveUpdated(srcPath, oldMeta)
				rollback()
				return fmt.Errorf("update branch meta: %w", err)
			}
		}

		// 6) Append a durable system notice so the agent knows it was
		// relocated (not that it misremembered its workspace). Best-effort:
		// a failure here must not undo an already-completed move.
		projectLabel := func(root string) string {
			if root == "" {
				return "Global(全局工作区)"
			}
			return root
		}
		_ = a.appendMoveNotice(dstPath, projectLabel(oldTitleRoot), projectLabel(targetWorkspaceRoot))

		invalidateTopicSessionIndex(srcDir)
		invalidateTopicSessionIndex(dstDir)
		return nil
	}
	if err := runMove(); err != nil {
		return err
	}
	if fallback.needs {
		fallback.topicID = ""
		if err := a.openFallbackRuntime(fallback); err != nil {
			return err
		}
	}
	return nil
}

// appendMoveNotice appends a durable system message to the moved session so
// the agent understands its workspace root changed because the conversation
// was relocated (not because it misremembered). The notice is a plain system
// message in the transcript; the agent reads it when the conversation resumes.
func (a *App) appendMoveNotice(sessionPath, fromLabel, toLabel string) error {
	sess, err := agent.LoadSession(sessionPath)
	if err != nil {
		return err
	}
	msg := provider.Message{
		Role:      provider.RoleSystem,
		Content:   fmt.Sprintf("系统通知:本对话的工作项目已从 %s 移动到 %s。你当前的工作项目就是 %s。这是正常的项目整理操作,不是错误。", fromLabel, toLabel, toLabel),
		CreatedAt: time.Now().UnixMilli(),
	}
	sess.Add(msg)
	return sess.Save(sessionPath)
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

// GetLastUserPreview returns the text of the most recent user-authored turn in
// the topic's session — the opposite of the official SessionMeta preview, which
// stores the FIRST user message for title/listing purposes. The sidebar hover
// card uses this field so it shows what the user last said instead of the
// session's opening message.
func (a *App) GetLastUserPreview(scope, workspaceRoot, topicID string) string {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return ""
	}
	sessionPath, _ := a.findTopicSessionForTarget(scope, workspaceRoot, topicID)
	if sessionPath == "" {
		return ""
	}
	sess, err := agent.LoadSession(sessionPath)
	if err != nil {
		return ""
	}
	msgs := sess.Snapshot()
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != provider.RoleUser {
			continue
		}
		text := agent.UserMessageText(m)
		if !agent.IsUserAuthoredTurn(text) {
			continue
		}
		text = agent.StripPasteDisplayLabel(text)
		text = lastUserPreviewProse(text)
		return truncateLastUserPreview(text)
	}
	return ""
}

// lastUserPreviewProse drops leading @file references a prompt opens with so
// the preview shows what was asked rather than a row of paths (mirrors
// internal/agent previewProse, which is unexported).
func lastUserPreviewProse(s string) string {
	rest := strings.TrimLeft(s, " \t")
	for strings.HasPrefix(rest, "@") {
		end := strings.IndexAny(rest, " \t\r\n")
		if end < 0 {
			return s
		}
		next := strings.TrimLeft(rest[end:], " \t")
		if strings.TrimSpace(next) == "" {
			return s
		}
		rest = next
	}
	if rest == "" {
		return s
	}
	return rest
}

// truncateLastUserPreview clamps the hover-card preview to 200 runes with an
// ellipsis (about four rendered lines).
func truncateLastUserPreview(s string) string {
	if r := []rune(s); len(r) > 200 {
		return string(r[:197]) + "…"
	}
	return s
}

package main

// Operation undo stack: merge and AI-title operations push undo entries; the
// frontend Ctrl+Z (or the toast action) pops the most recent entry and
// restores it. Entries persist under desktopConfigDir()/undo/ and the stack is
// capped so old snapshots are evicted.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	undoStackDirName    = "undo"
	undoStackMaxEntries = 5
	undoStackFile       = "stack.json"
	undoKindMerge       = "merge"
	undoKindAITitle     = "ai-title"
)

func undoStackDir() string {
	return filepath.Join(desktopConfigDir(), undoStackDirName)
}

type undoEntry struct {
	Kind string `json:"kind"`
	TS   int64  `json:"ts"`
	Dir  string `json:"dir"` // snapshot subdirectory name under undoStackDir()
}

func loadUndoStack() []undoEntry {
	b, err := os.ReadFile(filepath.Join(undoStackDir(), undoStackFile))
	if err != nil {
		return nil
	}
	var stack []undoEntry
	if err := json.Unmarshal(b, &stack); err != nil {
		return nil
	}
	return stack
}

func saveUndoStack(stack []undoEntry) error {
	if err := os.MkdirAll(undoStackDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(stack, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(undoStackDir(), undoStackFile), b, 0o644)
}

// pushUndoEntry appends a new stack entry (newest last), caps the stack, and
// returns the new snapshot directory name.
func pushUndoEntry(kind string) (string, error) {
	stack := loadUndoStack()
	ts := time.Now().UnixMilli()
	dir := fmt.Sprintf("%s-%d", kind, ts)
	if err := os.MkdirAll(filepath.Join(undoStackDir(), dir), 0o755); err != nil {
		return "", err
	}
	stack = append(stack, undoEntry{Kind: kind, TS: ts, Dir: dir})
	for len(stack) > undoStackMaxEntries {
		old := stack[0]
		stack = stack[1:]
		_ = os.RemoveAll(filepath.Join(undoStackDir(), old.Dir))
	}
	if err := saveUndoStack(stack); err != nil {
		return "", err
	}
	return dir, nil
}

// ---- merge snapshot ----

type mergeUndoSnapshot struct {
	SourceTopicID string            `json:"sourceTopicID"`
	TargetTopicID string            `json:"targetTopicID"`
	CreatedAt     int64             `json:"createdAt"`
	SourceFiles   map[string]string `json:"sourceFiles"` // artifact name -> original path
	TargetFiles   map[string]string `json:"targetFiles"` // artifact name -> original path
}

// createMergeUndoSnapshot backs up both session file families and the projects
// file so the merge can be reverted. Call before any merge mutation.
func (a *App) createMergeUndoSnapshot(sourceTopicID, targetTopicID, srcPath, tgtPath string) error {
	dirName, err := pushUndoEntry(undoKindMerge)
	if err != nil {
		return err
	}
	dir := filepath.Join(undoStackDir(), dirName)
	srcDir := filepath.Join(dir, "source")
	tgtDir := filepath.Join(dir, "target")
	for _, d := range []string{srcDir, tgtDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if data, err := os.ReadFile(filepath.Join(desktopConfigDir(), desktopProjectsFile)); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "projects.json.bak"), data, 0o644); err != nil {
			return err
		}
	}
	snap := mergeUndoSnapshot{
		SourceTopicID: sourceTopicID,
		TargetTopicID: targetTopicID,
		CreatedAt:     time.Now().UnixMilli(),
		SourceFiles:   map[string]string{},
		TargetFiles:   map[string]string{},
	}
	backupFamily := func(sessionPath, dstDir string, out map[string]string) error {
		key := filepath.Base(sessionPath)
		for _, art := range sessionTrashArtifacts(sessionPath, key) {
			info, err := os.Stat(art.src)
			if err != nil || info.IsDir() {
				continue
			}
			if err := copyFile(art.src, filepath.Join(dstDir, art.name), 0o644); err != nil {
				return err
			}
			out[art.name] = art.src
		}
		return nil
	}
	if err := backupFamily(srcPath, srcDir, snap.SourceFiles); err != nil {
		return err
	}
	if err := backupFamily(tgtPath, tgtDir, snap.TargetFiles); err != nil {
		return err
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "snapshot.json"), b, 0o644)
}

// ---- AI-title snapshot ----

type aiTitleUndoSnapshot struct {
	Scope         string `json:"scope"`
	WorkspaceRoot string `json:"workspaceRoot"`
	TopicID       string `json:"topicID"`
	OldTitle      string `json:"oldTitle"`
	OldSource     string `json:"oldSource"`
}

// pushUndoAITitle snapshots the previous title so UndoLastOperation can
// restore it. Call BEFORE writing the new title.
func (a *App) pushUndoAITitle(scope, workspaceRoot, topicID, oldTitle, oldSource string) error {
	dirName, err := pushUndoEntry(undoKindAITitle)
	if err != nil {
		return err
	}
	snap := aiTitleUndoSnapshot{
		Scope:         scope,
		WorkspaceRoot: workspaceRoot,
		TopicID:       topicID,
		OldTitle:      oldTitle,
		OldSource:     oldSource,
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(undoStackDir(), dirName, "snapshot.json"), b, 0o644)
}

// ---- undo ----

// UndoLastOperation reverts the most recent undoable operation (merge or
// AI-title) and removes it from the stack.
func (a *App) UndoLastOperation() error {
	stack := loadUndoStack()
	if len(stack) == 0 {
		return fmt.Errorf("没有可撤销的操作")
	}
	top := stack[len(stack)-1]
	switch top.Kind {
	case undoKindMerge:
		if err := a.undoMergeEntry(top); err != nil {
			return err
		}
	case undoKindAITitle:
		if err := a.undoAITitleEntry(top); err != nil {
			return err
		}
	default:
		return fmt.Errorf("未知的撤销类型: %s", top.Kind)
	}
	if err := saveUndoStack(stack[:len(stack)-1]); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(undoStackDir(), top.Dir))
	return nil
}

func (a *App) undoMergeEntry(entry undoEntry) error {
	dir := filepath.Join(undoStackDir(), entry.Dir)
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		return fmt.Errorf("撤销快照已丢失")
	}
	var snap mergeUndoSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("撤销快照已损坏")
	}
	if a.topicHasActiveRuntimeWork(snap.SourceTopicID) || a.topicHasActiveRuntimeWork(snap.TargetTopicID) {
		return errTopicHasActiveWork
	}
	if a.topicHasOpenTab(snap.SourceTopicID) || a.topicHasOpenTab(snap.TargetTopicID) {
		return fmt.Errorf("请先关闭相关对话再撤销合并")
	}
	restoreFamily := func(sub string, files map[string]string) error {
		for name, orig := range files {
			backup := filepath.Join(dir, sub, name)
			if _, err := os.Stat(backup); err != nil {
				continue
			}
			if err := copyFile(backup, orig, 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	if err := restoreFamily("target", snap.TargetFiles); err != nil {
		return err
	}
	if err := restoreFamily("source", snap.SourceFiles); err != nil {
		return err
	}
	if bak, err := os.ReadFile(filepath.Join(dir, "projects.json.bak")); err == nil {
		_ = os.WriteFile(filepath.Join(desktopConfigDir(), desktopProjectsFile), bak, 0o644)
	}
	a.emitProjectTreeMetadataChanged()
	return nil
}

func (a *App) undoAITitleEntry(entry undoEntry) error {
	dir := filepath.Join(undoStackDir(), entry.Dir)
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		return fmt.Errorf("撤销快照已丢失")
	}
	var snap aiTitleUndoSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("撤销快照已损坏")
	}
	if a.topicHasActiveRuntimeWork(snap.TopicID) {
		return errTopicHasActiveWork
	}
	if a.topicHasOpenTab(snap.TopicID) {
		return fmt.Errorf("请先关闭相关对话再撤销标题")
	}
	titleRoot := topicTitleRoot(snap.Scope, snap.WorkspaceRoot)
	if err := setTopicTitleWithSource(titleRoot, snap.TopicID, snap.OldTitle, snap.OldSource); err != nil {
		return err
	}
	a.updateOpenTopicTitle(snap.TopicID, snap.OldTitle, snap.OldSource)
	changedDirs := a.updateTopicSessionTitles(snap.TopicID, snap.OldTitle)
	if len(changedDirs) > 0 {
		a.emitProjectTreeChangedForSessionDirs(changedDirs...)
	} else {
		a.emitProjectTreeMetadataChanged()
	}
	return nil
}

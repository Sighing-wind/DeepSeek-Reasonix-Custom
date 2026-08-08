package main

// AI title button (custom edition): SuggestTopicTitle summarizes the most
// recent turns of a conversation with a one-shot no-tool LLM call (no agent
// loop, no history writes) and returns a one-line candidate title. The
// frontend shows it in a confirm dialog; ApplyTopicTitle writes it with a
// dedicated source so the official auto-title rule (source == "auto") never
// overwrites it, while the button stays repeatable (it may even overwrite a
// manually chosen title, per the product decision).

import (
	"context"
	"fmt"
	"strings"
	"time"

	"reasonix/internal/agent"
	"reasonix/internal/boot"
	"reasonix/internal/boundedllm"
	"reasonix/internal/config"
	"reasonix/internal/provider"
)

// topicTitleSourceAI marks a title produced by the AI title button. It is
// deliberately distinct from "auto" (rule-based, would overwrite) and "manual"
// (treated as final by other flows): the button may re-run and overwrite any
// previous title.
const topicTitleSourceAI = "ai"

const (
	// aiTitleRecentTurns bounds how many user/assistant turns feed the summary.
	aiTitleRecentTurns = 10
	// aiTitleMaxContextRunes caps the prompt evidence before the LLM call.
	// The boundedllm total budget is system+evidence; 3500 runes of Chinese
	// text stay well under it.
	aiTitleMaxContextRunes = 3500
	// aiTitleMaxLen is the enforced title length in runes.
	aiTitleMaxLen = 20
)

// SuggestTopicTitle returns a candidate one-line title (<= aiTitleMaxLen
// runes) for the topic by summarizing its most recent turns. It never writes
// anything; the frontend confirms before applying via ApplyTopicTitle.
func (a *App) SuggestTopicTitle(scope, workspaceRoot, topicID string) (string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return "", fmt.Errorf("topicID is required")
	}
	if a.topicHasActiveRuntimeWork(topicID) {
		return "", errTopicHasActiveWork
	}
	sessionPath, _ := a.findTopicSessionForTarget(scope, workspaceRoot, topicID)
	if sessionPath == "" {
		return "", fmt.Errorf("conversation has no saved session")
	}
	sess, err := agent.LoadSession(sessionPath)
	if err != nil {
		return "", fmt.Errorf("read session: %w", err)
	}
	transcript := recentConversationTranscript(sess.Snapshot(), aiTitleRecentTurns, aiTitleMaxContextRunes)
	if strings.TrimSpace(transcript) == "" {
		return "", fmt.Errorf("conversation has no messages to summarize")
	}
	title, err := a.suggestTitleWithLLM(scope, workspaceRoot, transcript)
	if err != nil {
		return "", err
	}
	title = cleanAITitle(title)
	if title == "" || lowSignalTopicTitle(title) {
		return "", fmt.Errorf("未能生成有效标题,请重试")
	}
	if r := []rune(title); len(r) > aiTitleMaxLen {
		title = string(r[:aiTitleMaxLen])
	}
	return title, nil
}

// ApplyTopicTitle writes the confirmed AI title with topicTitleSourceAI and
// refreshes the sidebar, open tabs, and session meta — mirroring the
// maybeAutoTitleTopic finishing sequence.
func (a *App) ApplyTopicTitle(scope, workspaceRoot, topicID, title string) error {
	topicID = strings.TrimSpace(topicID)
	title = strings.TrimSpace(title)
	if topicID == "" {
		return fmt.Errorf("topicID is required")
	}
	if title == "" {
		return fmt.Errorf("标题不能为空")
	}
	if r := []rune(title); len(r) > aiTitleMaxLen {
		title = string(r[:aiTitleMaxLen])
	}
	titleRoot := topicTitleRoot(scope, workspaceRoot)
	oldTitle := loadTopicTitle(titleRoot, topicID)
	oldSource := loadTopicTitleSource(titleRoot, topicID)
	if err := a.pushUndoAITitle(scope, workspaceRoot, topicID, oldTitle, oldSource); err != nil {
		return fmt.Errorf("create title undo snapshot: %w", err)
	}
	if err := setTopicTitleWithSource(titleRoot, topicID, title, topicTitleSourceAI); err != nil {
		return err
	}
	a.updateOpenTopicTitle(topicID, title, topicTitleSourceAI)
	changedDirs := a.updateTopicSessionTitles(topicID, title)
	if len(changedDirs) > 0 {
		a.emitProjectTreeChangedForSessionDirs(changedDirs...)
	} else {
		a.emitProjectTreeMetadataChanged()
	}
	return nil
}

// recentConversationTranscript walks the transcript backwards, collecting the
// most recent user/assistant turns (skipping system/tool noise) into
// "用户: … / 助手: …" lines, then truncates to maxRunes.
func recentConversationTranscript(msgs []provider.Message, maxTurns, maxRunes int) string {
	var parts []string
	turns := 0
	for i := len(msgs) - 1; i >= 0 && turns < maxTurns; i-- {
		m := msgs[i]
		var text string
		switch m.Role {
		case provider.RoleUser:
			text = agent.UserMessageText(m)
			if !agent.IsUserAuthoredTurn(text) {
				continue
			}
			text = agent.StripPasteDisplayLabel(text)
			parts = append(parts, "用户: "+strings.TrimSpace(text))
		case provider.RoleAssistant:
			text = strings.TrimSpace(m.Content)
			if text == "" {
				continue
			}
			parts = append(parts, "助手: "+text)
		default:
			continue
		}
		turns++
	}
	// parts were collected newest-first; reverse for chronological order.
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	out := strings.Join(parts, "\n")
	if r := []rune(out); len(r) > maxRunes {
		out = string(r[:maxRunes])
	}
	return out
}

// suggestTitleWithLLM runs one bounded no-tool provider call (temperature 0,
// no tools, capped output) asking for a single-line title only.
func (a *App) suggestTitleWithLLM(scope, workspaceRoot, transcript string) (string, error) {
	var cfg *config.Config
	if strings.TrimSpace(scope) == "project" && strings.TrimSpace(workspaceRoot) != "" {
		if c, err := config.LoadForRootReadOnly(workspaceRoot); err == nil {
			cfg = c
		}
	}
	if cfg == nil {
		cfg = config.LoadForEdit(config.UserConfigPath())
	}
	ref := strings.TrimSpace(cfg.DefaultModel)
	config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, ref)
	resolved, _, ok := cfg.ResolveDesktopNewSessionModel()
	if !ok || strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("没有可用的模型配置")
	}
	ref = resolved
	entry, ok := cfg.ResolveModel(ref)
	if !ok {
		return "", fmt.Errorf("resolve model %q: no matching provider entry", ref)
	}
	prov, err := boot.NewProvider(entry)
	if err != nil {
		return "", fmt.Errorf("init provider: %w", err)
	}
	system := "You write conversation titles. Given a conversation excerpt, reply with ONLY one concise title of at most 20 characters (Chinese characters count as one each). No quotes, punctuation, newline, or explanation."
	evidence := "Conversation excerpt:\n" + transcript
	text, err := boundedllm.Call(context.Background(), boundedllm.Config{
		Provider:       prov,
		ModelRef:       ref,
		Timeout:        60 * time.Second,
		MaxTokens:      2048,
		MaxOutputBytes: 16 * 1024,
		MaxTotalBytes:  16 * 1024,
	}, system, evidence)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %w", err)
	}
	return strings.TrimSpace(text), nil
}

// cleanAITitle strips quotes, brackets, and sentence punctuation the model
// tends to wrap the title in, then collapses inner whitespace.
func cleanAITitle(title string) string {
	s := strings.TrimSpace(title)
	s = strings.Trim(s, "\"'“”‘’「」『』《》【】[]()（）{}《》<>")
	s = strings.Trim(s, " \t\r\n.,;:!?。，；：！？…-–—")
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

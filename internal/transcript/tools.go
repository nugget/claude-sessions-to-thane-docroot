package transcript

import (
	"fmt"
	"sort"
	"strings"
)

// humanizeTool turns a tool name and its raw input into a compact one-line
// summary plus optional multi-line detail. The goal is a readable narrative of
// what the assistant did, not a faithful dump of every argument.
func humanizeTool(name string, input map[string]any) (summary, detail string, sub *SubagentRef) {
	switch name {
	case "Bash":
		cmd := str(input, "command")
		desc := str(input, "description")
		if desc != "" {
			summary = desc
		} else {
			summary = firstLine(cmd)
		}
		detail = cmd
	case "Read":
		summary = "read " + str(input, "file_path")
	case "Write":
		summary = "write " + str(input, "file_path")
	case "Edit", "MultiEdit":
		summary = "edit " + str(input, "file_path")
	case "NotebookEdit":
		summary = "edit notebook " + str(input, "notebook_path")
	case "Glob":
		summary = "glob " + str(input, "pattern")
		if p := str(input, "path"); p != "" {
			summary += " in " + p
		}
	case "Grep":
		summary = "grep " + quoteShort(str(input, "pattern"))
		if p := str(input, "path"); p != "" {
			summary += " in " + p
		}
	case "LS":
		summary = "list " + str(input, "path")
	case "Task", "Agent":
		st := firstNonEmpty(str(input, "subagent_type"), str(input, "agentType"))
		desc := str(input, "description")
		sub = &SubagentRef{Type: st, Description: desc}
		summary = "dispatch subagent"
		if st != "" {
			summary += " " + st
		}
		if desc != "" {
			summary += " — " + desc
		}
	case "TodoWrite":
		summary = "update todo list"
	case "TaskCreate":
		summary = "create task: " + str(input, "subject")
	case "TaskUpdate":
		summary = "update task " + str(input, "taskId")
		if s := str(input, "status"); s != "" {
			summary += " → " + s
		}
	case "WebFetch":
		summary = "fetch " + str(input, "url")
	case "WebSearch":
		summary = "web search: " + quoteShort(str(input, "query"))
	case "AskUserQuestion":
		summary = "ask the user a question"
	case "ExitPlanMode", "EnterPlanMode":
		summary = strings.ToLower(splitCamel(name))
	default:
		summary = humanizeGeneric(name, input)
	}
	return summary, detail, sub
}

// humanizeGeneric handles unknown and MCP tools. MCP tool names look like
// mcp__server__action; surface them readably and append a couple of scalar
// arguments for context.
func humanizeGeneric(name string, input map[string]any) string {
	display := name
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.SplitN(strings.TrimPrefix(name, "mcp__"), "__", 2)
		if len(parts) == 2 {
			display = parts[1] + " (" + parts[0] + ")"
		}
	}
	hint := scalarHint(input)
	if hint != "" {
		return display + ": " + hint
	}
	return display
}

// scalarHint renders up to two short scalar arguments as "key=value", sorted by
// key for determinism. Long or non-scalar values are skipped.
func scalarHint(input map[string]any) string {
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		v, ok := input[k].(string)
		if !ok {
			if f, isNum := input[k].(float64); isNum {
				parts = append(parts, fmt.Sprintf("%s=%s", k, trimNum(f)))
			}
			continue
		}
		v = firstLine(v)
		if v == "" || len(v) > 60 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		if len(parts) == 2 {
			break
		}
	}
	return strings.Join(parts, ", ")
}

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + " …"
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func quoteShort(s string) string {
	s = firstLine(s)
	if s == "" {
		return s
	}
	return "`" + s + "`"
}

func trimNum(f float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), ".")
}

// splitCamel turns "ExitPlanMode" into "Exit Plan Mode".
func splitCamel(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

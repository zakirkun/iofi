-- CommandCode's CLI was unverified when 0002 shipped, so its row was a
-- placeholder and stayed disabled. These are its documented flags.
--
-- The binary is `command-code`; the short alias is `cmd` on macOS/Linux and
-- `cmdc` on Windows, so the full name is the only one safe to hardcode.
-- Headless is `-p`, and `--output-format json` emits newline-delimited event
-- frames, which is the generic jsonl parser.
UPDATE agent_tools SET
    command    = 'command-code',
    args       = '["-p","{{.Prompt}}","--output-format","json","--permission-mode","auto-accept","--skip-onboarding"]'::jsonb,
    parse_mode = 'jsonl',
    mcp_style  = 'config_file',
    -- Project scope: .mcp.json in the repository root, which is the worktree.
    mcp_target = '{{.Workspace}}/.mcp.json',
    enabled    = true
WHERE kind = 'command_code';

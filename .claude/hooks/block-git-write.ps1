# generic: copy to any Invoke-Build repo
# PreToolUse hook (.claude/settings.json): blocks agent shell commands that commit, push or tag.
# Claude Code passes the tool call as JSON on stdin; the command is tool_input.command. Exit 2
# blocks the call and shows stderr to the agent (https://code.claude.com/docs/en/hooks.md).
$ErrorActionPreference = 'Stop'
try {
    $call = [Console]::In.ReadToEnd() | ConvertFrom-Json
}
catch {
    # Unreadable input: block rather than let an unchecked command through.
    [Console]::Error.WriteLine("block-git-write: could not parse the hook input ($_). Jerry commits. Stage and report.")
    exit 2
}
$command = "$($call.tool_input.command)"
# Listing tags (git tag -l, git tag --list) only reads, so it is allowed.
if ($command -match '\bgit\b(\s+-\S+(\s+\S+)?)*\s+(commit|push|tag(?!\s+(-l|--list)\b))\b') {
    [Console]::Error.WriteLine('Jerry commits. Stage and report.')
    exit 2
}
exit 0

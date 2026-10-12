package main

import "fmt"

// completionCommands and completionFlags list the static top-level
// vocabulary used to generate shell completion scripts. They're kept in
// one place so the completion scripts stay in sync with usage as new
// commands/flags are added.
var completionCommands = []string{
	"list", "get", "set", "toggle", "displays",
	"install", "uninstall", "version", "completion", "help",
}

var completionFlags = []string{
	"--json", "--display", "--force", "--list", "--clear",
}

// cmdCompletion returns the completion script for the given shell, or an
// error if shell isn't one of "bash", "zsh", or "fish".
func cmdCompletion(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletionScript(), nil
	case "zsh":
		return zshCompletionScript(), nil
	case "fish":
		return fishCompletionScript(), nil
	default:
		return "", fmt.Errorf("unsupported shell %q (expected bash, zsh, or fish)", shell)
	}
}

func bashCompletionScript() string {
	return `# resctl bash completion
# Install: resctl completion bash > /etc/bash_completion.d/resctl
#      or: resctl completion bash >> ~/.bashrc
_resctl() {
    local cur prev commands flags
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="` + joinSpace(completionCommands) + `"
    flags="` + joinSpace(completionFlags) + `"

    if [[ "$prev" == "completion" ]]; then
        COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
        return 0
    fi

    if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return 0
    fi

    if [[ $COMP_CWORD -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
        return 0
    fi
}
complete -F _resctl resctl
`
}

func zshCompletionScript() string {
	return `#compdef resctl
# resctl zsh completion
# Install: resctl completion zsh > "${fpath[1]}/_resctl"
_resctl() {
    local -a commands flags
    commands=(` + joinSpace(completionCommands) + `)
    flags=(` + joinSpace(completionFlags) + `)

    if (( CURRENT == 2 )); then
        _describe 'command' commands
        return
    fi

    if [[ "${words[2]}" == "completion" && CURRENT -eq 3 ]]; then
        _values 'shell' bash zsh fish
        return
    fi

    _describe 'flag' flags
}
_resctl
`
}

func fishCompletionScript() string {
	script := "# resctl fish completion\n"
	script += "# Install: resctl completion fish > ~/.config/fish/completions/resctl.fish\n"
	for _, c := range completionCommands {
		script += fmt.Sprintf("complete -c resctl -n '__fish_use_subcommand' -a %s\n", c)
	}
	for _, s := range []string{"bash", "zsh", "fish"} {
		script += fmt.Sprintf("complete -c resctl -n '__fish_seen_subcommand_from completion' -a %s\n", s)
	}
	for _, f := range completionFlags {
		script += fmt.Sprintf("complete -c resctl -l %s\n", trimDashes(f))
	}
	return script
}

func joinSpace(items []string) string {
	s := ""
	for i, item := range items {
		if i > 0 {
			s += " "
		}
		s += item
	}
	return s
}

func trimDashes(s string) string {
	for len(s) > 0 && s[0] == '-' {
		s = s[1:]
	}
	return s
}

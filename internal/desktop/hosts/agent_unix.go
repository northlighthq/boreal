//go:build !windows

package hosts

const agentHint = "Leave empty for $SSH_AUTH_SOCK, or 1Password and Bitwarden when it isn't set. Type a socket path such as ~/.1password/agent.sock to choose one, or none to turn the agent off."

export EDITOR={{ .Vars.editor }}
export EMAIL={{ .Vars.email }}
export GITHUB_TOKEN={{ secret "github_token" }}

alias g=git
alias ll='ls -lah'

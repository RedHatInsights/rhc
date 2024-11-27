#! /bin/bash

: "${PROG:=$(basename "${BASH_SOURCE[0]}")}"

_cli_bash_autocomplete() {
  if [[ "${COMP_WORDS[0]}" != "source" ]]; then
    local cur opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ "$cur" == "-"* ]]; then
      opts=$( "${COMP_WORDS[@]:0:$COMP_CWORD}" "${cur}" --generate-bash-completion )
    else
      opts=$( "${COMP_WORDS[@]:0:$COMP_CWORD}" --generate-bash-completion )
    fi
    # Redirect output of command to array variable COMPREPLY
    mapfile -t COMPREPLY < <(compgen -W "${opts}" -- "${cur}")
    return 0
  fi
}

complete -o bashdefault -o default -o nospace -F _cli_bash_autocomplete "${PROG}"
unset PROG

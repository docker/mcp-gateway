;; Neovim configuration for mcp-gateway project

(fn setup-keybindings []
  "Set up project-specific keybindings"
  (vim.keymap.set :n :<leader>b
                  (fn []
                    (vim.cmd "!make docker-mcp"))
                  {:desc "Build docker-mcp plugin"
                   :noremap true
                   :silent false}))

;; Call setup function
(setup-keybindings)

;; Return module table
{:setup setup-keybindings}

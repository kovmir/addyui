# VHS tape: records the addyui demo GIF for the README.
#
# Requires the addyui binary and a running mockaddy instance:
#   make demo
#
# Output: vhs/demo.gif
Output vhs/demo.gif

Set FontSize 14
Set Width 800
Set Height 600
Set Margin 0
Set Padding 0
Set Shell zsh
Set TypingSpeed 60ms
Set Theme "Gruvbox Light"

# Start the TUI against the mock API server.
Type "./addyui"
Sleep 1s
Enter
Sleep 2s

# Scroll the cursor through the list.
Type "j"
Sleep 400ms
Type "j"
Sleep 400ms
Type "k"
Sleep 400ms

# Open the search field and type a term.
Type "/"
Sleep 500ms
Type "git"
Sleep 500ms
Type "hub"
Sleep 1.2s
Enter
Sleep 1.5s

# Open the first result's details view.
Enter
Sleep 2.5s

# Scroll the details viewport.
Type "J"
Sleep 400ms
Type "J"
Sleep 400ms
Type "K"
Sleep 400ms

# Back to the list, then quit.
Escape
Sleep 1s
Type "q"

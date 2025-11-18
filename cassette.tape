# Define terminal
Set WindowBar Colorful
Set FontSize 16
Set Width 860
Set Height 520
Set Padding 8
Set CursorBlink false
Set LineHeight 1.2
Set LetterSpacing 0.85

# Set PATH to current working directory
# This ensures we're using the local build of masked_fastmail
Hide
Type `export PATH="$PWD:$PATH" && clear`
Enter
Show

# Create 1st alias
Type `masked_fastmail newsletter.com "Very exciting newsletter"`
Sleep 1s
Enter
Wait /state/
Sleep 2s

# Enable 1st alias
Type `masked_fastmail --enable `
Sleep 150ms
Paste
Sleep 1s
Enter
Hide
Sleep 2.5s
Show
Sleep 4s

# Create 2nd alias
Enter
Sleep 200ms
Type `masked_fastmail example.com "Somehow related to newsletter.com"`
Enter
Wait /state/
Sleep 2s

# List aliases
Enter
Sleep 200ms
Type `masked_fastmail --list newsletter.com`
Enter
Sleep 12s

# Define output files
Output demo.gif
Output demo.webm

# Cleanup
Hide
Type masked_fastmail newsletter.com
Enter
Type `masked_fastmail --delete `
Paste
Enter
Sleep 4s

Type masked_fastmail example.com
Enter
Type `masked_fastmail --delete `
Paste
Enter
Sleep 6s
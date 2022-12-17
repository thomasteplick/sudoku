# sudoku
Sudoku Puzzle with entry verification and solution option.
This program is a web application written in Go and HTML.  Build the source code sudoku.go or issue "go run sudoku.go" from a Windows Command Prompt.  In a web browser enter
URL "http://127.0.0.1:8080/sudoku" in the address bar.  The rules of Sudoku are that each column, row, and subgrid must have the numbers 1-9 with no duplicates.  A 
subgrid is a 3x3 grid of cells and there are nine subgrids in the 9x9 grid.  Invalid entries are colored in red when the user issues submit.  The user can reset the 
puzzle, start a new puzzle with a desired number of cells already specified, or request the solution to the current puzzle.  A solution is denoted upon submit with
green status.  A red status denotes an invalid puzzle; i.e., duplicate entries in a row, column or subgrid.  A blue status indicates a valid puzzle but it is not
solved yet; that is, the Sudoku rules are obeyed.

![image](https://user-images.githubusercontent.com/117768679/208264646-ede94a1a-2d48-4554-9f08-923858fd9f02.png)
![image](https://user-images.githubusercontent.com/117768679/208265056-73b0ec4c-d4e6-4e6f-9b36-5b5d750e9631.png)
![image](https://user-images.githubusercontent.com/117768679/208265103-510ca590-973e-4732-a23b-379aab475d68.png)



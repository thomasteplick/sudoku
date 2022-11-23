/*
 Sudoku Puzzle: a 3x3 grid composed of 9 subgrids.
 Each subgrid contains 9 cells, each cell can have the numbers [1, 2, ..., 9].
 There are a total of 81 cells in the grid.
 Rules:
   Unique number [1-9] in each row of the grid
   Unique number [1-9] in each column of the grid
   Unique number [1-9] in each subgrid

   An http server will serve the grid in html using the html/template package.
   The submit handler will validate the user form entries and send the response back,
   highlighting any rule violations.  An initial connection handler will send a
   default Sudoku puzzle to the client web browser.

*/

package main

import (
	"bufio"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	subgrids      int = 9
	rows          int = 9
	cols          int = 9
	tmpl              = "../../src/sudoku/templates/sudoku.html" // html template relative address
	addr              = "127.0.0.1:8080"                         // http server listen address
	pattern           = "/sudoku"                                // http handler initialization pattern
	patternSubmit     = "/sudoku-submit"                         // http handler submit pattern
	initGridFile      = "../../src/sudoku/grids/sudoku50.txt"
	nTrials           = 1000
)

// Each cell in the grid has these properties.
type Cell struct {
	Name     string // row_col_subgrd, row=[0-8], col=[0-8], subgrd=[0-8]
	Value    string // [1-9]
	Invalid  string // invalid or valid user cell value doesn't obey rules
	Readonly string // readonly; given initial grid entries cannot be changed
}

// Sudoku board is a 9x9 grid (81 squares) consisting of nine 3x3 (9 squares) subregions.
// Each square can contain digits 1-9.  Zero signifies empty square.
type Grid [rows][cols]int

// Bad cell
type Bad struct {
	rule string // row, col, subgrid rule violated
	num  int    // 1-9 of the rule
	val  string // "1" - "9"
}

// results from a subregion on number of cells with no values assigned
type result struct {
	notAssigned int   // number of cells that are not assigned a valid digit
	x, y        int   // location of cell to  a digit
	nchoices    int   // number of choices for this subregion
	choices     []int // these are the choices for this subregion
}

type SudokuError []error

type SudokuT struct {
	Grid   map[string]Cell // Sudoku grid
	Status struct {        // status of the puzzle
		Message string // Puzzle state
		State   string //  validstatus, invalidstatus, solvedstatus
	}
}

var (
	set         = make([]bool, rows*cols) // fixed digits are set to true
	errOob      = errors.New("out of bounds")
	errInvalDig = errors.New("invalid digit")
	errFixDig   = errors.New("fixed digit")
	errRules    = errors.New("sudoku rule")
)

var (
	t *template.Template
)

// init parses the html template file done only once
func init() {
	t = template.Must(template.ParseFiles(tmpl))
}

// Error returns one or more errors separated by commas
func (se SudokuError) Error() string {
	var s []string
	for _, err := range se {
		s = append(s, err.Error())
	}
	return strings.Join(s, ", ")
}

// handleSudoku processes the initial Sudoku connection
func handleSudoku(w http.ResponseWriter, r *http.Request) {
	// Open file
	f, err := os.Open(initGridFile)
	if err != nil {
		log.Fatalf("Error opening %s: %v\n", initGridFile, err)
	}
	defer f.Close()

	var sudoku SudokuT
	sudoku.Grid = make(map[string]Cell)

	// Fill in the grid
	input := bufio.NewScanner(f)
	row := 0
	for input.Scan() {
		line := input.Text()
		// Each line has 9 values:  numbers 1-9
		values := strings.Split(line, " ")
		col := 0
		for _, val := range values {
			subgrid := (row/3)*3 + col/3
			name := fmt.Sprintf("%d_%d_%d", row, col, subgrid)
			// Mark as readonly in name by appending "_ro"
			if val != "0" {
				sudoku.Grid[name] = Cell{Name: name + "_ro", Value: val, Invalid: "valid", Readonly: "readonly"}
			} else {
				sudoku.Grid[name] = Cell{Name: name, Value: "", Invalid: "valid", Readonly: ""}
			}
			col++
		}
		row++
	}

	// Set puzzle status
	sudoku.Status.Message = "Status: Valid Puzzle"
	sudoku.Status.State = "validstatus"

	// Write to HTTP output using template and grid
	if err = t.Execute(w, sudoku); err != nil {
		log.Fatalf("Write to HTTP output using template with grid error: %v\n", err)
	}
}

// handleSudokuSubmit processes the Sudoku form submission for evaluate option
func evaluateSudokuSubmit(w http.ResponseWriter, r *http.Request) {

	// histograms for row, column, and subgrid values holding counts
	// for values 1-9.  invalids hold the bad cell information
	// grid is the Sudoku grid showing the values
	var (
		colHist    [cols][10]int8
		rowHist    [rows][10]int8
		sgHist     [subgrids][10]int8
		invalids   []Bad
		emptyCells int = 0
		badValues  int = 0
		sudoku     SudokuT
	)
	sudoku.Grid = make(map[string]Cell)

	// Loop over the rows/columns, get the Request form values, insert into the grid
	// Verify values obey Sudoku rules.
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			subgrid := (row/3)*3 + col/3
			name := fmt.Sprintf("%d_%d_%d", row, col, subgrid)
			// Check for readonly cell first by appending "_ro"
			val := r.FormValue(name + "_ro")
			if len(val) > 0 {
				sudoku.Grid[name] = Cell{Name: name + "_ro", Value: val, Invalid: "valid", Readonly: "readonly"}
				n, _ := strconv.Atoi(val)
				colHist[col][n]++
				// Mark bad if column rule violated
				if colHist[col][n] > 1 {
					invalids = append(invalids, Bad{rule: "col", num: col, val: val})
				}
				rowHist[row][n]++
				// Mark bad if row rule violated
				if rowHist[row][n] > 1 {
					invalids = append(invalids, Bad{rule: "row", num: row, val: val})
				}
				sgHist[subgrid][n]++
				// Mark bad if subgrid rule violated
				if sgHist[subgrid][n] > 1 {
					invalids = append(invalids, Bad{rule: "subgrid", num: subgrid, val: val})
				}
			} else {
				val = r.FormValue(name)
				// check for valid entry that is not empty ""
				if len(val) > 0 {
					if n, err := strconv.Atoi(val); err == nil {
						if n > 0 && n < 10 {
							colHist[col][n]++
							// Mark bad if column rule violated
							if colHist[col][n] > 1 {
								invalids = append(invalids, Bad{rule: "col", num: col, val: val})
							}
							rowHist[row][n]++
							// Mark bad if row rule violated
							if rowHist[row][n] > 1 {
								invalids = append(invalids, Bad{rule: "row", num: row, val: val})
							}
							sgHist[subgrid][n]++
							// Mark bad if subgrid rule violated
							if sgHist[subgrid][n] > 1 {
								invalids = append(invalids, Bad{rule: "subgrid", num: subgrid, val: val})
							}

							// Insert Cell state into the grid for valid
							sudoku.Grid[name] = Cell{Name: name, Value: val, Invalid: "valid", Readonly: ""}
						} else {
							// Mark bad
							sudoku.Grid[name] = Cell{Name: name, Value: val, Invalid: "invalid", Readonly: ""}
							badValues++

						}
					} else {
						// Mark bad
						sudoku.Grid[name] = Cell{Name: name, Value: val, Invalid: "invalid", Readonly: ""}
						badValues++
					}
				} else {
					sudoku.Grid[name] = Cell{Name: name, Value: val, Invalid: "valid", Readonly: ""}
					emptyCells++
				}
			}
		}
	}

	// Set puzzle status
	if len(invalids) > 0 || badValues > 0 {
		sudoku.Status.Message = "Status: Invalid Puzzle"
		sudoku.Status.State = "invalidstatus"
	} else if emptyCells == 0 {
		sudoku.Status.Message = "Status: Solved Puzzle"
		sudoku.Status.State = "solvedstatus"
	} else {
		sudoku.Status.Message = "Status: Valid Puzzle"
		sudoku.Status.State = "validstatus"
	}

	// Process invalid values and mark the cells invalid for non-readonly cells
	for _, bad := range invalids {
		if bad.rule == "row" {
			// Scan the columns of this row and mark any non-readonly invalid cells
			for col := 0; col < 9; col++ {
				subgrid := (bad.num/3)*3 + col/3
				name := fmt.Sprintf("%d_%d_%d", bad.num, col, subgrid)
				if sudoku.Grid[name].Value == bad.val && sudoku.Grid[name].Readonly == "" {
					cell := sudoku.Grid[name]
					cell.Invalid = "invalid"
					sudoku.Grid[name] = cell
				}
			}
		} else if bad.rule == "col" {
			// Scan the rows of this column and mark any invalid cells
			for row := 0; row < 9; row++ {
				subgrid := (row/3)*3 + bad.num/3
				name := fmt.Sprintf("%d_%d_%d", row, bad.num, subgrid)
				if sudoku.Grid[name].Value == bad.val && sudoku.Grid[name].Readonly == "" {
					cell := sudoku.Grid[name]
					cell.Invalid = "invalid"
					sudoku.Grid[name] = cell
				}
			}
		} else { // subgrid
			// Scan the rows and columns of this subgrid and mark any invalid cells.
			// Do not mark Readonly cells.
			r0 := (bad.num / 3) * 3
			c0 := (bad.num % 3) * 3
			for row := r0; row < r0+3; row++ {
				for col := c0; col < c0+3; col++ {
					name := fmt.Sprintf("%d_%d_%d", row, col, bad.num)
					if sudoku.Grid[name].Value == bad.val && sudoku.Grid[name].Readonly == "" {
						cell := sudoku.Grid[name]
						cell.Invalid = "invalid"
						sudoku.Grid[name] = cell
					}
				}
			}
		}
	}

	// Write to HTTP output using template and grid
	if err := t.Execute(w, sudoku); err != nil {
		log.Fatalf("Write to HTTP output using template with grid error: %v\n", err)
	}
}

// resetSudokuSubmit processes the Sudoku form submission for reset option
func resetSudokuSubmit(w http.ResponseWriter, r *http.Request) {

	var sudoku SudokuT
	sudoku.Grid = make(map[string]Cell)

	// Loop over the rows/columns, get the Request form values, insert into the grid
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			subgrid := (row/3)*3 + col/3
			name := fmt.Sprintf("%d_%d_%d", row, col, subgrid)
			// Check for readonly cell first by appending "_ro"
			val := r.FormValue(name + "_ro")
			if len(val) > 0 {
				sudoku.Grid[name] = Cell{Name: name + "_ro", Value: val, Invalid: "valid", Readonly: "readonly"}
			} else {
				sudoku.Grid[name] = Cell{Name: name, Value: "", Invalid: "valid", Readonly: ""}
			}
		}
	}

	// Set puzzle status
	sudoku.Status.Message = "Status: Valid Puzzle"
	sudoku.Status.State = "validstatus"

	// Write to HTTP output using template and grid
	if err := t.Execute(w, sudoku); err != nil {
		log.Fatalf("Write to HTTP output using template with grid error: %v\n", err)
	}
}

// newSudokuSubmit processes the Sudoku form submission for new option
func newSudokuSubmit(w http.ResponseWriter, r *http.Request) {

	var (
		n      int
		err    error
		s      Grid // Grid to use in solver functions
		sudoku SudokuT
	)
	sudoku.Grid = make(map[string]Cell)

	// Get the number of blank cells
	fv := r.FormValue("blankvalues")
	if len(fv) > 0 {
		if n, err = strconv.Atoi(fv); err != nil {
			log.Fatalf("Blank value conversion error: %v\n", err)
		}
	} else {
		log.Fatal("No blank cells specified in dropdown list.")
	}

	// seed the random number generator
	rand.Seed(time.Now().Unix())

	// trials or attempts to solve the Sudoku puzzle
	trial := 0
	results := make(chan result)
	begin := time.Now()
	fmt.Printf("\nStart time: %v\n", begin.Format(time.StampMilli))
trials:
	for trial < nTrials {
		trial++
		fmt.Printf("Trial %v\n", trial)
		nsets := 0
		// loop for nsets
	sets:
		for {
			// launch a goroutine for each 3x3 subregion to find results
			for r := 0; r < rows; r += rows / 3 {
				for c := 0; c < cols; c += cols / 3 {
					go s.getResult(int(r), int(c), results)
				}
			}

			nchoices := 10 // how many digits available for this cell in a sub-region
			var cell result
			noneAssigned := 0 // number of subregions that are completely assigned values
			// Collect results and find subregion with smallest number of satisfying digits
			for i := 0; i < rows; i++ {
				r := <-results
				if r.notAssigned == 0 {
					noneAssigned++
				} else if r.nchoices < nchoices {
					nchoices = r.nchoices
					cell = r
				}
			}

			// puzzle solved if all cells filled with valid values
			if noneAssigned == rows {
				// Show the Sudoku board that is the solution
				fmt.Printf("\n                Solved Sudoku                    \n")
				break trials
			}

			// no solution if nchoices is zero in any subregion with unassigned cells
			// start a new trial
			if nchoices == 0 {
				NewSudoku(r, &sudoku, &s)
				fmt.Printf("Number of sets done for trial %v is %v. Start new trial.\n",
					trial, nsets)
				break sets
			}

			// Assign a random value for the cell and continue this trial
			n := rand.Intn(nchoices)
			s.Set(cell.y, cell.x, cell.choices[n])
			nsets++
		}
	}
	fmt.Printf("\nEnd time: %v, run time: %v\n", time.Now().Format(time.StampMilli), time.Since(begin))

	// Add nflag zeros in random positions to the Grid
	for i := 0; i < n; i++ {
		r := rand.Intn(rows)
		c := rand.Intn(cols)
		// check if already set to zero and try r,c another if so
		for s[r][c] == 0 {
			r = rand.Intn(rows)
			c = rand.Intn(cols)
		}
		s[r][c] = 0
	}

	// Fill in the sudoku
	// Loop over the rows/columns
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			subgrid := (row/3)*3 + col/3
			name := fmt.Sprintf("%d_%d_%d", row, col, subgrid)
			// Set readonly cell by appending "_ro"
			if s[row][col] > 0 {
				val := strconv.Itoa(s[row][col])
				sudoku.Grid[name] = Cell{Name: name + "_ro", Value: val, Invalid: "valid", Readonly: "readonly"}
			} else {
				sudoku.Grid[name] = Cell{Name: name, Value: "", Invalid: "valid", Readonly: ""}
			}
		}
	}

	// Set puzzle status
	sudoku.Status.Message = "Status: Valid Puzzle"
	sudoku.Status.State = "validstatus"

	// Write to HTTP output using template and grid
	if err = t.Execute(w, sudoku); err != nil {
		log.Fatalf("Write to HTTP output using template with grid error: %v\n", err)
	}
}

// getResult finds cells in subregion not set and their satisfying values
func (g *Grid) getResult(r, c int, out chan<- result) {
	var setsSR [10]int
	// count values in this subregion, setsSR
	for i := r; i < r+3; i++ {
		for j := c; j < c+3; j++ {
			setsSR[g[i][j]]++
		}
	}
	// check if all values are set implies no cells have value zero
	if setsSR[0] == 0 {
		out <- result{notAssigned: 0, x: c, y: r, nchoices: 0, choices: nil}
		return
	}

	// count values of the columns of this subregion, setsCL
	var setsCL [3][10]int
	for cc := c; cc < c+3; cc++ {
		for i := 0; i < rows; i++ {
			setsCL[cc-c][g[i][cc]]++
		}
	}

	// count values of the rows of this subregion, setsRW
	var setsRW [3][10]int
	for rr := r; rr < r+3; rr++ {
		for j := 0; j < cols; j++ {
			setsRW[rr-r][g[rr][j]]++
		}
	}

	// check every cell in this 3x3 subregion for non-assignment
	var (
		xc int
		yr int
	)
	min := 10
	cnt := 0
	for rr := r; rr < r+3; rr++ {
		for cc := c; cc < c+3; cc++ {
			if g[rr][cc] == 0 {
				// check counts for values 1 to 9
				for i := 1; i < 10; i++ {
					sets := setsSR[i] + setsCL[cc-c][i] + setsRW[rr-r][i]
					if sets == 0 {
						cnt++
					}
				}
				if cnt < min {
					xc = cc
					yr = rr
					min = cnt
				}
				cnt = 0
			}
		}
	}

	// create result to send to out channel
	unused := make([]int, min)
	j := 0
	// check counts for values 1 to 9 as before
	for i := 1; i < 10; i++ {
		n := setsSR[i] + setsCL[xc-c][i] + setsRW[yr-r][i]
		if n == 0 {
			unused[j] = int(i)
			j++
		}
	}
	res := result{notAssigned: setsSR[0], x: xc, y: yr, choices: unused, nchoices: min}
	out <- res
}

// inBounds checks row,column are inside the grid
func inBounds(row, column int) bool {
	if row < 0 || row >= rows {
		return false
	}
	if column < 0 || column >= cols {
		return false
	}
	return true
}

// validDigit checks that digit is 1-9
func validDigit(digit int) bool {
	return digit > 0 && digit <= 9
}

// ruleCheck enforces the Sudoku rules for digit uniqueness in rows, columns, and subregions
func (g *Grid) ruleCheck(row, col int, digit int) bool {
	// row digit uniqueness constraint
	for c := 0; c < cols; c++ {
		if g[row][c] == digit {
			fmt.Printf("row digit uniqness constraint\n")
			return false
		}
	}

	// column digit uniqueness constraint
	for r := 0; r < rows; r++ {
		if g[r][col] == digit {
			fmt.Printf("column digit uniqueness constraint\n")
			return false
		}
	}

	// subregion digit uniqueness constraint
	// find upper left corner of subregion: (r0,c0)
	r0 := (row / 3) * 3
	c0 := (col / 3) * 3
	for r := r0; r < r0+3; r++ {
		for c := c0; c < c0+3; c++ {
			if g[r][c] == digit {
				fmt.Printf("subregion digit uniqueness constraint\n")
				return false
			}
		}
	}
	return true
}

// set sets a digit at a specific location in the grid
func (g *Grid) Set(row, col, digit int) error {
	// validate digit, location, fixed digit, and Sudoku rules
	var errs SudokuError

	if !inBounds(row, col) {
		errs = append(errs, errOob)
		if !validDigit(digit) {
			errs = append(errs, errInvalDig)
		}
		return errs
	}

	if !validDigit(digit) {
		errs = append(errs, errInvalDig)
	}

	// Check if this location and digit satisfies the Sudoku rules
	if !g.ruleCheck(row, col, digit) {
		errs = append(errs, errRules)
	}

	// Check if this location has a fixed digit which can't be changed
	if set[row*cols+col] {
		errs = append(errs, errFixDig)
	}

	if len(errs) > 0 {
		return errs
	}

	// validaion passed, set the location to digit
	g[row][col] = digit
	return nil
}

// NewSudoku constructs a Sudoku board, initializes it, and sets fixed digits
func NewSudoku(r *http.Request, sudoku *SudokuT, s *Grid) {

	// Loop over the rows/columns, get the Request form values, insert into the grid
	// Transfer sudoku struct to solution matrix, replace blanks with zeros
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			subgrid := (row/3)*3 + col/3
			name := fmt.Sprintf("%d_%d_%d", row, col, subgrid)
			// Check for readonly cell by appending "_ro"
			val := r.FormValue(name + "_ro")
			if len(val) > 0 {
				sudoku.Grid[name] = Cell{Name: name + "_ro", Value: val, Invalid: "valid", Readonly: "readonly"}
				if val, err := strconv.Atoi(val); err != nil {
					fmt.Printf("Atoi error: %v, row = %v, col = %v", err, row, col)
					s[row][col] = 0
				} else {
					s[row][col] = int(val)
				}
			} else {
				sudoku.Grid[name] = Cell{Name: name, Value: "", Invalid: "valid", Readonly: ""}
				s[row][col] = 0
			}
		}
	}
}

// solveSudokuSubmit processes the Sudoku form submission for the solve option
func solveSudokuSubmit(w http.ResponseWriter, r *http.Request) {

	// SudokuT to use in HTML parse and execute
	// Grid to use in solver functions

	var sudoku SudokuT
	sudoku.Grid = make(map[string]Cell)

	// Grid to use in solver functions
	var s Grid

	NewSudoku(r, &sudoku, &s)

	// seed the random number generator
	rand.Seed(time.Now().Unix())

	// trials or attempts to solve the Sudoku puzzle
	trial := 0
	results := make(chan result)
	begin := time.Now()
	fmt.Printf("\nStart time: %v\n", begin.Format(time.StampMilli))
trials:
	for trial < nTrials {
		trial++
		fmt.Printf("Trial %v\n", trial)
		nsets := 0
		// loop for nsets
	sets:
		for {
			// launch a goroutine for each 3x3 subregion to find results
			for r := 0; r < rows; r += rows / 3 {
				for c := 0; c < cols; c += cols / 3 {
					go s.getResult(int(r), int(c), results)
				}
			}

			nchoices := 10 // how many digits available for this cell in a sub-region
			var cell result
			noneAssigned := 0 // number of subregions that are completely assigned values
			// Collect results and find subregion with smallest number of satisfying digits
			for i := 0; i < rows; i++ {
				r := <-results
				if r.notAssigned == 0 {
					noneAssigned++
				} else if r.nchoices < nchoices {
					nchoices = r.nchoices
					cell = r
				}
			}

			// puzzle solved if all cells filled with valid values
			if noneAssigned == rows {
				// Show the Sudoku board that is the solution
				fmt.Printf("\n                Solved Sudoku                    \n")
				break trials
			}

			// no solution if nchoices is zero in any subregion with unassigned cells
			// start a new trial
			if nchoices == 0 {
				NewSudoku(r, &sudoku, &s)
				fmt.Printf("Number of sets done for trial %v is %v. Start new trial.\n",
					trial, nsets)
				break sets
			}

			// Assign a random value for the cell and continue this trial
			n := rand.Intn(nchoices)
			s.Set(cell.y, cell.x, cell.choices[n])
			nsets++
		}
	}
	fmt.Printf("\nEnd time: %v, run time: %v\n", time.Now().Format(time.StampMilli), time.Since(begin))

	// Copy solution in s into sudoku
	// Loop over the rows/columns, get the Request form values, insert into sudoku
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			subgrid := (row/3)*3 + col/3
			name := fmt.Sprintf("%d_%d_%d", row, col, subgrid)
			// Check for readonly cell first by appending "_ro"
			val := r.FormValue(name + "_ro")
			if len(val) > 0 {
				sudoku.Grid[name] = Cell{Name: name + "_ro", Value: val, Invalid: "valid", Readonly: "readonly"}
			} else {
				val := strconv.Itoa(s[row][col])
				sudoku.Grid[name] = Cell{Name: name, Value: val, Invalid: "valid", Readonly: ""}
			}
		}
	}

	// Set puzzle status
	sudoku.Status.Message = "Status: Valid Puzzle"
	sudoku.Status.State = "validstatus"

	// Write to HTTP output using template and grid
	if err := t.Execute(w, sudoku); err != nil {
		log.Fatalf("Write to HTTP output using template with grid error: %v\n", err)
	}
}

// handleSudokuSubmit processes the Sudoku form submissions
func handleSudokuSubmit(w http.ResponseWriter, r *http.Request) {

	// Choose an action to take
	switch r.FormValue("action") {
	case "evaluate":
		evaluateSudokuSubmit(w, r)
	case "reset":
		resetSudokuSubmit(w, r)
	case "new":
		newSudokuSubmit(w, r)
	case "solve":
		solveSudokuSubmit(w, r)
	default:
		log.Fatalf("Invalid action for form submission: %v\n", r.FormValue("action"))
	}
}

func main() {
	// Setup http server with handlers for initial connection and form submissions
	http.HandleFunc(pattern, handleSudoku)
	http.HandleFunc(patternSubmit, handleSudokuSubmit)
	http.ListenAndServe(addr, nil)
}

package git

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// AddedLines returns a slice of [start, end] pairs representing added lines in a diff.
func AddedLines(diffText string) [][2]int {
	var out [][2]int
	sc := bufio.NewScanner(strings.NewReader(diffText))
	newLine := 0
	inHunk := false

	for sc.Scan() {
		line := sc.Text()
		if m := hunkRe.FindStringSubmatch(line); m != nil {
			start, _ := strconv.Atoi(m[1])
			if start == 0 {
				start = 1
			}
			newLine = start
			inHunk = true
			continue
		}
		if !inHunk {
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			if len(out) > 0 && out[len(out)-1][1]+1 == newLine {
				out[len(out)-1][1] = newLine // extend existing range
			} else {
				out = append(out, [2]int{newLine, newLine})
			}
			newLine++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// ignore deletions
		} else if strings.HasPrefix(line, " ") {
			newLine++
		}
	}
	return out
}

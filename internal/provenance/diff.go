package provenance

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

type fileDiff struct {
	path   string
	ranges []LineRange
	binary bool
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func ParseUnifiedDiff(diffText string) []LineRange {
	var out []LineRange
	var current *fileDiff
	inBinaryNotice := false

	flush := func() {
		if current == nil {
			return
		}
		if current.binary {
			out = append(out, LineRange{File: current.path, Start: 0, End: 0, Origin: OriginUnknown})
		} else {
			out = append(out, current.ranges...)
		}
		current = nil
	}

	sc := bufio.NewScanner(strings.NewReader(diffText))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	newLine := 0
	for sc.Scan() {
		line := sc.Text()

		if strings.HasPrefix(line, "diff --git ") {
			flush()
			current = &fileDiff{}
			inBinaryNotice = false
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "Binary files ") {
			current.binary = true
			inBinaryNotice = true
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			p := strings.TrimPrefix(line, "+++ ")
			p = strings.TrimPrefix(p, "b/")
			if p != "/dev/null" {
				current.path = p
			}
			continue
		}
		if m := hunkRe.FindStringSubmatch(line); m != nil {
			start, _ := strconv.Atoi(m[1])
			if start == 0 {
				start = 1
			}
			newLine = start
			inBinaryNotice = false
			continue
		}
		if inBinaryNotice || current.path == "" {
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			addLine(current, &out, newLine)
			newLine++
			continue
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, " ") {
			newLine++
			continue
		}
		if strings.HasPrefix(line, "\\ No newline") {
			continue
		}
	}
	flush()
	return mergeAdjacent(out)
}

func addLine(fd *fileDiff, out *[]LineRange, n int) {
	if len(fd.ranges) > 0 {
		last := &fd.ranges[len(fd.ranges)-1]
		if last.End+1 == n {
			last.End = n
			return
		}
	}
	fd.ranges = append(fd.ranges, LineRange{File: fd.path, Start: n, End: n, Origin: OriginAgent})
}

func mergeAdjacent(rs []LineRange) []LineRange {
	if len(rs) <= 1 {
		return rs
	}
	byFile := map[string][]LineRange{}
	order := []string{}
	for _, r := range rs {
		if _, seen := byFile[r.File]; !seen {
			order = append(order, r.File)
		}
		byFile[r.File] = append(byFile[r.File], r)
	}
	var merged []LineRange
	for _, f := range order {
		prev := byFile[f][0]
		for _, r := range byFile[f][1:] {
			if prev.End+1 == r.Start && prev.Origin == r.Origin {
				prev.End = r.End
				continue
			}
			merged = append(merged, prev)
			prev = r
		}
		merged = append(merged, prev)
	}
	return merged
}

package provenance

import (
	"fmt"
	"strings"
)

type Attribution struct {
	Found  bool
	Record Record
	Range  LineRange
	Note   string
}

func LookupLine(records []Record, file string, line int) Attribution {
	file = normalizePath(file)
	if line < 1 {
		return Attribution{Note: fmt.Sprintf("line %d is invalid", line)}
	}
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		for _, lr := range r.Lines {
			if normalizePath(lr.File) != file {
				continue
			}
			if line >= lr.Start && line <= lr.End {
				return Attribution{Found: true, Record: r, Range: lr}
			}
		}
	}
	return Attribution{Note: "no ledger entry covers this line"}
}

func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return p
}

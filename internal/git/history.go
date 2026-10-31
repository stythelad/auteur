package git

import (
	"strings"
)

func CommitMessage(dir, sha string) (string, error) {
	return Run(dir, "show", "-s", "--format=%B", sha)
}

func CommitAuthor(dir, sha string) (name, email string, err error) {
	out, err := Run(dir, "show", "-s", "--format=%an%x00%ae", sha)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(out, "\x00", 2)
	if len(parts) != 2 {
		return out, "", nil
	}
	return parts[0], parts[1], nil
}

func CommitParents(dir, sha string) ([]string, error) {
	out, err := Run(dir, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(out)
	if len(parts) <= 1 {
		return nil, nil
	}
	return parts[1:], nil
}

func CommitDiff(dir, sha string) (string, error) {
	parents, err := CommitParents(dir, sha)
	if err != nil {
		return "", err
	}
	if len(parents) == 0 {
		return Run(dir, "show", "--format=", "--", sha)
	}
	return Run(dir, "diff", parents[0], sha)
}

func CommitList(dir, base string, limit int, includeMerges bool) ([]string, error) {
	if base == "" {
		base = "HEAD"
	}
	args := []string{"log", "--format=%H"}
	if !includeMerges {
		args = append(args, "--no-merges")
	}
	if limit > 0 {
		args = append(args, "-n", itoa(limit))
	}
	args = append(args, base)
	out, err := Run(dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

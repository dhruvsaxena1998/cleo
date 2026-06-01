package sys

import (
	"os/exec"
	"strconv"
	"strings"
)

// ProcessTreeRSS returns the total RSS (resident set size) in bytes for the
// process pid and all its descendants, walked via ps and pgrep.
func ProcessTreeRSS(pid int) (int64, error) {
	return processTreeRSS(pid, 0)
}

func processTreeRSS(pid int, depth int) (int64, error) {
	if depth > 20 || pid <= 1 {
		return 0, nil
	}

	// Read RSS for this process. ps -o rss= returns RSS in KiB.
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, nil // process may have exited; skip
	}
	rssKiB, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, nil
	}
	total := rssKiB * 1024

	// List direct children via pgrep -P.
	childrenOut, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		// pgrep exits 1 when there are no child processes; that's fine.
		return total, nil
	}
	for _, line := range strings.Split(strings.TrimSpace(string(childrenOut)), "\n") {
		if line == "" {
			continue
		}
		childPid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		childRSS, err := processTreeRSS(childPid, depth+1)
		if err != nil {
			continue
		}
		total += childRSS
	}
	return total, nil
}

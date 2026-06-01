package sys

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// ProcessTreeRSS returns the total RSS (resident set size) in bytes for the
// process pid and all its descendants, walked recursively via /proc.
func ProcessTreeRSS(pid int) (int64, error) {
	return processTreeRSS(pid, 0)
}

func processTreeRSS(pid int, depth int) (int64, error) {
	if depth > 20 || pid <= 1 {
		return 0, nil
	}

	pageSize := int64(unix.Getpagesize())

	// Read RSS for this process from /proc/<pid>/statm.
	statmPath := fmt.Sprintf("/proc/%d/statm", pid)
	data, err := os.ReadFile(statmPath)
	if err != nil {
		return 0, nil // process may have exited; skip
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, nil
	}
	residentPages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, nil
	}
	total := residentPages * pageSize

	// Read direct children from /proc/<pid>/children.
	childrenPath := fmt.Sprintf("/proc/%d/children", pid)
	childrenData, err := os.ReadFile(childrenPath)
	if err != nil {
		// No children file or permission denied; just return current RSS.
		return total, nil
	}
	for _, field := range strings.Fields(string(childrenData)) {
		childPid, err := strconv.Atoi(field)
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

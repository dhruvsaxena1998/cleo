package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/events"
)

func newEventsCmd(getCtx func() *Ctx) *cobra.Command {
	var (
		follow bool
		typ    string
		since  string
		limit  int
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "events <session-id>",
		Short: "Print or tail events for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()
			id := args[0]
			path, archived, err := resolveEventsPath(c, id)
			if err != nil {
				return err
			}
			opts := events.ReadOpts{Type: typ, Limit: limit}
			if since != "" {
				d, err := time.ParseDuration(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				opts.Since = time.Now().Add(-d)
			}
			if archived {
				return printArchivedEvents(cmd, path, opts, asJSON)
			}
			return printActiveEvents(cmd, path, opts, asJSON, follow)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "tail the file (poll-based)")
	cmd.Flags().StringVar(&typ, "type", "", "filter to one event type (e.g. notification)")
	cmd.Flags().StringVar(&since, "since", "", "only events newer than now-<duration> (e.g. 15m)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "show only the most recent N events")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit raw JSONL lines")
	return cmd
}

// resolveEventsPath returns (path, archived, error).
//  1. Exact match on active events file.
//  2. Exact match on archived events file (.jsonl.gz).
//  3. Substring match across active+archived; error if multiple match.
func resolveEventsPath(c *Ctx, id string) (string, bool, error) {
	active := c.Paths.EventsLog(id)
	if _, err := os.Stat(active); err == nil {
		return active, false, nil
	}
	archived := filepath.Join(c.Paths.ArchiveDir(), id+".jsonl.gz")
	if _, err := os.Stat(archived); err == nil {
		return archived, true, nil
	}
	// Substring match: enumerate active and archived directories
	candidates, err := substringEventCandidates(c, id)
	if err != nil {
		return "", false, err
	}
	switch len(candidates) {
	case 0:
		return "", false, fmt.Errorf("unknown session: %s", id)
	case 1:
		return candidates[0].path, candidates[0].archived, nil
	default:
		var ids []string
		for _, c := range candidates {
			ids = append(ids, c.id)
		}
		return "", false, fmt.Errorf("ambiguous session %q matches: %s", id, strings.Join(ids, ", "))
	}
}

type eventCandidate struct {
	id       string
	path     string
	archived bool
}

func substringEventCandidates(c *Ctx, needle string) ([]eventCandidate, error) {
	var out []eventCandidate
	if entries, err := os.ReadDir(c.Paths.EventsDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".jsonl") {
				continue
			}
			id := strings.TrimSuffix(name, ".jsonl")
			if strings.Contains(id, needle) {
				out = append(out, eventCandidate{id: id, path: filepath.Join(c.Paths.EventsDir(), name), archived: false})
			}
		}
	}
	if entries, err := os.ReadDir(c.Paths.ArchiveDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".jsonl.gz") {
				continue
			}
			id := strings.TrimSuffix(name, ".jsonl.gz")
			if strings.Contains(id, needle) {
				out = append(out, eventCandidate{id: id, path: filepath.Join(c.Paths.ArchiveDir(), name), archived: true})
			}
		}
	}
	return out, nil
}

func printActiveEvents(cmd *cobra.Command, path string, opts events.ReadOpts, asJSON, follow bool) error {
	if asJSON {
		return streamJSONL(cmd.OutOrStdout(), path, follow)
	}
	log := events.NewLog(path)
	entries, err := log.ReadFiltered(opts)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "(no events yet)")
		return nil
	}
	printEventsTable(cmd.OutOrStdout(), entries)
	if !follow {
		return nil
	}
	return tailEvents(cmd.OutOrStdout(), path, opts, asJSON)
}

func printArchivedEvents(cmd *cobra.Command, gzPath string, opts events.ReadOpts, asJSON bool) error {
	f, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	if asJSON {
		_, err := io.Copy(cmd.OutOrStdout(), gz)
		return err
	}
	// Decode line by line, apply filters, reuse table printer.
	dec := json.NewDecoder(gz)
	var entries []events.Entry
	for dec.More() {
		var e events.Entry
		if err := dec.Decode(&e); err != nil {
			break
		}
		if opts.Type != "" && e.Type != opts.Type {
			continue
		}
		if !opts.Since.IsZero() && e.At.Before(opts.Since) {
			continue
		}
		entries = append(entries, e)
	}
	if opts.Limit > 0 && len(entries) > opts.Limit {
		entries = entries[len(entries)-opts.Limit:]
	}
	printEventsTable(cmd.OutOrStdout(), entries)
	return nil
}

func printEventsTable(w io.Writer, entries []events.Entry) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	for _, e := range entries {
		ts := dim.Render(e.At.Local().Format("15:04:05"))
		typ := e.Type
		msg := strings.TrimSpace(e.Detail)
		if msg == "" {
			msg = e.Tool
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", ts, typ, msg)
	}
}

func streamJSONL(w io.Writer, path string, follow bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	if !follow {
		return nil
	}
	return tailRaw(w, path)
}

func tailEvents(w io.Writer, path string, opts events.ReadOpts, asJSON bool) error {
	return tailLoop(w, path, func(line []byte) {
		if asJSON {
			fmt.Fprintln(w, string(line))
			return
		}
		var e events.Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return
		}
		if opts.Type != "" && e.Type != opts.Type {
			return
		}
		if !opts.Since.IsZero() && e.At.Before(opts.Since) {
			return
		}
		printEventsTable(w, []events.Entry{e})
	})
}

func tailRaw(w io.Writer, path string) error {
	return tailLoop(w, path, func(line []byte) {
		fmt.Fprintln(w, string(line))
	})
}

// tailLoop polls path, calling onLine for each newly appended JSONL line.
// Reopens the file when the inode changes (so a prune→archive doesn't kill
// the follow). Caller is responsible for stopping the loop (Ctrl-C / process
// exit). The poll cadence is 500 ms.
func tailLoop(w io.Writer, path string, onLine func([]byte)) error {
	openFile := func() (*os.File, os.FileInfo, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, err
		}
		st, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		// Seek to end; we already dumped initial contents in printActiveEvents.
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			f.Close()
			return nil, nil, err
		}
		return f, st, nil
	}

	f, st, err := openFile()
	if err != nil {
		return err
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	buf := make([]byte, 0, 1<<20)
	tmp := make([]byte, 32*1024)
	for {
		n, err := f.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			for {
				idx := bytes.IndexByte(buf, '\n')
				if idx < 0 {
					break
				}
				onLine(buf[:idx])
				buf = buf[idx+1:]
			}
		}
		if err != nil && err != io.EOF {
			return err
		}
		// Sleep, then check for inode change
		time.Sleep(500 * time.Millisecond)
		newSt, statErr := os.Stat(path)
		if statErr == nil && !sameFile(st, newSt) {
			f.Close()
			f, st, err = openFile()
			if err != nil {
				return err
			}
			buf = buf[:0]
		}
	}
}

func sameFile(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return false
	}
	return os.SameFile(a, b)
}

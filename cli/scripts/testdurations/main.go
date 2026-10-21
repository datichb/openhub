//go:build ignore

// testdurations summarises the output of `go test -json`.
//
//	go test ./... -short -count=1 -json > before.json
//	go run scripts/testdurations/main.go before.json
//	go test ./internal/semver/... -json | go run scripts/testdurations/main.go
//	go run scripts/testdurations/main.go -compare before.json after.json
//
// Summary: real duration (first to last Time of the stream), pass / fail /
// skip counts, duration of each package (sorted) and the slowest top-level
// tests. Comparison: the same figures for two files, per package, with the
// difference. Standard library only; the build tag keeps it out of the module
// (go build ./..., go vet ./...).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

type event struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Elapsed float64
}

type counts struct{ pass, fail, skip int }

func (c counts) total() int { return c.pass + c.fail + c.skip }

type testRun struct {
	pkg, name string
	elapsed   float64
}

type report struct {
	first, last time.Time
	skipped     int // lines that are not JSON events
	pkgs        map[string]float64
	pkgFailed   map[string]bool
	tests       counts // every test, subtests included
	topLevel    counts // tests without "/" in their name
	slowest     []testRun
}

func parse(r io.Reader) (*report, error) {
	rep := &report{pkgs: map[string]float64{}, pkgFailed: map[string]bool{}}
	top := map[[2]string]testRun{}
	seen := map[[2]string]string{}

	br := bufio.NewReaderSize(r, 1<<20)
	for {
		line, err := br.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var e event
			if jerr := json.Unmarshal(line, &e); jerr != nil || e.Action == "" {
				rep.skipped++
			} else {
				rep.add(e, top, seen)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	for _, t := range top {
		rep.slowest = append(rep.slowest, t)
	}
	sort.Slice(rep.slowest, func(i, j int) bool {
		if rep.slowest[i].elapsed != rep.slowest[j].elapsed {
			return rep.slowest[i].elapsed > rep.slowest[j].elapsed
		}
		return rep.slowest[i].pkg+rep.slowest[i].name < rep.slowest[j].pkg+rep.slowest[j].name
	})
	return rep, nil
}

func (rep *report) add(e event, top map[[2]string]testRun, seen map[[2]string]string) {
	if !e.Time.IsZero() {
		if rep.first.IsZero() || e.Time.Before(rep.first) {
			rep.first = e.Time
		}
		if e.Time.After(rep.last) {
			rep.last = e.Time
		}
	}
	if e.Action != "pass" && e.Action != "fail" && e.Action != "skip" {
		return
	}
	if e.Test == "" {
		if e.Elapsed > 0 {
			rep.pkgs[e.Package] = e.Elapsed
		}
		if e.Action == "fail" {
			rep.pkgFailed[e.Package] = true
		}
		return
	}
	key := [2]string{e.Package, e.Test}
	if prev, ok := seen[key]; ok {
		rep.uncount(prev, e.Test)
	}
	seen[key] = e.Action
	rep.count(e.Action, e.Test)
	if !strings.Contains(e.Test, "/") {
		top[key] = testRun{pkg: e.Package, name: e.Test, elapsed: e.Elapsed}
	}
}

func (rep *report) count(action, test string)   { rep.bump(action, test, 1) }
func (rep *report) uncount(action, test string) { rep.bump(action, test, -1) }

func (rep *report) bump(action, test string, d int) {
	targets := []*counts{&rep.tests}
	if !strings.Contains(test, "/") {
		targets = append(targets, &rep.topLevel)
	}
	for _, c := range targets {
		switch action {
		case "pass":
			c.pass += d
		case "fail":
			c.fail += d
		case "skip":
			c.skip += d
		}
	}
}

func (rep *report) wall() time.Duration {
	if rep.first.IsZero() {
		return 0
	}
	return rep.last.Sub(rep.first)
}

func (rep *report) cpu() float64 {
	var s float64
	for _, v := range rep.pkgs {
		s += v
	}
	return s
}

func short(pkg string) string {
	if i := strings.Index(pkg, "/cli"); i >= 0 {
		pkg = pkg[i+len("/cli"):]
	}
	return "." + pkg
}

func fmtDur(d time.Duration) string {
	return fmt.Sprintf("%.1f s (%.1f min)", d.Seconds(), d.Minutes())
}

func summary(w io.Writer, rep *report, topN int) {
	fmt.Fprintf(w, "Real duration:       %s\n", fmtDur(rep.wall()))
	fmt.Fprintf(w, "Sum of packages:     %.1f s\n", rep.cpu())
	fmt.Fprintf(w, "Tests (all):         pass=%d fail=%d skip=%d total=%d\n", rep.tests.pass, rep.tests.fail, rep.tests.skip, rep.tests.total())
	fmt.Fprintf(w, "Tests (top level):   pass=%d fail=%d skip=%d total=%d\n", rep.topLevel.pass, rep.topLevel.fail, rep.topLevel.skip, rep.topLevel.total())
	if len(rep.pkgFailed) > 0 {
		fmt.Fprintf(w, "Failed packages:     %d\n", len(rep.pkgFailed))
	}
	if rep.skipped > 0 {
		fmt.Fprintf(w, "Unreadable lines:    %d (ignored)\n", rep.skipped)
	}

	names := make([]string, 0, len(rep.pkgs))
	for p := range rep.pkgs {
		names = append(names, p)
	}
	sort.Slice(names, func(i, j int) bool {
		if rep.pkgs[names[i]] != rep.pkgs[names[j]] {
			return rep.pkgs[names[i]] > rep.pkgs[names[j]]
		}
		return names[i] < names[j]
	})
	fmt.Fprintf(w, "\nPackages by duration (%d):\n", len(names))
	for _, p := range names {
		mark := ""
		if rep.pkgFailed[p] {
			mark = "  FAIL"
		}
		fmt.Fprintf(w, "  %9.2f s  %s%s\n", rep.pkgs[p], short(p), mark)
	}

	if topN > len(rep.slowest) {
		topN = len(rep.slowest)
	}
	fmt.Fprintf(w, "\nSlowest top-level tests (%d):\n", topN)
	for _, t := range rep.slowest[:topN] {
		fmt.Fprintf(w, "  %9.2f s  %s  %s\n", t.elapsed, short(t.pkg), t.name)
	}
}

func signed(v float64) string { return fmt.Sprintf("%+.2f", v) }

func percent(before, after float64) string {
	if before == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%+.0f%%", (after-before)/before*100)
}

func compare(w io.Writer, before, after *report) {
	fmt.Fprintf(w, "%-20s %14s %14s %12s\n", "", "before", "after", "difference")
	fmt.Fprintf(w, "%-20s %12.1f s %12.1f s %10s s  %s\n", "Real duration", before.wall().Seconds(), after.wall().Seconds(),
		signed(after.wall().Seconds()-before.wall().Seconds()), percent(before.wall().Seconds(), after.wall().Seconds()))
	fmt.Fprintf(w, "%-20s %12.1f s %12.1f s %10s s  %s\n", "Sum of packages", before.cpu(), after.cpu(),
		signed(after.cpu()-before.cpu()), percent(before.cpu(), after.cpu()))
	row := func(label string, b, a int) {
		fmt.Fprintf(w, "%-20s %14d %14d %+12d\n", label, b, a, a-b)
	}
	row("Tests pass", before.tests.pass, after.tests.pass)
	row("Tests fail", before.tests.fail, after.tests.fail)
	row("Tests skip", before.tests.skip, after.tests.skip)
	row("Tests total", before.tests.total(), after.tests.total())
	row("Top-level total", before.topLevel.total(), after.topLevel.total())

	set := map[string]bool{}
	for p := range before.pkgs {
		set[p] = true
	}
	for p := range after.pkgs {
		set[p] = true
	}
	names := make([]string, 0, len(set))
	for p := range set {
		names = append(names, p)
	}
	delta := func(p string) float64 { return after.pkgs[p] - before.pkgs[p] }
	abs := func(v float64) float64 {
		if v < 0 {
			return -v
		}
		return v
	}
	sort.Slice(names, func(i, j int) bool {
		di, dj := abs(delta(names[i])), abs(delta(names[j]))
		if di != dj {
			return di > dj
		}
		return names[i] < names[j]
	})
	fmt.Fprintf(w, "\nPackages, by largest difference (%d):\n", len(names))
	fmt.Fprintf(w, "  %10s %10s %10s %7s  %s\n", "before s", "after s", "diff s", "%", "package")
	for _, p := range names {
		b, bok := before.pkgs[p]
		a, aok := after.pkgs[p]
		note := ""
		switch {
		case !bok:
			note = "  (only after)"
		case !aok:
			note = "  (only before)"
		}
		fmt.Fprintf(w, "  %10.2f %10.2f %10s %7s  %s%s\n", b, a, signed(a-b), percent(b, a), short(p), note)
	}
}

func load(path string) (*report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rep, err := parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return rep, nil
}

func main() {
	topN := flag.Int("top", 20, "number of slowest top-level tests to list")
	cmp := flag.Bool("compare", false, "compare two files: -compare before.json after.json")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: testdurations [-top N] [file.json]   (stdin when no file)")
		fmt.Fprintln(os.Stderr, "       testdurations -compare before.json after.json")
		flag.PrintDefaults()
	}
	flag.Parse()

	fail := func(err error) {
		fmt.Fprintln(os.Stderr, "testdurations:", err)
		os.Exit(1)
	}

	if *cmp {
		if flag.NArg() != 2 {
			flag.Usage()
			os.Exit(2)
		}
		before, err := load(flag.Arg(0))
		if err != nil {
			fail(err)
		}
		after, err := load(flag.Arg(1))
		if err != nil {
			fail(err)
		}
		compare(os.Stdout, before, after)
		return
	}

	var rep *report
	var err error
	switch flag.NArg() {
	case 0:
		rep, err = parse(os.Stdin)
	case 1:
		rep, err = load(flag.Arg(0))
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
	summary(os.Stdout, rep, *topN)
}

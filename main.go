package main

/*
⚠️ This tool uses flags ⚠️

Verbose accepts none, lite, or heavy. It's automatically set to none. lite gives some useful info while heavy logs everything it can. An example use of verbose would be --verbose lite

Queue lets you decide if you want to queue up the languages being ran instead of running them simultaneously. It's just --queue, no additional stuff. If you queue it *MIGHT* reduce CPU strain.

Chaos decides how many languages to use. low chaos runs a few languages that were in ptrsg 1.0.0 while high chaos, the default, runs ALL languages.

S is the flag for how long the seed should be, 1-512. Basically it either prints the entire full seed (512) or cuts it down a bit. An example command would be -S 128.
*/

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"
)

const version = "2.1.0 [Go]"

type Verbosity int

const (
	VerbosityNone Verbosity = iota
	VerbosityLite
	VerbosityHeavy
)

func parseFlags() (Verbosity, bool, string, int) {
	args := os.Args[1:]
	verbosity := VerbosityNone
	newArgs := []string{os.Args[0]}

	for i := 0; i < len(args); i++ {
		if args[i] == "--verbose" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				switch args[i+1] {
				case "none":
					verbosity = VerbosityNone
				case "lite":
					verbosity = VerbosityLite
				case "heavy":
					verbosity = VerbosityHeavy
				default:
					fmt.Fprintf(os.Stderr, "invalid verbosity %q\n", args[i+1])
					os.Exit(1)
				}
				i++
			} else {
				verbosity = VerbosityHeavy
			}
		} else {
			newArgs = append(newArgs, args[i])
		}
	}

	os.Args = newArgs

	queue := flag.Bool("queue", false, "")
	chaos := flag.String("chaos", "high", "")
	seed := flag.Int("S", 512, "")

	flag.Parse()

	if *seed < 1 || *seed > 512 {
		fmt.Fprintln(os.Stderr, "--seed must be 1-512")
		os.Exit(1)
	}

	if *chaos != "low" && *chaos != "high" {
		fmt.Fprintln(os.Stderr, "--chaos must be low or high")
		os.Exit(1)
	}

	return verbosity, *queue, *chaos, *seed
}

// preflightLangCheck prints version info for each required tool
// and exits if any are missing.
func preflightLangCheck(v Verbosity) {
	tools := []struct {
		name  string
		flags []string
	}{
		{"lua", []string{"-v"}},
		{"python", []string{"--version"}},
		{"node", []string{"--version"}},
		{"g++", []string{"--version"}},
		{"rustc", []string{"--version"}},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	missing := []string{}

	for _, t := range tools {
		wg.Add(1)
		go func(name string, flags []string) {
			defer wg.Done()
			cmd := exec.Command(name, flags...)
			out, err := cmd.CombinedOutput()
			if v == VerbosityHeavy {
				fmt.Printf("[DEBUG] %s %s → ", name, strings.Join(flags, " "))
				if err != nil {
					fmt.Printf("error: %v\n", err)
				} else {
					fmt.Printf(strings.TrimSpace(string(out)) + "\n")
				}
			}
			if err != nil {
				mu.Lock()
				missing = append(missing, name)
				mu.Unlock()
			}
		}(t.name, t.flags)
	}
	wg.Wait()

	if len(missing) > 0 {
		fmt.Printf("Preflight check failed: %s missing!\n", strings.Join(missing, ", "))
		os.Exit(1)
	}

	if v == VerbosityHeavy {
		fmt.Println("[DEBUG] Preflight check passed: all required tools are available")
	}
}

var codeMap = map[string]string{
	"lua": `local t = {}
for i = 1, 100000 do
    t[i] = tostring(i) .. i
end
table.sort(t)
`,
	"python": `lst = [str(i) + str(i*i) for i in range(100000)]
lst.sort()
`,
	"node": `let arr = Array.from({length: 100000}, (_, i) => '' + i + (i*i));
arr.sort();
`,
}

func writeFiles(tmpdir string, langs []string) (map[string]string, error) {
	paths := make(map[string]string)
	for _, lang := range langs {
		ext := map[string]string{
			"lua":    "lua",
			"python": "py",
			"node":   "js",
		}[lang]
		fname := fmt.Sprintf("task.%s", ext)
		path := filepath.Join(tmpdir, fname)
		if err := os.WriteFile(path, []byte(codeMap[lang]), 0644); err != nil {
			return nil, err
		}
		paths[lang] = path
	}
	return paths, nil
}

func compileCpp(path string, v Verbosity) (string, error) {
	dir := filepath.Dir(path)
	exe := filepath.Join(dir, "task_cpp.exe")
	cmd := exec.Command("g++", "-O0", path, "-o", exe)
	if v == VerbosityHeavy {
		fmt.Printf("[DEBUG] gcc compile: %v\n", cmd.Args)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return exe, cmd.Run()
}

func compileGoFile(path string, v Verbosity) (string, error) {
	dir := filepath.Dir(path)
	exe := filepath.Join(dir, "task_go.exe")
	cmd := exec.Command("go", "build", "-o", exe, path)
	if v == VerbosityHeavy {
		fmt.Printf("[DEBUG] go build: %v\n", cmd.Args)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return exe, cmd.Run()
}

func compileRust(path string, v Verbosity) (string, error) {
	dir := filepath.Dir(path)
	exe := filepath.Join(dir, "task_rust.exe")
	cmd := exec.Command("rustc", "-C", "opt-level=0", path, "-o", exe)
	if v == VerbosityHeavy {
		fmt.Printf("[DEBUG] rustc compile: %v\n", cmd.Args)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return exe, cmd.Run()
}

func writeAndCompileExtra(tmpdir, chaos string, v Verbosity) (map[string]string, error) {
	extraCodes := map[string]struct {
		code string
		comp func(string, Verbosity) (string, error)
	}{
		"cpp": {
			code: `#include <iostream>
#include <vector>
#include <string>
#include <algorithm>
#include <sstream>
int main() {
    std::vector<std::string> v;
    v.reserve(100000);
    for (int i = 0; i < 100000; ++i) {
        std::ostringstream oss;
        oss << i << i*i;
        v.push_back(oss.str());
    }
    std::sort(v.begin(), v.end());
    return 0;
}
`,
			comp: compileCpp,
		},
		"go": {
			code: `package main
import (
    "sort"
    "strconv"
)
func main() {
    s := make([]string, 100000)
    for i := 0; i < 100000; i++ {
        s[i] = strconv.Itoa(i) + strconv.Itoa(i*i)
    }
    sort.Strings(s)
}
`,
			comp: compileGoFile,
		},
		"rust": {
			code: `fn main() {
    let mut v: Vec<String> = (0u64..100_000)
        .map(|i| format!("{}{}", i, i * i))
        .collect();
    v.sort();
}
`,
			comp: compileRust,
		},
	}

	langs := []string{"go"}
	if chaos == "high" {
		langs = []string{"go", "cpp", "rust"}
	}

	result := make(map[string]string)
	for _, lang := range langs {
		ext := map[string]string{"cpp": "cpp", "go": "go", "rust": "rs"}[lang]
		path := filepath.Join(tmpdir, fmt.Sprintf("task.%s", ext))
		if err := os.WriteFile(path, []byte(extraCodes[lang].code), 0644); err != nil {
			return nil, err
		}
		exe, err := extraCodes[lang].comp(path, v)
		if err != nil {
			return nil, err
		}
		result[lang] = exe
	}
	return result, nil
}

func timeRun(cmdArgs []string, v Verbosity) (int64, error) {
	if v == VerbosityHeavy {
		fmt.Printf("[DEBUG] Running: %v\n", cmdArgs)
	}
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	if v == VerbosityHeavy {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	start := time.Now()
	err := cmd.Run()
	return time.Since(start).Nanoseconds(), err
}

func main() {
	verbosity, queue, chaos, seedVal := parseFlags()
	preflightLangCheck(verbosity)

	if verbosity >= VerbosityLite {
		fmt.Printf("PTRSG %s\n", version)
		fmt.Printf("Using chaos=%s, queue=%v\n", chaos, queue)
	}

	tmpdir, err := os.MkdirTemp("", "prandom_")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpdir)

	if verbosity >= VerbosityLite {
		fmt.Printf("Preparing files in %s...\n", tmpdir)
	}

	langs := []string{"lua", "python", "node"}
	paths, err := writeFiles(tmpdir, langs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	extra, err := writeAndCompileExtra(tmpdir, chaos, verbosity)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	procMap := make(map[string][]string)
	for lang, p := range paths {
		procMap[lang] = []string{lang, p}
	}
	for lang, exe := range extra {
		procMap[lang] = []string{exe}
	}

	timings := make(map[string]int64)
	if queue {
		for lang, cmdArgs := range procMap {
			if verbosity >= VerbosityLite {
				fmt.Printf("Running %s...\n", lang)
			}
			t, err := timeRun(cmdArgs, verbosity)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			timings[lang] = t
		}
	} else {
		var wg2 sync.WaitGroup
		var mu2 sync.Mutex
		for lang, cmdArgs := range procMap {
			wg2.Add(1)
			go func(l string, args []string) {
				defer wg2.Done()
				t, err := timeRun(args, verbosity)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				mu2.Lock()
				timings[l] = t
				mu2.Unlock()
			}(lang, cmdArgs)
		}
		wg2.Wait()
	}

	if verbosity >= VerbosityLite {
		fmt.Println("Timings (ns):")
		keys := make([]string, 0, len(timings))
		for k := range timings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s: %d\n", k, timings[k])
		}
	}

	buf := new(bytes.Buffer)
	for _, t := range timings {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(t))
		buf.Write(b[:])
	}

	hash := blake2b.Sum512(buf.Bytes())

	if verbosity == VerbosityHeavy {
		fmt.Printf("[DEBUG] Full Blake2b: %x\n", hash)
	}

	byteLen := (seedVal + 7) / 8
	raw := hash[:byteLen]
	if seedVal%8 != 0 {
		raw[0] >>= (8 - (seedVal % 8))
	}

	seedInt := new(big.Int).SetBytes(raw)
	fmt.Printf("Seed generated (%d-bit): %s\n", seedVal, seedInt)
	_ = rand.New(rand.NewSource(seedInt.Int64()))
}

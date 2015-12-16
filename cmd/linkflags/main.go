package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const versionPackage = "github.com/gravitational/version"

// pkg is the path to the package the tool will create linker flags for.
var pkg = flag.String("pkg", "", "root package path")

// semverPattern defines a regexp pattern to modify the results of `git describe` to be semver-complaint.
var semverPattern = regexp.MustCompile(`(.+)-([0-9]{1,})-g([0-9a-f]{14})$`)

// goVersionPattern defines a regexp pattern to parse versions of the `go tool`.
var goVersionPattern = regexp.MustCompile(`go([1-9])\.(\d+)(?:.\d+)*`)

func main() {
	log.SetFlags(0)
	flag.Parse()
	if *pkg == "" {
		log.Fatalln("pkg required")
	}

	goVersion, err := goToolVersion()
	if err != nil {
		log.Fatalf("failed to determine go tool version: %v\n", err)
	}

	git := newGit(*pkg)
	commitID, err := git.commitID()
	if err != nil {
		log.Fatalf("failed to obtain git commit ID: %v\n", err)
	}
	treeState, err := git.treeState()
	if err != nil {
		log.Fatalf("failed to determine git tree state: %v\n", err)
	}
	// FIXME: empty the version only on exit code error
	version, err := git.version(string(commitID))
	if err != nil {
		version = ""
	}
	if version != "" {
		version = semverify(version)
		if treeState == dirty {
			version = version + "-dirty"
		}
	}

	var linkFlags []string
	linkFlag := func(key, value string) string {
		if goVersion <= 14 {
			return fmt.Sprintf("-X %s.%s %s", versionPackage, key, value)
		} else {
			return fmt.Sprintf("-X %s.%s=%s", versionPackage, key, value)
		}
	}

	// Determine the values of version-related variables as commands to the go linker.
	if commitID != "" {
		linkFlags = append(linkFlags, linkFlag("gitCommit", commitID))
		linkFlags = append(linkFlags, linkFlag("gitTreeState", string(treeState)))
	}
	if version != "" {
		linkFlags = append(linkFlags, linkFlag("version", version))
	}

	log.Printf("%s", strings.Join(linkFlags, " "))
}

// toolError is a tool execution error.
type toolError struct {
	tool   string
	output []byte
	err    error
}

func (r *toolError) Error() string {
	return fmt.Sprintf("error executing `%s`: %v (%s)", r.tool, r.err, r.output)
}

// goToolVersion determines the version of the `go tool`.
func goToolVersion() (toolVersion, error) {
	out, err := exec.Command("go", "version").CombinedOutput()
	if err != nil {
		return toolVersionUnknown, &toolError{
			tool:   "go",
			output: out,
			err:    err,
		}
	}
	build := bytes.Split(out, []byte(" "))
	if len(build) > 2 {
		return parseToolVersion(string(build[2])), nil
	}
	return toolVersionUnknown, nil
}

// parseToolVersion translates a string version of the form 'go1.4.3' to a numeric value 14.
func parseToolVersion(version string) toolVersion {
	match := goVersionPattern.FindStringSubmatch(version)
	if len(match) > 2 {
		// After a successful match, match[1] and match[2] are integers
		major := mustAtoi(match[1])
		minor := mustAtoi(match[2])
		return toolVersion(major*10 + minor)
	}
	return toolVersionUnknown
}

func newGit(pkg string) *git {
	args := []string{"--work-tree", pkg, "--git-dir", filepath.Join(pkg, ".git")}
	return &git{cmd: "git", args: args}
}

type git struct {
	cmd  string
	args []string
}

type treeState string

const (
	clean treeState = "clean"
	dirty           = "dirty"
)

type toolVersion int

const toolVersionUnknown toolVersion = 0

func (r *git) commitID() (string, error) {
	return r.exec("rev-parse", "HEAD^{commit}")
}

func (r *git) treeState() (treeState, error) {
	out, err := r.exec("status", "--porcelain")
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return clean, nil
	}
	return dirty, nil
}

func (r *git) version(commitID string) (string, error) {
	return r.exec("describe", "--tags", "--abbrev=14", commitID+"^{commit}")
}

// exec executes a given git command specified with args and returns the output
// with whitespace trimmed.
func (r *git) exec(args ...string) (string, error) {
	opts := append([]string{}, r.args...)
	opts = append(opts, args...)
	out, err := exec.Command(r.cmd, opts...).CombinedOutput()
	if err == nil {
		out = bytes.TrimSpace(out)
	}
	if err != nil {
		err = &toolError{
			tool:   r.cmd,
			output: out,
			err:    err,
		}
	}
	return string(out), err
}

// semverify transforms the output of `git describe` to be semver-complaint.
func semverify(version string) string {
	var result []byte
	match := semverPattern.FindStringSubmatchIndex(version)
	if match != nil {
		return string(semverPattern.ExpandString(result, "$1.$2+$3", string(version), match))
	}
	return version
}

// mustAtoi converts value to an integer.
// It panics if the value does not represent a valid integer.
func mustAtoi(value string) int {
	result, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return result
}

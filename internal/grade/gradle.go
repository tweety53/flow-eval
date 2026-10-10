package grade

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// _sourceSet splits a Gradle source path into its module directory and source set.
var _sourceSet = regexp.MustCompile(`^(?:(.*)/)?src/(test|commonTest|desktopTest)/`)

// _className is a test file's base name that is safe to quote as a --tests selector.
var _className = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)

// _moduleName is a module directory's base name that is safe to put unquoted in a Gradle task path.
var _moduleName = regexp.MustCompile(`^[\w.-]+$`)

// TestRun is one Gradle module's test task and the test classes to run in it by name.
type TestRun struct {
	// Dir is the module's directory, empty at the repository root.
	Dir string
	// Task is the module's test task: :<directory name>:<source set>, or the bare source set at
	// the root, commonTest running under desktopTest.
	Task    string
	Classes []string
}

// Command runs r's classes by name: ./gradlew <task> --tests '<Class>' ….
func (r TestRun) Command() string {
	cmd := "./gradlew " + r.Task
	for _, class := range r.Classes {
		cmd += " --tests '" + class + "'"
	}
	return cmd
}

// TestRuns groups the Kotlin and Java files among paths under a module's src/test/,
// src/commonTest/ or src/desktopTest/ by module and test task, in the order each first appears.
// Any other path, or one whose file name is no class name or whose module directory's base name is
// no Gradle project name, is skipped.
func TestRuns(paths []string) []TestRun {
	var runs []TestRun
	for _, p := range paths {
		m := _sourceSet.FindStringSubmatch(p)
		ext := path.Ext(p)
		class := strings.TrimSuffix(path.Base(p), ext)
		if m == nil || (ext != ".kt" && ext != ".java") || !_className.MatchString(class) {
			continue
		}
		task := m[2]
		if task == "commonTest" {
			task = "desktopTest"
		}
		if m[1] != "" {
			if !_moduleName.MatchString(path.Base(m[1])) {
				continue
			}
			task = ":" + path.Base(m[1]) + ":" + task
		}
		i := slices.IndexFunc(runs, func(r TestRun) bool { return r.Dir == m[1] && r.Task == task })
		if i < 0 {
			i = len(runs)
			runs = append(runs, TestRun{Dir: m[1], Task: task})
		}
		runs[i].Classes = append(runs[i].Classes, class)
	}
	return runs
}

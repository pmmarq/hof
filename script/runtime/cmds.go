// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hofstadter-io/hof/lib/gotils/intern/textutil"
	"github.com/hofstadter-io/hof/lib/gotils/txtar"
)

// scriptCmds are the script command implementations.
// Keep list and the implementations below sorted by name.
//
// NOTE: If you make changes here, update doc.go.
//
var scriptCmds = map[string]func(*Script, int, []string){
	"call":    (*Script).cmdCall,
	"cd":      (*Script).cmdCd,
	"chmod":   (*Script).cmdChmod,
	"cmp":     (*Script).cmdCmp,
	"cmpenv":  (*Script).cmdCmpenv,
	"cp":      (*Script).cmdCp,
	"env":     (*Script).cmdEnv,
	"exec":    (*Script).cmdExec,
	"exists":  (*Script).cmdExists,
	"grep":    (*Script).cmdGrep,
	"http":    (*Script).cmdHttp,
	"mkdir":   (*Script).cmdMkdir,
	"regexp":  (*Script).cmdRegexp,
	"rm":      (*Script).cmdRm,
	"unquote": (*Script).cmdUnquote,
	"sed":     (*Script).cmdSed,
	"skip":    (*Script).cmdSkip,
	"stdin":   (*Script).cmdStdin,
	"stderr":  (*Script).cmdStderr,
	"stdout":  (*Script).cmdStdout,
	"status":  (*Script).cmdStatus,
	"stop":    (*Script).cmdStop,
	"symlink": (*Script).cmdSymlink,
	"wait":    (*Script).cmdWait,
}


// http	makes an http call.
func (ts *Script) cmdHttp(neg int, args []string) {
	if len(args) < 1 {
		ts.Fatalf("usage: http function [args...]")
	}

	var err error
	ts.stdout, ts.stderr, ts.status, err = ts.http(args)
	if ts.stdout != "" {
		fmt.Fprintf(&ts.log, "[stdout]\n%s", ts.stdout)
	}
	if ts.stderr != "" {
		fmt.Fprintf(&ts.log, "[stderr]\n%s", ts.stderr)
	}
	if err == nil && neg > 0 {
		ts.Fatalf("unexpected http success")
	}

	if err != nil {
		fmt.Fprintf(&ts.log, "[%v]\n", err)
		if ts.ctxt.Err() != nil {
			ts.Fatalf("test timed out while making http request")
		} else if neg > 0 {
			ts.Fatalf("unexpected http failure")
		}
	}
}

// call runs the given function.
func (ts *Script) cmdCall(neg int, args []string) {
	if len(args) < 1 {
		ts.Fatalf("usage: call function [args...]")
	}

	var err error
	ts.stdout, ts.stderr, err = ts.call(args[0], args[1:]...)
	if ts.stdout != "" {
		fmt.Fprintf(&ts.log, "[stdout]\n%s", ts.stdout)
	}
	if ts.stderr != "" {
		fmt.Fprintf(&ts.log, "[stderr]\n%s", ts.stderr)
	}
	if err == nil && neg > 0 {
		ts.Fatalf("unexpected command success")
	}

	if err != nil {
		fmt.Fprintf(&ts.log, "[%v]\n", err)
		if ts.ctxt.Err() != nil {
			ts.Fatalf("test timed out while running command")
		} else if neg > 0 {
			ts.Fatalf("unexpected command failure")
		}
	}
}


// cd changes to a different directory.
func (ts *Script) cmdCd(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? cd")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: cd dir")
	}

	dir := args[0]
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(ts.cd, dir)
	}
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		ts.Fatalf("directory %s does not exist", dir)
	}
	ts.Check(err)
	if !info.IsDir() {
		ts.Fatalf("%s is not a directory", dir)
	}
	ts.cd = dir
	ts.Logf("%s\n", ts.cd)
}

func (ts *Script) cmdChmod(neg int, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: chmod mode file")
	}
	mode, err := strconv.ParseInt(args[0], 8, 32)
	if err != nil {
		ts.Fatalf("bad file mode %q: %v", args[0], err)
	}
	if mode > 0777 {
		ts.Fatalf("unsupported file mode %.3o", mode)
	}
	err = os.Chmod(ts.MkAbs(args[1]), os.FileMode(mode))
	if neg > 0 {
		if err == nil {
			ts.Fatalf("unexpected chmod success")
		}
		return
	}
	if err != nil {
		ts.Fatalf("unexpected chmod failure: %v", err)
	}
}

// cmp compares two files.
func (ts *Script) cmdCmp(neg int, args []string) {
	if neg != 0 {
		// It would be strange to say "this file can have any content except this precise byte sequence".
		ts.Fatalf("unsupported: !? cmp")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: cmp file1 file2")
	}

	ts.doCmdCmp(args, false)
}

// cmpenv compares two files with environment variable substitution.
func (ts *Script) cmdCmpenv(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? cmpenv")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: cmpenv file1 file2")
	}
	ts.doCmdCmp(args, true)
}

func (ts *Script) doCmdCmp(args []string, env bool) {
	name1, name2 := args[0], args[1]
	text1 := ts.ReadFile(name1)

	absName2 := ts.MkAbs(name2)
	data, err := ioutil.ReadFile(absName2)
	ts.Check(err)
	text2 := string(data)
	if env {
		text2 = ts.expand(text2)
	}
	if text1 == text2 {
		return
	}
	if ts.params.UpdateScripts && !env && (args[0] == "stdout" || args[0] == "stderr") {
		if scriptFile, ok := ts.scriptFiles[absName2]; ok {
			ts.scriptUpdates[scriptFile] = text1
			return
		}
		// The file being compared against isn't in the txtar archive, so don't
		// update the script.
	}

	ts.Logf("[diff -%s +%s]\n%s\n", name1, name2, textutil.Diff(text1, text2))
	ts.Fatalf("%s and %s differ", name1, name2)
}

// cp copies files, maybe eventually directories.
func (ts *Script) cmdCp(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? cp")
	}
	if len(args) < 2 {
		ts.Fatalf("usage: cp src... dst")
	}

	dst := ts.MkAbs(args[len(args)-1])
	info, err := os.Stat(dst)
	dstDir := err == nil && info.IsDir()
	if len(args) > 2 && !dstDir {
		ts.Fatalf("cp: destination %s is not a directory", dst)
	}

	for _, arg := range args[:len(args)-1] {
		var (
			src  string
			data []byte
			mode os.FileMode
		)
		switch arg {
		case "stdout":
			src = arg
			data = []byte(ts.stdout)
			mode = 0666
		case "stderr":
			src = arg
			data = []byte(ts.stderr)
			mode = 0666
		default:
			src = ts.MkAbs(arg)
			info, err := os.Stat(src)
			ts.Check(err)
			mode = info.Mode() & 0777
			data, err = ioutil.ReadFile(src)
			ts.Check(err)
		}
		targ := dst
		if dstDir {
			targ = filepath.Join(dst, filepath.Base(src))
		}
		ts.Check(ioutil.WriteFile(targ, data, mode))
	}
}

// env displays or adds to the environment.
func (ts *Script) cmdEnv(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? env")
	}
	if len(args) == 0 {
		printed := make(map[string]bool) // env list can have duplicates; only print effective value (from envMap) once
		for _, kv := range ts.env {
			k := envvarname(kv[:strings.Index(kv, "=")])
			if !printed[k] {
				printed[k] = true
				ts.Logf("%s=%s\n", k, ts.envMap[k])
			}
		}
		return
	}
	for _, env := range args {
		i := strings.Index(env, "=")
		if i < 0 {
			// Display value instead of setting it.
			ts.Logf("%s=%s\n", env, ts.Getenv(env))
			continue
		}
		k, v := env[:i], env[i+1:]
		if v[0] == '@' {
			fname := v[1:] // for error messages
			if fname == "stdout" {
				v = ts.stdout
			} else if fname == "stderr" {
				v = ts.stderr
			} else {
				data, err := ioutil.ReadFile(ts.MkAbs(fname))
				ts.Check(err)
				v = string(data)
			}
		}
		ts.Setenv(k,v)
	}
}

// exec runs the given command.
func (ts *Script) cmdExec(neg int, args []string) {

	if len(args) < 1 || (len(args) == 1 && args[0] == "&") {
		ts.Fatalf("usage: exec program [args...] [&]")
	}

	var err error
	if len(args) > 0 && args[len(args)-1] == "&" {
		var cmd *exec.Cmd
		cmd, err = ts.execBackground(args[0], args[1:len(args)-1]...)
		if err == nil {
			wait := make(chan struct{})
			go func() {
				werr := ctxWait(ts.ctxt, cmd)
				close(wait)
				ts.status = cmd.ProcessState.ExitCode()
				err = werr
			}()
			ts.background = append(ts.background, backgroundCmd{cmd, wait, neg})
		}
		ts.stdout, ts.stderr = "", ""
	} else {
		ts.stdout, ts.stderr, err = ts.exec(args[0], args[1:]...)
		if ts.stdout != "" {
			fmt.Fprintf(&ts.log, "[stdout]\n%s", ts.stdout)
		}
		if ts.stderr != "" {
			fmt.Fprintf(&ts.log, "[stderr]\n%s", ts.stderr)
		}
		if err == nil && neg > 0 {
			ts.Fatalf("unexpected command success")
		}
	}

	if err != nil {
		fmt.Fprintf(&ts.log, "[%v]\n", err)
		if ts.ctxt.Err() != nil {
			ts.Fatalf("test timed out while running command")
		} else if neg == 0 {
			ts.Fatalf("unexpected exec command failure")
		}
	}
}

// exists checks that the list of files exists.
func (ts *Script) cmdExists(neg int, args []string) {
	var readonly bool
	if len(args) > 0 && args[0] == "-readonly" {
		readonly = true
		args = args[1:]
	}
	if len(args) == 0 {
		ts.Fatalf("usage: exists [-readonly] file...")
	}

	for _, file := range args {
		file = ts.MkAbs(file)
		info, err := os.Stat(file)
		if err == nil && neg > 0 {
			what := "file"
			if info.IsDir() {
				what = "directory"
			}
			ts.Fatalf("%s %s unexpectedly exists", what, file)
		}
		if err != nil && neg == 0 {
			ts.Fatalf("%s does not exist", file)
		}
		if err == nil && neg == 0 && readonly && info.Mode()&0222 != 0 {
			ts.Fatalf("%s exists but is writable", file)
		}
	}
}

// mkdir creates directories.
func (ts *Script) cmdMkdir(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? mkdir")
	}
	if len(args) < 1 {
		ts.Fatalf("usage: mkdir dir...")
	}
	for _, arg := range args {
		ts.Check(os.MkdirAll(ts.MkAbs(arg), 0777))
	}
}

// unquote unquotes files.
func (ts *Script) cmdUnquote(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? unquote")
	}
	for _, arg := range args {
		file := ts.MkAbs(arg)
		data, err := ioutil.ReadFile(file)
		ts.Check(err)
		data, err = txtar.Unquote(data)
		ts.Check(err)
		err = ioutil.WriteFile(file, data, 0666)
		ts.Check(err)
	}
}

// rm removes files or directories.
func (ts *Script) cmdRm(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? rm")
	}
	if len(args) < 1 {
		ts.Fatalf("usage: rm file...")
	}
	for _, arg := range args {
		file := ts.MkAbs(arg)
		removeAll(file)              // does chmod and then attempts rm
		ts.Check(os.RemoveAll(file)) // report error
	}
}

// skip marks the test skipped.
func (ts *Script) cmdSkip(neg int, args []string) {
	if neg != 0{
		ts.Fatalf("unsupported: !? skip")
	}

	if len(args) > 1 {
		ts.Fatalf("usage: skip [msg]")
	}

	// Before we mark the test as skipped, shut down any background processes and
	// make sure they have returned the correct status.
	for _, bg := range ts.background {
		interruptProcess(bg.cmd.Process)
	}
	ts.cmdWait(0, nil)

	if len(args) == 1 {
		ts.t.Skip(args[0])
	}
	ts.t.Skip()
}

func (ts *Script) cmdStdin(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? stdin")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: stdin filename")
	}
	ts.stdin = ts.ReadFile(args[0])
}

// stdout checks that the last go command standard output matches a regexp.
func (ts *Script) cmdStdout(neg int, args []string) {
	scriptMatch(ts, neg, args, ts.stdout, "stdout")
}

// stderr checks that the last go command standard output matches a regexp.
func (ts *Script) cmdStderr(neg int, args []string) {
	scriptMatch(ts, neg, args, ts.stderr, "stderr")
}

// status checks the exit or status code from the last exec or http call
func (ts *Script) cmdStatus(neg int, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: status <int>")
	}

	// Don't care
	if neg < 0 {
		return
	}

	// Check arg
	code, err := strconv.Atoi(args[0])
	if err != nil {
		ts.Fatalf("error: %v\nusage: stdin <int>", err)
	}

	// wanted different but got samd
	if neg > 0 && ts.status == code {
		ts.Fatalf("unexpected status match: %d", code)
	}

	if neg == 0 && ts.status != code {
		ts.Fatalf("unexpected status mismatch:  wated: %d  got %d", code, ts.status)
	}

}

// regexp checks that file content matches a regexp.
// it accepts Go regexp syntax.
func (ts *Script) cmdRegexp(neg int, args []string) {
	scriptMatch(ts, neg, args, "", "regexp")
}

// regexp checks that file content matches a regexp.
// it accepts Go regexp syntax and returns the matches
func (ts *Script) cmdGrep(neg int, args []string) {
	scriptMatch(ts, neg, args, "", "grep")
}

// sed finds and replaces in text content
// it accepts Go regexp syntax and returns the replaced content
func (ts *Script) cmdSed(neg int, args []string) {
	scriptMatch(ts, neg, args, "", "sed")
}

// stop stops execution of the test (marking it passed).
func (ts *Script) cmdStop(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? stop")
	}
	if len(args) > 1 {
		ts.Fatalf("usage: stop [msg]")
	}
	if len(args) == 1 {
		ts.Logf("stop: %s\n", args[0])
	} else {
		ts.Logf("stop\n")
	}
	ts.stopped = true
}

// symlink creates a symbolic link.
func (ts *Script) cmdSymlink(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? symlink")
	}
	if len(args) != 3 || args[1] != "->" {
		ts.Fatalf("usage: symlink file -> target")
	}
	// Note that the link target args[2] is not interpreted with MkAbs:
	// it will be interpreted relative to the directory file is in.
	ts.Check(os.Symlink(args[2], ts.MkAbs(args[0])))
}

// Tait waits for background commands to exit, setting stderr and stdout to their result.
func (ts *Script) cmdWait(neg int, args []string) {
	if neg != 0 {
		ts.Fatalf("unsupported: !? wait")
	}
	if len(args) > 0 {
		ts.Fatalf("usage: wait")
	}

	var stdouts, stderrs []string
	for _, bg := range ts.background {
		<-bg.wait

		args := append([]string{filepath.Base(bg.cmd.Args[0])}, bg.cmd.Args[1:]...)
		fmt.Fprintf(&ts.log, "[background] %s: %v\n", strings.Join(args, " "), bg.cmd.ProcessState)

		cmdStdout := bg.cmd.Stdout.(*strings.Builder).String()
		if cmdStdout != "" {
			fmt.Fprintf(&ts.log, "[stdout]\n%s", cmdStdout)
			stdouts = append(stdouts, cmdStdout)
		}

		cmdStderr := bg.cmd.Stderr.(*strings.Builder).String()
		if cmdStderr != "" {
			fmt.Fprintf(&ts.log, "[stderr]\n%s", cmdStderr)
			stderrs = append(stderrs, cmdStderr)
		}

		if bg.cmd.ProcessState.Success() {
			if bg.neg > 0 {
				ts.Fatalf("unexpected command success")
			}
		} else {
			if ts.ctxt.Err() != nil {
				ts.Fatalf("test timed out while running command")
			} else if bg.neg == 0 {
				ts.Fatalf("unexpected command failure")
			}
		}
	}

	ts.stdout = strings.Join(stdouts, "")
	ts.stderr = strings.Join(stderrs, "")
	ts.background = nil
}

// scriptMatch implements both stdout and stderr.
func scriptMatch(ts *Script, neg int, args []string, text, name string) {
	n := 0
	if len(args) >= 1 && strings.HasPrefix(args[0], "-count=") {
		if neg != 0 {
			ts.Fatalf("cannot use -count= with negated match")
		}
		var err error
		n, err = strconv.Atoi(args[0][len("-count="):])
		if err != nil {
			ts.Fatalf("bad -count=: %v", err)
		}
		if n < 1 {
			ts.Fatalf("bad -count=: must be at least 1")
		}
		args = args[1:]
	}

	isRegexp := name == "regexp"
	isGrep := name == "grep"
	isSed := name == "sed"

	extraUsage := ""
	want := 1
	if isRegexp || isGrep {
		extraUsage = " file"
		want = 2
	}
	if isSed {
		extraUsage = " replace file"
		want = 3
	}
	if len(args) != want {
		ts.Fatalf("usage: %s [-count=N] 'pattern'%s", name, extraUsage)
	}

	pattern := args[0]
	switch pattern {
	case "stdout":
		pattern = ts.stdout
	case "stderr":
		pattern = ts.stderr

	default:
		if pattern[0] == '@' {
			fname := pattern[1:] // for error messages
			data, err := ioutil.ReadFile(ts.MkAbs(fname))
			ts.Check(err)
			pattern = string(data)
		}
	}
	re, err := regexp.Compile(`(?m)` + pattern)
	ts.Check(err)


	if isRegexp || isGrep {
		content := args[1]
		switch  content {
		case "stdout", "$WORK/stdout":
			text = ts.stdout
		case "stderr", "$WORK/stderr":
			text = ts.stderr

		default:
			name = args[1] // for error messages
			data, err := ioutil.ReadFile(ts.MkAbs(args[1]))
			ts.Check(err)
			text = string(data)
		}
	}
	replace := ""
	if isSed {
		replace = args[1]
		switch  replace {
		case "stdout", "$WORK/stdout":
			text = ts.stdout
		case "stderr", "$WORK/stderr":
			text = ts.stderr

		default:
			if replace[0] == '@' {
				fname := replace[1:] // for error messages
				data, err := ioutil.ReadFile(ts.MkAbs(fname))
				ts.Check(err)
				replace = string(data)
			}
		}
		content := args[2]
		switch  content {
		case "stdout", "$WORK/stdout":
			text = ts.stdout
		case "stderr", "$WORK/stderr":
			text = ts.stderr

		default:
			if content[0] == '@' {
				fname := content[1:] // for error messages
				data, err := ioutil.ReadFile(ts.MkAbs(fname))
				ts.Check(err)
				content = string(data)
			}
		}
	}

	if neg > 0 {
		if re.MatchString(text) {
			if isGrep {
				ts.Logf("[%s]\n%s\n", name, text)
			}
			ts.Fatalf("unexpected match for %#q found in %s: %s", pattern, name, re.FindString(text))
		}

		if isGrep {
			c := -1
			if n > 0 {
				c = n
			}
			matches := re.FindAllString(text, c)
			if c > 0 && len(matches) > c {
				matches = matches[:c]
			}
			ts.stdout = strings.Join(matches, "\n")
		}
		if isSed {
			ts.stdout = re.ReplaceAllString(text, replace)
		}
	} else {
		if isGrep || isSed {
			ts.Fatalf("%s does not support status checking", name)
		}
		if !re.MatchString(text) {
			if isGrep {
				ts.Logf("[%s]\n%s\n", name, text)
			}
			ts.Fatalf("no match for %#q found in %s", pattern, name)
		}
		if n > 0 {
			count := len(re.FindAllString(text, -1))
			if count != n {
				if isGrep {
					ts.Logf("[%s]\n%s\n", name, text)
				}
				ts.Fatalf("have %d matches for %#q, want %d", count, pattern, n)
			}
		}
	}
}

// cmdExpectExec starts an expect session.
func (ts *Script) cmdExpectExec(neg int, args []string) {
	fmt.Println("expect-exec:", neg, args)

}

// cmdExpectRepl is the main expect function which looks for a string match and sends something in reply.
func (ts *Script) cmdExpectRepl(neg int, args []string) {
	fmt.Println("expect-repl:", neg, args)

}

// cmdExpectDone finishes and cleans up an expect session.
func (ts *Script) cmdExpectDone(neg int, args []string) {
	fmt.Println("expect-done:", neg, args)
	/*
	if len(args) < 1 || (len(args) == 1 && args[0] == "&") {
		ts.Fatalf("usage: exec program [args...] [&]")
	}

	var err error
	if len(args) > 0 && args[len(args)-1] == "&" {
		var cmd *exec.Cmd
		cmd, err = ts.execBackground(args[0], args[1:len(args)-1]...)
		if err == nil {
			wait := make(chan struct{})
			go func() {
				werr := ctxWait(ts.ctxt, cmd)
				close(wait)
				ts.status = cmd.ProcessState.ExitCode()
				err = werr
			}()
			ts.background = append(ts.background, backgroundCmd{cmd, wait, neg})
		}
		ts.stdout, ts.stderr = "", ""
	} else {
		ts.stdout, ts.stderr, err = ts.exec(args[0], args[1:]...)
		if ts.stdout != "" {
			fmt.Fprintf(&ts.log, "[stdout]\n%s", ts.stdout)
		}
		if ts.stderr != "" {
			fmt.Fprintf(&ts.log, "[stderr]\n%s", ts.stderr)
		}
		if err == nil && neg > 0 {
			ts.Fatalf("unexpected command success")
		}
	}

	if err != nil {
		fmt.Fprintf(&ts.log, "[%v]\n", err)
		if ts.ctxt.Err() != nil {
			ts.Fatalf("test timed out while running command")
		} else if neg == 0 {
			ts.Fatalf("unexpected exec command failure")
		}
	}
	*/


}

package sandbox

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeDocker records every docker argv and fails each call fail reports true for.
type fakeDocker struct {
	calls     [][]string
	fail      func(args []string) bool
	logs      string     // what docker logs prints, on stderr as docker relays a container's stderr
	cancelled [][]string // calls made with an already-cancelled context
}

func (f *fakeDocker) Run(ctx context.Context, _ io.Reader, args ...string) (Result, error) {
	f.calls = append(f.calls, args)
	if ctx.Err() != nil {
		f.cancelled = append(f.cancelled, args)
	}
	if f.fail != nil && f.fail(args) {
		return Result{Exit: 1, Stderr: "boom"}, nil
	}
	if args[0] == "logs" {
		return Result{Stderr: f.logs}, nil
	}
	if args[0] == "run" {
		return Result{Stdout: "c0ffee\n"}, nil // docker run -d prints the new container's id
	}
	return Result{}, nil
}

func failOn(arg string) func([]string) bool {
	return func(args []string) bool { return slices.Contains(args, arg) }
}

func isDindProbe(args []string) bool {
	return slices.Equal(args, []string{"exec", "fe-t1-dind", "docker", "version"})
}

// call returns the first recorded argv starting with prefix and holding every one of with.
func (f *fakeDocker) call(t *testing.T, prefix []string, with ...string) []string {
	t.Helper()
	for _, c := range f.calls {
		if len(c) >= len(prefix) && slices.Equal(c[:len(prefix)], prefix) && containsAll(c, with) {
			return c
		}
	}
	t.Fatalf("no docker %v call holding %v; calls:\n%s", prefix, with, f)
	return nil
}

func (f *fakeDocker) String() string {
	var b strings.Builder
	for _, c := range f.calls {
		b.WriteString("docker " + strings.Join(c, " ") + "\n")
	}
	return b.String()
}

func containsAll(argv, want []string) bool {
	for _, w := range want {
		if !slices.Contains(argv, w) {
			return false
		}
	}
	return true
}

// _harnessMount is a harness's own volume, as muse-code mounts its login.
const _harnessMount = "type=volume,src=fe-harness,dst=/fe,volume-subpath=config"

func spec(needsDocker bool) TrialSpec {
	return TrialSpec{
		ID: "t1", Repo: "gymie", NeedsDocker: needsDocker, Credentials: []string{"FE_TEST_TOKEN"},
		Mounts:      []string{_harnessMount},
		RunnerImage: "flow-eval-runner:test", ProxyImage: "flow-eval-proxy:test",
	}
}

func start(t *testing.T, needsDocker bool) *fakeDocker {
	t.Helper()
	t.Setenv("FE_TEST_TOKEN", "s3cret-value")
	f := &fakeDocker{}
	tr, err := Start(context.Background(), f, spec(needsDocker))
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestTrialNetworkIsInternal(t *testing.T) {
	f := start(t, true)
	f.call(t, []string{"network", "create"}, "--internal", "fe-t1")
	for _, c := range f.calls {
		if slices.Equal(c[:2], []string{"network", "create"}) && !slices.Contains(c, "--internal") {
			t.Errorf("network created without --internal: %v", c)
		}
	}
	trial := f.call(t, []string{"run"}, "--name", "fe-t1-trial")
	if !containsAll(trial, []string{"--network", "fe-t1"}) {
		t.Errorf("trial container not on the internal network: %v", trial)
	}
	for _, c := range f.calls {
		if len(c) >= 2 && c[0] == "network" && c[1] == "connect" && slices.Contains(c, "fe-t1-trial") {
			t.Errorf("trial container connected to a second network: %v", c)
		}
	}
	// The proxy is the one container on both the internal network and the default bridge.
	f.call(t, []string{"create"}, "--name", "fe-t1-proxy", "--network", "bridge")
	f.call(t, []string{"network", "connect"}, "--alias", "proxy", "fe-t1", "fe-t1-proxy")
	trialEnv := envOf(trial)
	for _, kv := range []string{"HTTPS_PROXY=http://proxy:3128", "HTTP_PROXY=http://proxy:3128"} {
		if !slices.Contains(trialEnv, kv) {
			t.Errorf("trial env lacks %s: %v", kv, trialEnv)
		}
	}
}

func TestNoHostMounts(t *testing.T) {
	for _, needsDocker := range []bool{false, true} {
		f := start(t, needsDocker)
		for _, c := range f.calls {
			for i, a := range c {
				switch {
				case a == "-v" || a == "--volume" || strings.HasPrefix(a, "--volume=") || strings.HasPrefix(a, "-v="):
					t.Errorf("bind-style volume flag in %v", c)
				case a == "--mount" && (i+1 >= len(c) || !strings.HasPrefix(c[i+1], "type=volume,")):
					t.Errorf("--mount that is not a named volume in %v", c)
				case strings.Contains(a, "docker.sock"), strings.Contains(a, ".claude"), strings.Contains(a, ".codex"):
					t.Errorf("host path in %v", c)
				case strings.Contains(a, "s3cret-value"):
					t.Errorf("credential value on argv: %v", c)
				}
			}
		}
		trial := f.call(t, []string{"run"}, "--name", "fe-t1-trial")
		if !containsAll(trial, []string{"--mount", _harnessMount}) {
			t.Errorf("trial argv lacks the harness's mount: %v", trial)
		}
		if !containsAll(trial, []string{"-e", "FE_TEST_TOKEN"}) {
			t.Errorf("trial argv lacks the credential pass-through: %v", trial)
		}
		if slices.Contains(trial, "--privileged") {
			t.Errorf("trial container is privileged: %v", trial)
		}
	}
}

// The trial reads its repository's warmed Gradle volume but cannot write it, and Gradle's user home is
// the container's own: a build cache, daemon log or init script one trial leaves never reaches another.
func TestTrialGradleCacheIsReadOnlyAndPrivate(t *testing.T) {
	f := start(t, false)
	trial := f.call(t, []string{"run"}, "--name", "fe-t1-trial")
	i := slices.Index(trial, "--mount")
	if i < 0 || trial[i+1] != "type=volume,src=flow-eval-gradle-ro-gymie,dst=/gradle-ro,readonly" {
		t.Errorf("trial does not mount the gradle volume read-only at /gradle-ro: %v", trial)
	}
	if !slices.Contains(envOf(trial), "GRADLE_RO_DEP_CACHE=/gradle-ro/caches") {
		t.Errorf("trial env lacks the read-only dependency cache: %v", envOf(trial))
	}
	for _, e := range envOf(trial) {
		if strings.HasPrefix(e, "GRADLE_USER_HOME=") {
			t.Errorf("trial's Gradle user home is set to %q, want the container's own", e)
		}
	}
	seed := f.call(t, []string{"exec", "fe-t1-trial", "sh", "-c"})
	if !strings.Contains(seed[len(seed)-1], "cp -a /gradle-ro/wrapper /root/.gradle/") {
		t.Errorf("trial's own Gradle home is not seeded with the wrapper: %v", seed)
	}
}

// A visual trial never builds, so it gets no Gradle volume, no read-only cache and no wrapper copy.
func TestNoGradleVolumeWithoutRepo(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "x")
	f := &fakeDocker{}
	sp := spec(false)
	sp.Repo, sp.Mounts = "", nil
	tr, err := Start(t.Context(), f, sp)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if slices.Contains(c, "--mount") || slices.ContainsFunc(c, func(a string) bool { return strings.Contains(a, "gradle") }) {
			t.Errorf("gradle volume or cache for a trial with no repo: %v", c)
		}
	}
}

func TestDindOnlyWhenNeeded(t *testing.T) {
	without := start(t, false)
	for _, c := range without.calls {
		if slices.Contains(c, DindImage) || slices.Contains(c, "fe-t1-dind") {
			t.Errorf("dind referenced for a case without needsDocker: %v", c)
		}
	}
	for _, kv := range envOf(without.call(t, []string{"run"}, "--name", "fe-t1-trial")) {
		if strings.HasPrefix(kv, "DOCKER_HOST=") || strings.HasPrefix(kv, "TESTCONTAINERS_") {
			t.Errorf("trial env has %s without a dind sidecar", kv)
		}
	}

	with := start(t, true)
	dind := with.call(t, []string{"run"}, DindImage)
	if !containsAll(dind, []string{"--privileged", "--name", "fe-t1-dind", "--network", "fe-t1", "--network-alias", "dind", "DOCKER_TLS_CERTDIR="}) {
		t.Errorf("dind argv: %v", dind)
	}
	trialEnv := envOf(with.call(t, []string{"run"}, "--name", "fe-t1-trial"))
	for _, kv := range []string{"DOCKER_HOST=tcp://dind:2375", "TESTCONTAINERS_RYUK_DISABLED=true"} {
		if !slices.Contains(trialEnv, kv) {
			t.Errorf("trial env lacks %s: %v", kv, trialEnv)
		}
	}
}

func TestStopRemovesEverythingButTheVolume(t *testing.T) {
	f := start(t, true)
	f.call(t, []string{"rm", "-f", "--volumes"}, "fe-t1-trial", "fe-t1-dind", "fe-t1-proxy")
	f.call(t, []string{"network", "rm", "fe-t1"})
	for _, c := range f.calls {
		if c[0] == "volume" {
			t.Errorf("Stop touched a volume: %v", c)
		}
	}
}

func TestStopRunsWithCancelledContext(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "x")
	f := &fakeDocker{}
	ctx, cancel := context.WithCancel(context.Background())
	tr, err := Start(ctx, f, spec(true))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := tr.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	f.call(t, []string{"rm", "-f"}, "fe-t1-trial")
	if len(f.cancelled) != 0 {
		t.Errorf("teardown ran on a cancelled context, so a real docker would skip it: %v", f.cancelled)
	}
}

func TestStartFailureTearsDown(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "x")
	f := &fakeDocker{fail: failOn(DindImage)}
	if _, err := Start(context.Background(), f, spec(true)); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Start err = %v, want the failing call's stderr", err)
	}
	f.call(t, []string{"rm", "-f"}, "fe-t1-proxy")
	f.call(t, []string{"network", "rm", "fe-t1"})
	for _, c := range f.calls {
		if slices.Contains(c, "fe-t1-trial") {
			t.Errorf("trial container touched after the dind failure: %v", c)
		}
	}
}

func TestStartWaitsForDind(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "x")
	setDindTiming(t, time.Millisecond, time.Minute)
	probes := 0
	f := &fakeDocker{fail: func(args []string) bool {
		if isDindProbe(args) {
			probes++
			return probes <= 2
		}
		return false
	}}
	if _, err := Start(context.Background(), f, spec(true)); err != nil {
		t.Fatal(err)
	}
	ready := slices.IndexFunc(f.calls, func(c []string) bool { return isDindProbe(c) }) + 2
	trial := slices.IndexFunc(f.calls, func(c []string) bool { return slices.Contains(c, "fe-t1-trial") })
	if probes != 3 || trial < ready {
		t.Errorf("probes = %d, trial created at call %d, dind ready at call %d; want the trial after 3 probes", probes, trial, ready)
	}
}

func TestStartFailsWhenDindNeverReady(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "x")
	setDindTiming(t, time.Millisecond, 20*time.Millisecond)
	f := &fakeDocker{fail: isDindProbe}
	if _, err := Start(context.Background(), f, spec(true)); err == nil || !strings.Contains(err.Error(), "dind") {
		t.Fatalf("Start err = %v, want a dind readiness error", err)
	}
	f.call(t, []string{"rm", "-f"}, "fe-t1-dind", "fe-t1-proxy")
	f.call(t, []string{"network", "rm", "fe-t1"})
	for _, c := range f.calls {
		if slices.Contains(c, "fe-t1-trial") && c[0] != "rm" {
			t.Errorf("trial container created although dind never became ready: %v", c)
		}
	}
}

func setDindTiming(t *testing.T, poll, timeout time.Duration) {
	oldPoll, oldTimeout := dindPoll, dindReadyTimeout
	dindPoll, dindReadyTimeout = poll, timeout
	t.Cleanup(func() { dindPoll, dindReadyTimeout = oldPoll, oldTimeout })
}

func TestStartRefusesUnsetCredentialAndBadNames(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "") // registers the restore
	os.Unsetenv("FE_TEST_TOKEN")
	for name, s := range map[string]TrialSpec{
		"unset credential": spec(false),
		"bad id":           func() TrialSpec { s := spec(false); s.ID = "a/b"; return s }(),
		"bad repo":         func() TrialSpec { s := spec(false); s.Repo = "../x"; return s }(),
		"bind mount": func() TrialSpec {
			s := spec(false)
			s.Credentials, s.Mounts = nil, []string{"type=bind,src=/,dst=/host"}
			return s
		}(),
	} {
		f := &fakeDocker{}
		if _, err := Start(context.Background(), f, s); err == nil {
			t.Errorf("%s: Start succeeded, want an error", name)
		}
		if len(f.calls) != 0 {
			t.Errorf("%s: docker called before validation: %s", name, f)
		}
	}
}

// docker's --mount parser reads one CSV record, folds key case and lets a later type= win, so each
// of the first three bind mounts the host directory although it starts with type=volume. The local
// volume driver's options bind a Docker-host path through a volume, so the last two do as well.
func TestStartRefusesABindMountDressedAsAVolume(t *testing.T) {
	for _, m := range []string{
		"type=volume,type=bind,src=/,dst=/host",
		"type=volume,TYPE=bind,src=/,dst=/host",
		`type=volume,"type=bind",src=/,dst=/host`,
		"src=fe-harness,dst=/fe",
		"type=volume,src=fe-x,dst=/h,volume-driver=local,volume-opt=type=none,volume-opt=o=bind,volume-opt=device=/etc",
		"type=volume,src=fe-x,dst=/h,VOLUME-OPT=type=none,volume-opt=o=bind,volume-opt=device=/etc",
	} {
		s := spec(false)
		s.Credentials, s.Mounts = nil, []string{m}
		f := &fakeDocker{}
		if _, err := Start(context.Background(), f, s); err == nil {
			t.Errorf("%s: Start succeeded, want an error", m)
		}
		if len(f.calls) != 0 {
			t.Errorf("%s: docker called before validation: %s", m, f)
		}
	}
}

func envOf(argv []string) []string {
	var env []string
	for i, a := range argv {
		if a == "-e" && i+1 < len(argv) {
			env = append(env, argv[i+1])
		}
	}
	return env
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestSnapshotHasSingleCommit(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("base.txt", "base\n")
	write(".gitignore", "*.jar\n")
	write("wrapper.jar", "tracked despite .gitignore\n")
	if err := os.Symlink("base.txt", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A", "-f")
	git(t, repo, "commit", "-q", "-m", "first")
	first := git(t, repo, "rev-parse", "HEAD")
	write("answer.txt", "the reference answer\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "second")
	second := git(t, repo, "rev-parse", "HEAD")

	tarPath, err := Snapshot(context.Background(), repo, first, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	assertTarEntries(t, tarPath)
	work := t.TempDir()
	if out, err := exec.Command("tar", "-xf", tarPath, "-C", work).CombinedOutput(); err != nil {
		t.Fatalf("tar: %v\n%s", err, out)
	}
	if n := git(t, work, "rev-list", "--count", "HEAD"); n != "1" {
		t.Errorf("snapshot history has %s commits, want 1", n)
	}
	for _, sha := range []string{second, first} {
		if err := exec.Command("git", "-C", work, "cat-file", "-e", sha).Run(); err == nil {
			t.Errorf("source commit %s is reachable in the snapshot", sha)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "answer.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("answer.txt from the later commit is in the snapshot: %v", err)
	}
	if st := git(t, work, "status", "--porcelain", "--ignored"); st != "" {
		t.Errorf("snapshot work tree differs from its one commit:\n%s", st)
	}
	if files := git(t, work, "ls-files"); files != ".gitignore\nbase.txt\nlink\nwrapper.jar" {
		t.Errorf("snapshot commit holds %q", files)
	}
	if target, err := os.Readlink(filepath.Join(work, "link")); err != nil || target != "base.txt" {
		t.Errorf("symlink not preserved: %q, %v", target, err)
	}
}

// assertTarEntries fails on an entry not owned by root, or on any extended-attribute record: docker cp
// into a Linux container refuses host xattrs such as macOS's com.apple.provenance.
func assertTarEntries(t *testing.T, tarPath string) {
	t.Helper()
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Uid != 0 || hdr.Gid != 0 {
			t.Errorf("snapshot entry %s is owned by %d:%d; git in the root-run container refuses a repository it does not own", hdr.Name, hdr.Uid, hdr.Gid)
		}
		for k := range hdr.PAXRecords {
			if strings.Contains(k, "xattr") {
				t.Errorf("snapshot entry %s carries %s", hdr.Name, k)
			}
		}
	}
}

func TestExecKillsTheCommandWhenCancelled(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "s3cret-value")
	f := &fakeDocker{}
	tr, err := Start(context.Background(), f, spec(false))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	if _, err := tr.Exec(ctx, "claude", "-p", "x"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Exec on an expired context: err %v, want one wrapping context.DeadlineExceeded", err)
	}
	kill := f.call(t, []string{"exec", "fe-t1-trial", "sh", "-c", "kill -KILL -1"})
	if slices.ContainsFunc(f.cancelled, func(c []string) bool { return slices.Equal(c, kill) }) {
		t.Errorf("kill ran on the cancelled context: %v", kill)
	}
	if _, err := tr.Exec(context.Background(), "true"); err != nil {
		t.Errorf("Exec on a live context: %v", err)
	}
}

func emptyTar(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "snapshot.tar")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// isMarkerProbe reports whether args test the Gradle volume for the warm marker.
func isMarkerProbe(args []string) bool {
	return slices.Contains(args, warmMarker) && slices.Contains(args, "test")
}

func TestWarmGradleRunsNoAgentAndNoCredential(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "s3cret-value")
	for _, tc := range []struct {
		name    string
		fail    func([]string) bool
		wantErr bool
	}{
		{name: "warms a cold volume", fail: isMarkerProbe},
		{name: "removes the container when gradle fails", fail: func(a []string) bool {
			return isMarkerProbe(a) || slices.ContainsFunc(a, func(s string) bool { return strings.Contains(s, "gradlew") })
		}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeDocker{fail: tc.fail}
			err := WarmGradle(context.Background(), f, "flow-eval-runner:test", "gymie", emptyTar(t))
			if (err != nil) != tc.wantErr {
				t.Fatalf("WarmGradle err = %v, want error %v", err, tc.wantErr)
			}
			name := warmName(t, f)
			run := f.call(t, []string{"run", "-d"}, "--name", name, "--network", "bridge", "--mount",
				"type=volume,src=flow-eval-gradle-ro-gymie,dst=/gradle-ro", "flow-eval-runner:test")
			for _, c := range f.calls {
				opts := c // the docker flags of a run or exec: everything before the image or container
				if i := slices.Index(c, name); c[0] == "exec" && i > 0 {
					opts = c[:i]
				} else if i := slices.Index(c, "flow-eval-runner:test"); c[0] == "run" && i > 0 {
					opts = c[:i]
				} else if c[0] != "run" && c[0] != "exec" {
					opts = nil
				}
				for i, a := range opts {
					switch {
					case a == "-e" || a == "--env" || strings.HasPrefix(a, "--env"):
						t.Errorf("environment passed to the warm-up container: %v", c)
					case a == "-v" || strings.HasPrefix(a, "--volume"):
						t.Errorf("bind-style volume flag: %v", c)
					case a == "--mount" && !strings.HasPrefix(opts[i+1], "type=volume,"):
						t.Errorf("--mount that is not a named volume: %v", c)
					}
				}
				for _, a := range c {
					if strings.HasPrefix(a, "fe-") || a == "claude" || a == "codex" || strings.Contains(a, "s3cret-value") {
						t.Errorf("trial network, agent or credential in %v", c)
					}
				}
			}
			if i := slices.Index(run, "--network"); run[i+1] != "bridge" {
				t.Errorf("warm-up not on the default bridge: %v", run)
			}
			f.call(t, []string{"cp", "-", name + ":/work"})
			f.call(t, []string{"exec", "-w", "/work", name, "sh", "-c"})
			if last := f.calls[len(f.calls)-1]; !slices.Equal(last, []string{"rm", "-f", "--volumes", name}) {
				t.Errorf("last docker call = %v, want the warm-up container removed", last)
			}
		})
	}
}

func TestWarmGradleSkipsAWarmVolume(t *testing.T) {
	f := &fakeDocker{}
	if err := WarmGradle(context.Background(), f, "flow-eval-runner:test", "gymie", emptyTar(t)); err != nil {
		t.Fatal(err)
	}
	name := warmName(t, f)
	f.call(t, []string{"exec", "-w", "/work", name, "test", "-e", warmMarker})
	for _, c := range f.calls {
		if c[0] == "cp" || slices.ContainsFunc(c, func(s string) bool { return strings.Contains(s, "gradlew") }) {
			t.Errorf("a warm volume was warmed again: %v", c)
		}
	}
	f.call(t, []string{"rm", "-f", "--volumes", name})
}

// A context that ends while docker run is creating the warm-up container leaves no id to remove; the
// container's name still reaches the removal.
func TestWarmGradleRemovesAContainerItCouldNotStart(t *testing.T) {
	f := &fakeDocker{fail: func(a []string) bool { return a[0] == "run" }}
	if err := WarmGradle(context.Background(), f, "flow-eval-runner:test", "gymie", emptyTar(t)); err == nil {
		t.Fatal("WarmGradle succeeded with docker run failing")
	}
	if last := f.calls[len(f.calls)-1]; !slices.Equal(last, []string{"rm", "-f", "--volumes", warmName(t, f)}) {
		t.Errorf("last docker call = %v, want the named warm-up container removed", last)
	}
}

// warmName returns the warm-up container's name from f's docker run.
func warmName(t *testing.T, f *fakeDocker) string {
	t.Helper()
	run := f.call(t, []string{"run", "-d"})
	name := run[slices.Index(run, "--name")+1]
	if !strings.HasPrefix(name, "flow-eval-warm-gymie-") {
		t.Fatalf("warm-up container name %q lacks the flow-eval-warm-gymie- prefix", name)
	}
	return name
}

// Two runs warming one repository at once each name their own container, so neither run's removal
// can reach the other's.
func TestWarmGradleNamesEachWarmUpApart(t *testing.T) {
	a, b := &fakeDocker{}, &fakeDocker{}
	for _, f := range []*fakeDocker{a, b} {
		if err := WarmGradle(t.Context(), f, "flow-eval-runner:test", "gymie", emptyTar(t)); err != nil {
			t.Fatal(err)
		}
	}
	if na, nb := warmName(t, a), warmName(t, b); na == nb {
		t.Errorf("two warm-ups share the container name %q", na)
	}
}

func TestEgressDeniedCountsProxyRefusals(t *testing.T) {
	t.Setenv("FE_TEST_TOKEN", "x")
	f := &fakeDocker{logs: "listening on :3128\negress-denied example.com\negress-denied github.com\n"}
	tr, err := Start(context.Background(), f, spec(false))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := tr.EgressDenied(context.Background()); err != nil || n != 2 {
		t.Errorf("EgressDenied = %d, %v; want 2", n, err)
	}
	f.call(t, []string{"logs", "fe-t1-proxy"})
}

// TestRealDockerGradleCacheIsReadOnlyAndPrivate warms a real volume from a one-class Gradle project
// with a dependency and the build cache on, then runs two real trials: each builds offline from the
// read-only cache, cannot write the volume, and never sees the other's Gradle user home. It needs
// FLOW_EVAL_IT_RUNNER and FLOW_EVAL_IT_PROXY (image names) and FLOW_EVAL_IT_GRADLEW_FROM (a checkout
// whose gradlew and gradle/wrapper it copies); without them it is skipped.
func TestRealDockerGradleCacheIsReadOnlyAndPrivate(t *testing.T) {
	runner, proxy, from := os.Getenv("FLOW_EVAL_IT_RUNNER"), os.Getenv("FLOW_EVAL_IT_PROXY"), os.Getenv("FLOW_EVAL_IT_GRADLEW_FROM")
	if runner == "" || proxy == "" || from == "" {
		t.Skip("set FLOW_EVAL_IT_RUNNER, FLOW_EVAL_IT_PROXY and FLOW_EVAL_IT_GRADLEW_FROM to run against real Docker")
	}
	ctx := t.Context()
	repoDir := t.TempDir()
	for _, f := range []string{"gradlew", "gradle/wrapper/gradle-wrapper.jar", "gradle/wrapper/gradle-wrapper.properties"} {
		data, err := os.ReadFile(filepath.Join(from, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repoDir, f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, f), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"settings.gradle.kts":       `rootProject.name = "it"` + "\n",
		"gradle.properties":         "org.gradle.caching=true\n",
		"build.gradle.kts":          "plugins { java }\nrepositories { mavenCentral() }\ndependencies { testImplementation(\"junit:junit:4.13.2\") }\n",
		"src/test/java/ItTest.java": "public class ItTest { @org.junit.Test public void t() {} }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repoDir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repoDir, "init", "-q")
	git(t, repoDir, "add", "-A")
	git(t, repoDir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "base")
	snap, err := Snapshot(ctx, repoDir, "HEAD", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	docker := DockerCLI{}
	repo := "fe-it-" + strings.ToLower(rand.Text()[:8])
	t.Cleanup(func() {
		if res, err := docker.Run(context.Background(), nil, "volume", "rm", "-f", "flow-eval-gradle-ro-"+repo); err != nil || res.Exit != 0 {
			t.Errorf("remove volume: %v %s", err, res.Stderr)
		}
	})
	if err := WarmGradle(ctx, docker, runner, repo, snap); err != nil {
		t.Fatal(err)
	}
	sh := func(tr *Trial, script string) Result {
		t.Helper()
		res, err := tr.Exec(ctx, "sh", "-c", script)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	var trials []*Trial
	for i := range 2 {
		tr, err := Start(ctx, docker, TrialSpec{ID: repo + "-" + fmt.Sprint(i), Repo: repo, RunnerImage: runner, ProxyImage: proxy})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := tr.Stop(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := tr.CopyIn(ctx, snap, "/work"); err != nil {
			t.Fatal(err)
		}
		trials = append(trials, tr)
	}
	first, second := trials[0], trials[1]
	for _, script := range []string{"./gradlew --offline --version", "./gradlew --offline testClasses"} {
		if res := sh(first, script); res.Exit != 0 {
			t.Fatalf("%s in a trial: exit %d: %s%s", script, res.Exit, res.Stdout, res.Stderr)
		}
	}
	if res := sh(first, "test -d /gradle-ro/caches/modules-2 && ! touch /gradle-ro/caches/x 2>/dev/null"); res.Exit != 0 {
		t.Error("the warmed dependency cache is missing, or a trial wrote into the shared Gradle volume")
	}
	if res := sh(first, "test -d /root/.gradle/caches/build-cache-1 && test ! -e /gradle-ro/caches/build-cache-1 && test ! -e /gradle-ro/daemon"); res.Exit != 0 {
		t.Error("the trial's build cache is not in its own Gradle home, or the volume holds a build cache or daemon logs")
	}
	if res := sh(second, "test ! -e /root/.gradle/caches/build-cache-1"); res.Exit != 0 {
		t.Error("the second trial sees the first trial's build cache")
	}
}

func TestRunReportsANonZeroExitAsAResult(t *testing.T) {
	res, err := run(t.Context(), nil, nil, "sh", "-c", "echo out; echo err >&2; exit 3")
	if err != nil || res.Exit != 3 || res.Stdout != "out\n" || res.Stderr != "err\n" {
		t.Errorf("run = %+v, %v; want exit 3 with its output and no error", res, err)
	}
}

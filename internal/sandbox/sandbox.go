// Package sandbox runs one trial in isolation (design.md §2): a per-trial internal Docker network
// whose only way out is the allowlist egress proxy, an optional docker:dind sidecar for cases that
// need Docker, and the trial container. Nothing from the host is bind-mounted; the repository
// snapshot and inputs are streamed in with docker cp, and credentials pass by environment name only.
package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// DindImage is the Docker-in-Docker sidecar image a needsDocker trial gets.
const DindImage = "docker:29.9.0-dind"

// proxyURL is where the trial and the dind daemon reach the egress proxy, by its network alias.
const proxyURL = "http://proxy:3128"

// proxyEnv routes the trial's clients and the dind daemon's image pulls through the egress proxy.
var proxyEnv = []string{
	"HTTPS_PROXY=" + proxyURL,
	"HTTP_PROXY=" + proxyURL,
	"NO_PROXY=localhost,127.0.0.1,dind",
}

// jvmProxyEnv gives Gradle and test JVMs, which ignore HTTPS_PROXY, the same route as system
// properties.
const jvmProxyEnv = "JAVA_TOOL_OPTIONS=-Dhttps.proxyHost=proxy -Dhttps.proxyPort=3128 " +
	"-Dhttp.proxyHost=proxy -Dhttp.proxyPort=3128 -Dhttp.nonProxyHosts=localhost|127.0.0.1|dind"

// Result is a finished docker or git command.
type Result struct {
	Exit           int
	Stdout, Stderr string
}

// Runner runs the docker CLI with args (without the leading "docker"). err is set only when the
// command could not run; a non-zero exit is reported in Result.Exit.
type Runner interface {
	Run(ctx context.Context, stdin io.Reader, args ...string) (Result, error)
}

// DockerCLI is the Runner that execs the host's docker CLI.
type DockerCLI struct{}

// Run execs docker with args.
func (DockerCLI) Run(ctx context.Context, stdin io.Reader, args ...string) (Result, error) {
	return run(ctx, stdin, nil, "docker", args...)
}

func run(ctx context.Context, stdin io.Reader, env []string, name string, args ...string) (Result, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, &stdout, &stderr
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	err := cmd.Run()
	res := Result{Exit: cmd.ProcessState.ExitCode(), Stdout: stdout.String(), Stderr: stderr.String()}
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		err = nil
	}
	return res, err
}

// TrialSpec describes one trial's sandbox.
type TrialSpec struct {
	ID          string   // unique per trial; names the network fe-<ID> and its containers
	Repo        string   // source repository name; keys the Gradle cache volume flow-eval-gradle-ro-<Repo>; "" mounts none
	NeedsDocker bool     // start a dind sidecar for Testcontainers
	Credentials []string // host environment variable names passed into the trial container
	Mounts      []string // the harness's own --mount values; each a named volume, never a host path
	RunnerImage string
	ProxyImage  string
}

var (
	dockerName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	envName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Trial is a started sandbox. Callers defer Stop right after Start succeeds.
type Trial struct {
	docker     Runner
	network    string   // set once created
	proxy      string   // the egress proxy container
	containers []string // created containers, in creation order
	name       string   // the trial container
}

// Start creates the trial's network, proxy, optional dind sidecar and trial container. On any
// failure it removes what it created before returning the error.
func Start(ctx context.Context, docker Runner, spec TrialSpec) (*Trial, error) {
	if err := validate(spec); err != nil {
		return nil, err
	}
	net := "fe-" + spec.ID
	t := &Trial{docker: docker, name: net + "-trial"}
	if err := t.start(ctx, spec, net); err != nil {
		return nil, errors.Join(err, t.Stop(ctx))
	}
	return t, nil
}

func validate(spec TrialSpec) error {
	if !dockerName.MatchString(spec.ID) || spec.Repo != "" && !dockerName.MatchString(spec.Repo) {
		return fmt.Errorf("sandbox: trial id %q and repo %q must match %s", spec.ID, spec.Repo, dockerName)
	}
	if spec.RunnerImage == "" || spec.ProxyImage == "" {
		return errors.New("sandbox: runner and proxy images are required")
	}
	for _, c := range spec.Credentials {
		if !envName.MatchString(c) {
			return fmt.Errorf("sandbox: credential %q is not an environment variable name", c)
		}
		if _, ok := os.LookupEnv(c); !ok {
			return fmt.Errorf("sandbox: credential %s is not set in the environment", c)
		}
	}
	for _, m := range spec.Mounts {
		if !isVolumeMount(m) {
			return fmt.Errorf("sandbox: mount %q is not a named volume", m)
		}
	}
	return nil
}

// _volumeMountKeys are the --mount keys a named volume needs. Any other is refused: the local
// driver's volume-driver and volume-opt keys bind a Docker-host path through a volume.
var _volumeMountKeys = []string{"type", "src", "source", "dst", "destination", "target", "volume-subpath",
	"readonly", "ro", "volume-nocopy"}

// isVolumeMount reports whether the --mount value m is a named volume mount, read as docker reads
// it: one CSV record whose keys are case-insensitive and whose last type= wins. So m must carry
// exactly one type key, and it must be volume, and no key outside _volumeMountKeys.
func isVolumeMount(m string) bool {
	fields, err := csv.NewReader(strings.NewReader(m)).Read()
	if err != nil {
		return false
	}
	var types []string
	for _, f := range fields {
		k, v, _ := strings.Cut(f, "=")
		k = strings.ToLower(k)
		if !slices.Contains(_volumeMountKeys, k) {
			return false
		}
		if k == "type" {
			types = append(types, v)
		}
	}
	return slices.Equal(types, []string{"volume"})
}

func (t *Trial) start(ctx context.Context, spec TrialSpec, net string) error {
	if err := t.docker1(ctx, "network", "create", "--internal", net); err != nil {
		return err
	}
	t.network = net

	// The proxy is the one container on both the internal network and the default bridge.
	proxy := net + "-proxy"
	t.proxy = proxy
	if err := t.docker1(ctx, "create", "--name", proxy, "--network", "bridge", spec.ProxyImage); err != nil {
		return err
	}
	t.containers = append(t.containers, proxy)
	if err := t.docker1(ctx, "network", "connect", "--alias", "proxy", net, proxy); err != nil {
		return err
	}
	if err := t.docker1(ctx, "start", proxy); err != nil {
		return err
	}

	env := append(append([]string{}, proxyEnv...), jvmProxyEnv)
	if spec.NeedsDocker {
		dind := net + "-dind"
		args := []string{"run", "-d", "--name", dind, "--network", net, "--network-alias", "dind",
			"--privileged", "-e", "DOCKER_TLS_CERTDIR="}
		args = append(args, envFlags(proxyEnv)...)
		t.containers = append(t.containers, dind)
		if err := t.docker1(ctx, append(args, DindImage)...); err != nil {
			return err
		}
		if err := t.waitDind(ctx, dind); err != nil {
			return err
		}
		env = append(env, "DOCKER_HOST=tcp://dind:2375", "TESTCONTAINERS_RYUK_DISABLED=true")
	}

	args := []string{"run", "-d", "--name", t.name, "--network", net, "-w", "/work"}
	if spec.Repo != "" {
		// The warmed volume, read-only: a trial reads its dependencies and wrapper but can leave
		// nothing in it for another trial. Gradle's user home stays the container's own /root/.gradle,
		// so its build cache, daemon logs and init scripts die with the trial.
		args = append(args, "--mount", gradleMount(spec.Repo)+",readonly")
		env = append(env, "GRADLE_RO_DEP_CACHE="+gradleRO+"/caches")
	}
	for _, m := range spec.Mounts {
		args = append(args, "--mount", m)
	}
	args = append(args, envFlags(env)...)
	// "-e NAME" without a value: docker copies the value from its own environment, so a secret never
	// reaches an argv.
	args = append(args, envFlags(spec.Credentials)...)
	t.containers = append(t.containers, t.name)
	if err := t.docker1(ctx, append(args, spec.RunnerImage, "sleep", "infinity")...); err != nil {
		return err
	}
	if spec.Repo == "" {
		return nil
	}
	// The wrapper locks the distribution it runs, which a read-only mount refuses, so each trial runs
	// its own copy.
	return t.docker1(ctx, "exec", t.name, "sh", "-c", _seedWrapper)
}

// gradleRO is where a trial mounts its repository's warmed Gradle volume, and where the warm-up
// fills it.
const gradleRO = "/gradle-ro"

// _seedWrapper copies the warmed wrapper distribution into the trial's own Gradle user home.
const _seedWrapper = "mkdir -p /root/.gradle && if [ -d " + gradleRO + "/wrapper ]; then cp -a " +
	gradleRO + "/wrapper /root/.gradle/; fi"

// gradleMount is the --mount value of repo's Gradle cache volume, the one volume that outlives a
// trial. Its name is not the flow-eval-gradle-<repo> of the shared-home layout, so a volume an older
// flow-eval filled with a build cache and daemon logs is never mounted into a trial.
func gradleMount(repo string) string {
	return "type=volume,src=flow-eval-gradle-ro-" + repo + ",dst=" + gradleRO
}

// warmMarker is the file WarmGradle leaves in a Gradle cache volume it has warmed.
const warmMarker = gradleRO + "/.flow-eval-warm"

// _warmScript resolves the snapshot's build in the warm-up container's own Gradle user home, then
// copies into the volume only what trials share: the wrapper distribution and the dependency cache
// (modules-2, less the lock and gc files Gradle's read-only cache must not carry). testClasses is
// best effort: a dependency it misses is downloaded by the trial that needs it. Nothing a build
// produces — build cache, daemon logs, compiled classes of the snapshot — reaches the volume.
const _warmScript = `set -e
if [ -f gradlew ]; then
	./gradlew --version
	./gradlew testClasses --continue || true
	mkdir -p ` + gradleRO + `/caches
	cp -a /root/.gradle/wrapper ` + gradleRO + `/
	if [ -d /root/.gradle/caches/modules-2 ]; then
		cp -a /root/.gradle/caches/modules-2 ` + gradleRO + `/caches/
		find ` + gradleRO + `/caches/modules-2 -name '*.lock' -delete
		rm -f ` + gradleRO + `/caches/modules-2/gc.properties
	fi
fi
touch ` + warmMarker

// WarmGradle fills repo's Gradle cache volume before its first trial (decision
// gradle-cache-warmed-host-side, as narrowed by gradle-volume-read-only): in one container from
// image on the default bridge, so with ordinary egress, but with no credential, no host path and no
// agent, it extracts snapshotTar into /work and runs _warmScript, which leaves the wrapper
// distribution, served from GitHub, and the dependency cache in the volume and writes warmMarker. A
// volume that already holds the marker is left alone, and a snapshot with no gradlew only gets the
// marker. The container is removed on every outcome.
func WarmGradle(ctx context.Context, docker Runner, image, repo, snapshotTar string) (err error) {
	if !dockerName.MatchString(repo) {
		return fmt.Errorf("sandbox: warm gradle: repo %q must match %s", repo, dockerName)
	}
	// Named, and its removal deferred before it starts, so a context ended between create and start
	// still removes it; `docker rm -f` of a name that never started exits 0. The random suffix keeps
	// two runs warming one repository from removing each other's container.
	name := "flow-eval-warm-" + repo + "-" + strings.ToLower(rand.Text())
	w := &Trial{docker: docker, name: name, containers: []string{name}}
	defer func() { err = errors.Join(err, w.Stop(ctx)) }()
	res, err := docker.Run(ctx, nil, "run", "-d", "--name", name, "--network", "bridge", "-w", "/work",
		"--mount", gradleMount(repo), image, "sleep", "infinity")
	if err := exitErr(res, err); err != nil {
		return fmt.Errorf("sandbox: warm gradle %s: docker run: %w", repo, err)
	}

	probe, err := w.Exec(ctx, "test", "-e", warmMarker)
	if err != nil {
		return fmt.Errorf("sandbox: warm gradle %s: %w", repo, err)
	}
	if probe.Exit == 0 {
		return nil
	}
	if err := w.CopyIn(ctx, snapshotTar, "/work"); err != nil {
		return fmt.Errorf("sandbox: warm gradle %s: %w", repo, err)
	}
	if err := exitErr(w.Exec(ctx, "sh", "-c", _warmScript)); err != nil {
		return fmt.Errorf("sandbox: warm gradle %s: %w", repo, err)
	}
	return nil
}

// stopTimeout bounds teardown, which must run even after the trial's own context ended.
const stopTimeout = time.Minute

// dindPoll and dindReadyTimeout bound the wait for a dind daemon to answer; vars so tests can
// shorten them.
var (
	dindPoll         = time.Second
	dindReadyTimeout = 90 * time.Second
)

// waitDind returns once the dind daemon answers, so a trial never starts against a daemon that is
// still booting.
func (t *Trial) waitDind(ctx context.Context, dind string) error {
	ctx, cancel := context.WithTimeout(ctx, dindReadyTimeout)
	defer cancel()
	tick := time.NewTicker(dindPoll)
	defer tick.Stop()
	for {
		res, err := t.docker.Run(ctx, nil, "exec", dind, "docker", "version")
		if err == nil && res.Exit == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("dind %s not ready (%w, budget %s): last probe exit %d: %s",
				dind, context.Cause(ctx), dindReadyTimeout, res.Exit, strings.TrimSpace(res.Stderr))
		case <-tick.C:
		}
	}
}

func envFlags(env []string) []string {
	var flags []string
	for _, e := range env {
		flags = append(flags, "-e", e)
	}
	return flags
}

// exitErr folds a non-zero exit into an error carrying stderr.
func exitErr(res Result, err error) error {
	if err == nil && res.Exit != 0 {
		err = fmt.Errorf("exit %d: %s", res.Exit, strings.TrimSpace(res.Stderr))
	}
	return err
}

// docker1 runs one management command; a non-zero exit is an error.
func (t *Trial) docker1(ctx context.Context, args ...string) error {
	if err := exitErr(t.docker.Run(ctx, nil, args...)); err != nil {
		return fmt.Errorf("docker %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// CopyIn extracts the tar archive at tarPath into dir inside the trial container.
func (t *Trial) CopyIn(ctx context.Context, tarPath, dir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := exitErr(t.docker.Run(ctx, f, "cp", "-", t.name+":"+dir)); err != nil {
		return fmt.Errorf("docker cp %s into %s: %w", tarPath, dir, err)
	}
	return nil
}

// Exec runs cmd in /work inside the trial container and returns its exit code and output. When ctx
// ends first, killing the host-side docker client does not stop cmd inside the container, so Exec
// kills every process there but PID 1 and returns an error wrapping ctx's cause: nothing the
// command started outlives it, and the container stays usable for grading.
func (t *Trial) Exec(ctx context.Context, cmd ...string) (Result, error) {
	res, err := t.docker.Run(ctx, nil, append([]string{"exec", "-w", "/work", t.name}, cmd...)...)
	if ctx.Err() == nil {
		return res, err
	}
	kctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopTimeout)
	defer cancel()
	killErr := t.docker1(kctx, "exec", t.name, "sh", "-c", "kill -KILL -1")
	return res, errors.Join(fmt.Errorf("exec %s: %w", strings.Join(cmd, " "), context.Cause(ctx)), killErr)
}

// EgressDenied returns how many connections the trial's egress proxy has refused: its
// "egress-denied <host>" log lines (design.md §2).
func (t *Trial) EgressDenied(ctx context.Context) (int, error) {
	res, err := t.docker.Run(ctx, nil, "logs", t.proxy)
	if err := exitErr(res, err); err != nil {
		return 0, fmt.Errorf("docker logs %s: %w", t.proxy, err)
	}
	n := 0
	for line := range strings.Lines(res.Stdout + res.Stderr) {
		if strings.HasPrefix(line, "egress-denied ") {
			n++
		}
	}
	return n, nil
}

// Stop removes the trial's containers and network, and never the Gradle cache volume. It is safe to
// call more than once, and it runs even when ctx is already cancelled, bounded by stopTimeout.
func (t *Trial) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopTimeout)
	defer cancel()
	var errs []error
	if len(t.containers) > 0 {
		rm := []string{"rm", "-f", "--volumes"} // dind's anonymous /var/lib/docker; never a named volume
		for i := len(t.containers) - 1; i >= 0; i-- {
			rm = append(rm, t.containers[i])
		}
		errs = append(errs, t.docker1(ctx, rm...))
		t.containers = nil
	}
	if t.network != "" {
		errs = append(errs, t.docker1(ctx, "network", "rm", t.network))
		t.network = ""
	}
	return errors.Join(errs...)
}

// Snapshot writes the tree of repo at sha, re-initialised as a repository holding one commit
// "base", to a tar under workDir and returns its path. No object of the source history, and so no
// later reference commit, is reachable from it; the agent's own changes are `git diff HEAD`. The
// tree it archived stays at workDir/tree, for the host to read the snapshot from.
func Snapshot(ctx context.Context, repo, sha, workDir string) (string, error) {
	archive := filepath.Join(workDir, "archive.tar")
	tree := filepath.Join(workDir, "tree")
	tarPath := filepath.Join(workDir, "snapshot.tar")
	if err := os.Mkdir(tree, 0o755); err != nil {
		return "", err
	}
	// Host git config (hooks, signing, templates) must not shape the snapshot.
	isolated := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	steps := [][]string{
		{"git", "-C", repo, "archive", "--format=tar", "-o", archive, sha},
		{"tar", "-xf", archive, "-C", tree},
		{"git", "-C", tree, "init", "-q"},
		// -f: a file the source tracked despite its .gitignore stays in the base commit.
		{"git", "-C", tree, "add", "-A", "-f"},
		{"git", "-C", tree, "-c", "user.name=flow-eval", "-c", "user.email=flow-eval@localhost",
			"commit", "-q", "-m", "base"},
	}
	for _, s := range steps {
		if err := exitErr(run(ctx, nil, isolated, s[0], s[1:]...)); err != nil {
			return "", fmt.Errorf("snapshot %s@%s: %s: %w", repo, sha, strings.Join(s, " "), err)
		}
	}
	if err := writeTar(tarPath, tree); err != nil {
		return "", fmt.Errorf("snapshot %s@%s: %w", repo, sha, err)
	}
	return tarPath, nil
}

// writeTar archives the contents of dir with every entry owned by root and no extended attributes:
// docker cp keeps a tar's ownership, git in the root-run trial container refuses a repository it
// does not own, and docker cp into a Linux container refuses host xattrs such as macOS's
// com.apple.provenance, which the host tar would carry.
func writeTar(tarPath, dir string) (err error) {
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	tw := tar.NewWriter(f)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		var link string
		if fi.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		}
		hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0, 0, "root", "root"
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
	return errors.Join(err, tw.Close())
}

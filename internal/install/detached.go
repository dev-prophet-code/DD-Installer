package install

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// Detached steps run on the target under nohup/setsid with their output in a
// log file. DD-Installer only polls: if the SSH connection drops, the computer
// sleeps or DD-Installer is closed, the step keeps running on the server and
// the next poll (after reconnecting) simply continues reading the log.

const (
	pollInterval   = 2 * time.Second
	maxReconnects  = 8
	maxPollsFailed = 40
)

func detachDir(id string) string { return "$HOME/.dd-installer/" + id }

func detachStartScript(id, script string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "D=\"%s\"\n", detachDir(id))
	// umask 077: the log can hold the generated admin password, so neither the
	// folder nor the files may ever be readable by other users.
	b.WriteString(`umask 077
mkdir -p "$D" && chmod 700 "$HOME/.dd-installer" "$D"
if [ -f "$D/pid" ] && [ ! -f "$D/exit" ] && kill -0 "$(cat "$D/pid")" 2>/dev/null; then
  echo "DD_DETACH already-running"
  exit 0
fi
rm -f "$D/exit"; : > "$D/log"
`)
	b.WriteString(heredoc("\"$D/step.sh\"", script))
	b.WriteString(`chmod 700 "$D/step.sh"
S=""; command -v setsid >/dev/null 2>&1 && S=setsid
( [ -z "$S" ] && set -m; $S nohup sh -c 'sh "$1" > "$2" 2>&1 < /dev/null; echo $? > "$3"' sh "$D/step.sh" "$D/log" "$D/exit" > /dev/null 2>&1 &
  echo $! > "$D/pid" )
echo "DD_DETACH started"
`)
	return b.String()
}

func detachPollScript(id string, offset int) string {
	// The state is read BEFORE the size. If the job has finished, everything it
	// wrote is already in the log, so the size below covers all of it; if it is
	// still running, the next poll picks up the rest. (Reading the size first
	// could report "exited" together with a stale size and drop the last lines.)
	return fmt.Sprintf(`D="%s"
st=running
if [ -f "$D/exit" ]; then st="exit:$(cat "$D/exit")"
elif ! { [ -f "$D/pid" ] && kill -0 "$(cat "$D/pid")" 2>/dev/null; }; then st=gone; fi
sz=$(wc -c < "$D/log" 2>/dev/null || echo 0)
echo "DD_STATE $st"
echo DD_BODY
tail -c +%d "$D/log" 2>/dev/null | head -c $((sz - %d)) 2>/dev/null
`, detachDir(id), offset+1, offset)
}

// detachCleanupScript removes the job folder (script, log, pid). The log holds
// everything the installer printed, including the generated admin password, so
// it must not stay on the server once DD-Installer has read it.
func detachCleanupScript(id string) string {
	return fmt.Sprintf(`rm -rf "%s"
rmdir "$HOME/.dd-installer" 2>/dev/null || true
`, detachDir(id))
}

// cleanupDetached is best effort; it is also safe to call twice.
func (o *Options) cleanupDetached(id string) {
	c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, _, _ = transport.Capture(c, o.Runner, detachCleanupScript(id))
}

func detachKillScript(id string) string {
	return fmt.Sprintf(`D="%s"
if [ -f "$D/pid" ]; then
  P=$(cat "$D/pid")
  # the job leads its own process group (setsid): stop the whole group; dash needs "-s TERM --"
  kill -s TERM -- "-$P" 2>/dev/null || kill -s TERM "$P" 2>/dev/null || true
  command -v pkill >/dev/null 2>&1 && pkill -TERM -P "$P" 2>/dev/null || true
fi
`, detachDir(id))
}

// parsePoll splits a poll result into state and new log bytes.
func parsePoll(out string) (state, body string, ok bool) {
	i := strings.Index(out, "DD_BODY\n")
	if i < 0 {
		return "", "", false
	}
	head := out[:i]
	for _, l := range strings.Split(head, "\n") {
		if strings.HasPrefix(l, "DD_STATE ") {
			state = strings.TrimSpace(strings.TrimPrefix(l, "DD_STATE "))
		}
	}
	if state == "" {
		return "", "", false
	}
	return state, out[i+len("DD_BODY\n"):], true
}

// reconnect replaces the runner with a fresh connection, retrying with
// back-off. It returns false when the connection could not be restored.
func (o *Options) reconnect(ctx context.Context) bool {
	if o.Reconnect == nil {
		return false
	}
	delay := 2 * time.Second
	for i := 1; i <= maxReconnects; i++ {
		if ctx.Err() != nil {
			return false
		}
		o.Emit(Event{Type: "log", Msg: fmt.Sprintf("Connection lost - reconnecting (attempt %d of %d)...", i, maxReconnects)})
		r, err := o.Reconnect(ctx)
		if err == nil {
			_ = o.Runner.Close()
			o.Runner = r
			o.Emit(Event{Type: "log", Msg: "Reconnected."})
			return true
		}
		o.Sleep(delay)
		if delay < 30*time.Second {
			delay *= 2
		}
	}
	return false
}

// runDetached starts (or re-attaches to) the step on the target and follows
// its log until it exits. It returns the step's exit code and its output.
func runDetached(ctx context.Context, o *Options, red *redactor, id, script string) (int, string, error) {
	var all bytes.Buffer
	emit := newLineSink(o, red, &all)
	defer emit.Flush()

	// start (or detect that it is already running)
	for {
		_, code, err := transport.Capture(ctx, o.Runner, detachStartScript(id, script))
		if err == nil && code == 0 {
			break
		}
		if err != nil && transport.IsConnectionLost(err) && o.reconnect(ctx) {
			continue
		}
		if err == nil {
			err = fmt.Errorf("could not start the background step (exit %d)", code)
		}
		return -1, all.String(), err
	}

	offset, failedPolls := 0, 0
	for {
		if ctx.Err() != nil {
			kctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			_, _, _ = transport.Capture(kctx, o.Runner, detachKillScript(id))
			cancel()
			o.cleanupDetached(id)
			return -1, all.String(), ctx.Err()
		}
		out, _, err := transport.Capture(ctx, o.Runner, detachPollScript(id, offset))
		state, body, ok := parsePoll(out)
		if err != nil || !ok {
			failedPolls++
			if failedPolls > maxPollsFailed {
				return -1, all.String(), fmt.Errorf("lost track of the background step: %v", err)
			}
			if err != nil && transport.IsConnectionLost(err) && !o.reconnect(ctx) {
				return -1, all.String(), err
			}
			o.Sleep(pollInterval)
			continue
		}
		failedPolls = 0
		if body != "" {
			offset += len(body)
			emit.Write([]byte(body))
		}
		switch {
		case state == "running":
			o.Sleep(pollInterval)
		case strings.HasPrefix(state, "exit:"):
			if body != "" {
				continue // drain what was written just before the exit
			}
			code, _ := strconv.Atoi(strings.TrimPrefix(state, "exit:"))
			o.cleanupDetached(id)
			return code, all.String(), nil
		default: // "gone": process vanished without an exit code (e.g. reboot)
			if body != "" {
				continue
			}
			o.cleanupDetached(id)
			return -1, all.String(), fmt.Errorf("the background step stopped unexpectedly (the server may have restarted)")
		}
	}
}

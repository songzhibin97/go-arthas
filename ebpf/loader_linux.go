//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target native watch ./bpf/watch.bpf.c -- -I./bpf/headers

const maxRegs = 6

// rawEvent 与 watch.bpf.c 的 struct event 内存布局严格一致
type rawEvent struct {
	KTime  uint64
	Cookie uint64
	Regs   [maxRegs]uint64
	PID    uint32
	TID    uint32
	Kind   uint8
	_      [7]byte
}

type linuxAttacher struct {
	objs   watchObjects
	links  []link.Link
	reader *ringbuf.Reader
	events chan Event
	cookie map[uint64]string
	done   chan struct{}
}

// Attach 加载 eBPF 程序并对目标函数的入口与各 RET 偏移挂 uprobe。
// 需要 root / CAP_BPF 与支持 bpf cookie 的内核（≥ 5.15）。
func Attach(opts AttachOptions) (Attacher, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove memlock: %w", err)
	}

	objs := watchObjects{}
	if err := loadWatchObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("load eBPF objects: %w", err)
	}

	ex, err := link.OpenExecutable(opts.BinaryPath)
	if err != nil {
		objs.Close()
		return nil, fmt.Errorf("open executable %s: %w", opts.BinaryPath, err)
	}

	a := &linuxAttacher{
		objs:   objs,
		events: make(chan Event, 512),
		cookie: map[uint64]string{},
		done:   make(chan struct{}),
	}

	for i, ft := range opts.Targets {
		ck := uint64(i)
		a.cookie[ck] = ft.Name

		// 入口 uprobe（OnEnter）
		enter, err := ex.Uprobe(ft.Name, objs.UprobeEnter, &link.UprobeOptions{PID: opts.PID, Cookie: ck})
		if err != nil {
			a.Close()
			return nil, fmt.Errorf("attach entry uprobe %s: %w", ft.Name, err)
		}
		a.links = append(a.links, enter)

		// 每个 RET 指令各挂一个 uprobe（OnExit）—— 绝不用 uretprobe
		for _, off := range ft.ReturnOffs {
			ret, err := ex.Uprobe(ft.Name, objs.UprobeExit, &link.UprobeOptions{PID: opts.PID, Cookie: ck, Offset: off})
			if err != nil {
				a.Close()
				return nil, fmt.Errorf("attach return uprobe %s+%#x: %w", ft.Name, off, err)
			}
			a.links = append(a.links, ret)
		}
	}

	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("open ringbuf: %w", err)
	}
	a.reader = rd

	go a.loop()
	return a, nil
}

func (a *linuxAttacher) loop() {
	defer close(a.events)
	for {
		rec, err := a.reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			continue
		}
		var raw rawEvent
		if err := binary.Read(bytes.NewReader(rec.RawSample), binary.LittleEndian, &raw); err != nil {
			continue
		}
		ev := Event{
			Kind:    EventKind(raw.Kind),
			Func:    a.cookie[raw.Cookie],
			PID:     raw.PID,
			TID:     raw.TID,
			KTimeNs: raw.KTime,
			Regs:    append([]uint64(nil), raw.Regs[:]...),
		}
		select {
		case a.events <- ev:
		case <-a.done:
			return
		}
	}
}

func (a *linuxAttacher) Events() <-chan Event { return a.events }

func (a *linuxAttacher) Close() error {
	select {
	case <-a.done:
	default:
		close(a.done)
	}
	if a.reader != nil {
		a.reader.Close()
	}
	for _, l := range a.links {
		_ = l.Close()
	}
	return a.objs.Close()
}
